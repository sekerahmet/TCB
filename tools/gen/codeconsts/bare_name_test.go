package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	dk "github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// "Çıplak ad yok": dilin kullanıcıya görünen adları (anahtar kelimeler, tip adları,
// yerleşikler, metot ve özellikler, adlı argümanlar, komut satırı adları) Go metin
// sabitlerine yazılmaz; mesajlar adı koddan üretir (kod.Ad(kod.FnPrint)). Böylece bir ad
// tr.json'da değişince ya da başka bir dil dosyası yüklenince mesajlar da değişir.
//
// Test, dilin adlarını üreten katmanların (internal/lang, internal/stdlib, internal/devtools,
// cmd/tr) test dışı Go dosyalarındaki metin sabitlerini tarar. Dilin adları çoğu zaman
// sıradan Türkçe sözcüklerdir ("devam", "doğru", "yok"); bu yüzden bir ad ancak kod gibi
// yazıldığı yerde yakalanır:
//   - tip adları (Tam, Liste ...) ve "_" içeren adlar (eşit_olmalı, öğrenme_oranı) her yerde;
//   - çağrı: ad(  ;  üye: .ad  ;  adlı argüman: ad:  ;  tırnak içinde: 'ad', "ad", `ad`;
//   - anahtar kelimeden sonra blok: ad {  ;  metnin tamamı bir ad: "doğru", "göster" (üye ve
//     adlı argüman adı dışında; bkz. looksLikeCode).
//
// Aranan yazım Türkçedir: İngilizce yazımlar Go'nun protokol ve veri anahtarlarıyla aynı düşer
// (hata ayıklama protokolünün "next", "step" istekleri; 2026-10-27 denemesinde 29 yanlış alarm).
//
// internal/compute bu taramanın dışındadır: o katman dili bilmez; İngilizce iç adlarının
// mesajlara sızması ayrı bir işin konusudur. Yorumlar ve yapı etiketleri taranmaz.
//
// İzin listesi küçük tutulur ve her kaydın gerekçesi yazılır. Yeni kod bu testi izin
// listesine kayıt eklemeden geçmelidir.

// bareNameAllowances: dosya (depo köküne göre) ve o dosyada izin verilen ad → gerekçe.
var bareNameAllowances = map[string]map[string]string{
	// Dosya biçiminin adı (JSON), TR_type_JSON tipinin adı değil: dil ve ileti dosyalarının okunma hatası.
	"internal/lang/codes/language.go":   {"JSON": "dil dosyası biçiminin adı (tip değil)"},
	"internal/lang/messages/catalog.go": {"JSON": "ileti dosyası biçiminin adı (tip değil)"},
}

// scanRoots adları koddan üretmesi gereken katmanlardır.
var scanRoots = []string{"internal/lang", "internal/stdlib", "internal/devtools", "cmd/tr"}

// skippedFiles kodların kendisini tanımlayan (adları dil dosyasından okuyan) dosyalardır.
var skippedFiles = map[string]bool{
	"internal/lang/codes/codes_generated.go": true, // üretilmiş: yorumlarda adlar
}

type liveName struct {
	code      dk.Code
	namespace dk.Namespace
}

// liveNames etkin dilin canlı adlarıdır (ad → kodlar; aynı ad farklı ad alanlarında olabilir).
func liveNames() map[string][]liveName {
	m := map[string][]liveName{}
	for _, k := range dk.Codes() {
		// Kitaplığın adları TR kodudur, Go onları anmaz; tek harfli parametre adları (x, v, k) Go'nun
		// biçim dizeleriyle (%v) karışırdı.
		if k.IsLibrary() {
			continue
		}
		for _, y := range dk.Source().Spellings(k) { // DENEME: Go metninde Türkçe ad aranır
			m[y] = append(m[y], liveName{k, k.Namespace()})
		}
	}
	return m
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// bareNames bir metindeki kod gibi yazılmış dil adlarını verir.
func bareNames(s string, names map[string][]liveName) []string {
	var found []string
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !isWordRune(r) {
			i += n
			continue
		}
		j := i
		for j < len(s) {
			r2, n2 := utf8.DecodeRuneInString(s[j:])
			if !isWordRune(r2) {
				break
			}
			j += n2
		}
		w := s[i:j]
		if entries, ok := names[w]; ok && looksLikeCode(s, i, j, entries) {
			found = append(found, w)
		}
		i = j
	}
	return found
}

