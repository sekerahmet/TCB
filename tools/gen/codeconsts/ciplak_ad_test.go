package main

import (
	"go/ast"
	"go/parser"
	"go/token"
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
)

// "Çıplak ad yok": dilin kullanıcıya görünen adları (anahtar kelimeler, tip adları,
// yerleşikler, metot ve özellikler, adlı argümanlar) Go metin sabitlerine yazılmaz;
// mesajlar adı koddan üretir (kod.Ad(kod.G100)). Böylece bir ad tr.json'da değişince ya da
// başka bir dil dosyası yüklenince mesajlar da değişir.
//
// Test, dilin adlarını üreten katmanların (internal/lang, internal/stdlib, internal/devtools,
// cmd/tr) test dışı Go dosyalarındaki metin sabitlerini tarar. Dilin adları çoğu zaman
// sıradan Türkçe sözcüklerdir ("devam", "doğru", "yok"); bu yüzden bir ad ancak kod gibi
// yazıldığı yerde yakalanır:
//   - tip adları (Tam, Liste ...) ve "_" içeren adlar (eşit_mi, öğrenme_oranı) her yerde;
//   - çağrı: ad(  ;  üye: .ad  ;  adlı argüman: ad:  ;  tırnak içinde: 'ad', "ad", `ad`;
//   - anahtar kelimeden sonra blok: ad {  ;  metnin tamamı bir ad: "doğru".
//
// internal/compute bu taramanın dışındadır: o katman dili bilmez; İngilizce iç adlarının
// mesajlara sızması ayrı bir işin konusudur. Yorumlar ve yapı etiketleri taranmaz.
//
// İzin listesi küçük tutulur ve her kaydın gerekçesi yazılır. Yeni kod bu testi izin
// listesine kayıt eklemeden geçmelidir.

// ciplakAdIzinleri: dosya (depo köküne göre) ve o dosyada izin verilen ad → gerekçe.
var ciplakAdIzinleri = map[string]map[string]string{
	// Komut arayüzü: komut satırının alt komutları ve hata ayıklayıcının komutları dilin
	// adları değil, aracın komutlarıdır. Kod listesine (K ailesi) çevirici işinde girecekler.
	"cmd/tr/main.go": {"test": "komut satırı alt komutu (tr test)"},
	"internal/devtools/debugger/ayiklayici.go":    {"devam": "hata ayıklayıcı komutu (devam); A014 anahtar kelimesi değil"},
	"internal/devtools/debugger/komut_rehberi.go": {"devam": "hata ayıklayıcı komutu (devam); eski eşlerin ipucu tablosu"},
	// Eklentiyle konuşulan iletinin alan adları ve değer türleri: protokolün sabit anahtarları
	// (eklenti bunlara göre ikon ve sütun seçer); kullanıcıya dilin adı olarak görünmez.
	"internal/devtools/debugger/dap_tablo.go": {
		"tip":   "İzleme Masası iletisinin alan adı (\"tip\": ...)",
		"metin": "İzleme Masası iletisinin değer türü (ikon seçimi)",
		"yok":   "İzleme Masası iletisinin değer türü (ikon seçimi)",
	},
	// tanim.json'un "tür" alanının değerleri: dosya biçiminin sabit anahtarlarıdır.
	"internal/lang/codes/tanim.go": {
		"tip":       "tanim.json tür değeri (TurTip)",
		"fonksiyon": "tanim.json tür değeri (TurFonksiyon)",
		"sabit":     "tanim.json tür değeri (TurSabit)",
	},
	// Hata türü başlıklarındaki düz yazı ("Fonksiyon çağrısı tip hatası", "Tip uyuşmazlığı",
	// "Tip ve Güvenlik Denetimi"): sıradan sözcük, A208 Fonksiyon ya da A215 Tip adı değil.
	"internal/lang/checker/denetci.go": {
		"Fonksiyon": "hata türü başlığı: Fonksiyon çağrısı tip hatası",
		"Tip":       "hata türü başlığı: Tip uyuşmazlığı",
	},
	"cmd/tr/hazirlik.go": {"Tip": "başlık: Tip ve Güvenlik Denetimi Hataları"},
	// Sözdizimi yardımındaki yer tutucu: "ad: Tip" herhangi bir tip adı demektir.
	"internal/lang/parser/yapi.go": {"Tip": "yer tutucu: ad: Tip (herhangi bir tip adı)"},
}

// taranacakKokler adları koddan üretmesi gereken katmanlardır.
var taranacakKokler = []string{"internal/lang", "internal/stdlib", "internal/devtools", "cmd/tr"}

// atlananDosyalar kodların kendisini tanımlayan (adları dil dosyasından okuyan) dosyalardır.
var atlananDosyalar = map[string]bool{
	"internal/lang/codes/kodlar_uretilmis.go": true, // üretilmiş: yorumlarda adlar
}

type canliAd struct {
	kod dk.Kod
	al  dk.Alan
}

// canliAdlar etkin dilin canlı adlarıdır (ad → kodlar; aynı ad farklı ad alanlarında olabilir).
func canliAdlar() map[string][]canliAd {
	m := map[string][]canliAd{}
	for _, k := range dk.Kodlar() {
		for _, y := range dk.Yazimlar(k) {
			m[y] = append(m[y], canliAd{k, k.Alan()})
		}
	}
	return m
}

func harfMi(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// ciplakAdlar bir metindeki kod gibi yazılmış dil adlarını verir.
func ciplakAdlar(s string, adlar map[string][]canliAd) []string {
	var bulunan []string
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !harfMi(r) {
			i += n
			continue
		}
		j := i
		for j < len(s) {
			r2, n2 := utf8.DecodeRuneInString(s[j:])
			if !harfMi(r2) {
				break
			}
			j += n2
		}
		w := s[i:j]
		if kayitlar, ok := adlar[w]; ok && kodGibi(s, i, j, kayitlar) {
			bulunan = append(bulunan, w)
		}
		i = j
	}
	return bulunan
}

// kodGibi s[i:j]'deki ad kod gibi mi yazılmış (bkz. dosya başı)?
func kodGibi(s string, i, j int, kayitlar []canliAd) bool {
	w := s[i:j]
	once, _ := utf8.DecodeLastRuneInString(s[:i])
	sonra, _ := utf8.DecodeRuneInString(s[j:])
	if i == 0 {
		once = 0
	}
	if j == len(s) {
		sonra = 0
	}
	if strings.Contains(w, "_") {
		return true
	}
	if (once == '\'' && sonra == '\'') || (once == '"' && sonra == '"') || (once == '`' && sonra == '`') {
		return true
	}
	butun := i == 0 && j == len(s) && utf8.RuneCountInString(w) > 1
	for _, c := range kayitlar {
		switch {
		case butun && (c.al == dk.AlanGenel || c.al == dk.AlanTip):
			return true // metnin tamamı bir anahtar kelime, yerleşik ya da tip adı: "doğru"
		case c.al == dk.AlanTip:
			return true
		case sonra == '(':
			return true
		case once == '.' && c.al == dk.AlanUye:
			return true
		case sonra == ':' && c.al == dk.AlanParametre:
			return true
		case c.kod.AnahtarKelime() && strings.HasPrefix(s[j:], " {"):
			return true
		}
	}
	return false
}

func TestCiplakAdYok(t *testing.T) {
	kok := reporoot.Kok()
	adlar := canliAdlar()
	kullanilanIzin := map[string]map[string]bool{}
	var bulgular []string
	for _, k := range taranacakKokler {
		err := filepath.WalkDir(filepath.Join(kok, k), func(yol string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(yol, ".go") || strings.HasSuffix(yol, "_test.go") {
				return nil
			}
			goreli, _ := filepath.Rel(kok, yol)
			goreli = filepath.ToSlash(goreli)
			if atlananDosyalar[goreli] {
				return nil
			}
			bulgular = append(bulgular, dosyaTara(t, yol, goreli, adlar, kullanilanIzin)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(bulgular)
	for _, b := range bulgular {
		t.Error(b)
	}
	if len(bulgular) > 0 {
		t.Logf("%d çıplak ad: adı koddan üretin (kod.Ad(kod.Xnnn)); gerçekten gerekiyorsa gerekçesiyle ciplakAdIzinleri'ne yazın", len(bulgular))
	}
	for dosya, izinler := range ciplakAdIzinleri {
		for ad := range izinler {
			if !kullanilanIzin[dosya][ad] {
				t.Errorf("izin listesinde kullanılmayan kayıt: %s %q; silin (liste yalnızca küçülür)", dosya, ad)
			}
		}
	}
}

func dosyaTara(t *testing.T, yol, goreli string, adlar map[string][]canliAd, kullanilan map[string]map[string]bool) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, yol, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Errorf("%s: %v", goreli, err)
		return nil
	}
	etiketler := map[*ast.BasicLit]bool{}
	for _, im := range f.Imports {
		etiketler[im.Path] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if fl, ok := n.(*ast.Field); ok && fl.Tag != nil {
			etiketler[fl.Tag] = true
		}
		return true
	})
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || etiketler[lit] {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		for _, ad := range ciplakAdlar(s, adlar) {
			if _, izinli := ciplakAdIzinleri[goreli][ad]; izinli {
				if kullanilan[goreli] == nil {
					kullanilan[goreli] = map[string]bool{}
				}
				kullanilan[goreli][ad] = true
				continue
			}
			out = append(out, goreli+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+": "+strconv.Quote(ad)+" :: "+kisalt(s))
		}
		return true
	})
	return out
}

func kisalt(s string) string {
	if r := []rune(s); len(r) > 100 {
		return string(r[:100]) + "…"
	}
	return s
}

// Tarayıcının kendisi: kod gibi yazılmış adları yakalar, düz yazıdaki sözcüklere dokunmaz.
func TestCiplakAdTarayicisi(t *testing.T) {
	adlar := canliAdlar()
	for metin, want := range map[string]string{
		"girdi() ya da girdi(istem)":            "girdi girdi",
		"%q bir sabittir ('sabit')":             "sabit",
		"eşit_mi başarısız":                     "eşit_mi",
		"Liste indeksi Tam sayı olmalıdır":      "Liste Tam",
		"x.uzunluk yazın":                       "uzunluk",
		"(tohum: 1)":                            "tohum",
		"doğru":                                 "doğru",
		"eğer { ... }":                          "eğer",
		"döngü devam ediyor; sonuç doğru değil": "",
		"önce toplam alın, sonra yok sayın":     "",
		"o":                                     "",
	} {
		if got := strings.Join(ciplakAdlar(metin, adlar), " "); got != want {
			t.Errorf("%q: %q bekleniyordu, %q bulundu", metin, want, got)
		}
	}
}