// looksLikeCode s[i:j]'deki ad kod gibi mi yazılmış (bkz. dosya başı)?
func looksLikeCode(s string, i, j int, entries []liveName) bool {
	w := s[i:j]
	before, _ := utf8.DecodeLastRuneInString(s[:i])
	after, _ := utf8.DecodeRuneInString(s[j:])
	if i == 0 {
		before = 0
	}
	if j == len(s) {
		after = 0
	}
	if strings.Contains(w, "_") {
		return true
	}
	if (before == '\'' && after == '\'') || (before == '"' && after == '"') || (before == '`' && after == '`') {
		return true
	}
	whole := i == 0 && j == len(s) && utf8.RuneCountInString(w) > 1
	for _, c := range entries {
		switch {
		case whole && c.namespace != dk.NamespaceMember && c.namespace != dk.NamespaceParam:
			// Metnin tamamı bir anahtar kelime, yerleşik, tip ya da komut satırı adı: "doğru",
			// "göster", "tanım". Üye ve adlı argüman adları veri biçimlerinin İngilizce anahtarlarıyla
			// aynı düşebilir (kontrol noktasının "momentum" durumu), onlar yalnız kod gibi yazıldığı
			// yerde (.ad, ad:) yakalanır.
			return true
		case c.namespace == dk.NamespaceType:
			return true
		case after == '(':
			return true
		case before == '.' && c.namespace == dk.NamespaceMember:
			return true
		case after == ':' && c.namespace == dk.NamespaceParam:
			return true
		case c.code.IsKeyword() && strings.HasPrefix(s[j:], " {"):
			return true
		}
	}
	return false
}

func TestNoBareNames(t *testing.T) {
	root := reporoot.Root()
	names := liveNames()
	usedAllowances := map[string]map[string]bool{}
	var findings []string
	for _, k := range scanRoots {
		err := reporoot.WalkDir(filepath.Join(root, k), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, _ := filepath.Rel(root, path)
			relPath = filepath.ToSlash(relPath)
			if skippedFiles[relPath] {
				return nil
			}
			findings = append(findings, scanFile(t, path, relPath, names, usedAllowances)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(findings)
	for _, b := range findings {
		t.Error(b)
	}
	if len(findings) > 0 {
		t.Logf("%d çıplak ad: adı koddan üretin (kod.Ad(kod.Xnnn)); gerçekten gerekiyorsa gerekçesiyle bareNameAllowances'ne yazın", len(findings))
	}
	for file, allowed := range bareNameAllowances {
		for name := range allowed {
			if !usedAllowances[file][name] {
				t.Errorf("izin listesinde kullanılmayan kayıt: %s %q; silin (liste yalnızca küçülür)", file, name)
			}
		}
	}
}

func scanFile(t *testing.T, path, relPath string, names map[string][]liveName, used map[string]map[string]bool) []string {
	var out []string
	err := testkit.GoStringLiterals(path, func(s string, line int) {
		for _, name := range bareNames(s, names) {
			if _, isAllowed := bareNameAllowances[relPath][name]; isAllowed {
				if used[relPath] == nil {
					used[relPath] = map[string]bool{}
				}
				used[relPath][name] = true
				continue
			}
			out = append(out, relPath+":"+strconv.Itoa(line)+": "+strconv.Quote(name)+" :: "+shorten(s))
		}
	})
	if err != nil {
		t.Errorf("%s: %v", relPath, err)
		return nil
	}
	return out
}

func shorten(s string) string {
	if r := []rune(s); len(r) > 100 {
		return string(r[:100]) + "…"
	}
	return s
}

// Tarayıcının kendisi: kod gibi yazılmış adları yakalar, düz yazıdaki sözcüklere dokunmaz.
func TestBareNameScanner(t *testing.T) {
	names := liveNames()
	for text, want := range map[string]string{
		"girdi() ya da girdi(istem)":       "girdi girdi",
		"%q bir sabittir ('sabit')":        "sabit",
		"eşit_olmalı başarısız":            "eşit_olmalı",
		"Liste indeksi Tam sayı olmalıdır": "Liste Tam",
		"x.uzunluk yazın":                  "uzunluk",
		"(tohum: 1)":                       "tohum",
		"doğru":                            "doğru",
		"göster":                           "göster",
		"tanım":                            "tanım",
		"zıpla":                            "zıpla",
		"momentum":                         "",
		"eğer { ... }":                     "eğer",
		"döngü devam ediyor; sonuç doğru değil": "",
		"önce toplam alın, sonra yok sayın":     "",
		"o":                                     "",
	} {
		if got := strings.Join(bareNames(text, names), " "); got != want {
			t.Errorf("%q: %q bekleniyordu, %q bulundu", text, want, got)
		}
	}
}
