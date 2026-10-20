package internal_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Go metninde kullanıcıya görünen Türkçe yoktur: adlar ad kodudur (internal/lang/codes), iletiler
// ileti kodudur (internal/lang/messages); metinleri dil dosyalarındadır. Bu test, test dışı Go
// dosyalarında Türkçe harf (çğıöşüâîû ÇĞİÖŞÜÂÎÛ) taşıyan metin sabitini arar. Bilinçli yerler
// turkishLiteralAllowed'da gerekçesiyle durur; liste yalnızca küçülür.
//
// Başka projelerin karşılığı: Android lint "HardcodedText", Kubernetes'in i18n denetimi ve
// go-i18n'in çıkarıcısı kaynak dilin metnini koddan dil dosyasına taşımayı ister. Türkçe harf
// ölçütü kaba ama kesindir (yanlış alarm vermez); Türkçe harfsiz sözcükleri ("bitti") ise
// ileti denetimi (msgconsts: fmt.Errorf/errors.New yasağı) ve çıktı yerlerinin taraması yakalar.

const turkishLetters = "çğıöşüâîûÇĞİÖŞÜÂÎÛ"

// turkishLiteralAllowed "dosya:metin" anahtarlı kalıcı izinlerdir → gerekçe: veri biçiminin
// anahtarları (kod listesi, kontrol noktası, günlük, veri kümesi durumu, profil dosyası) ve Türkçe
// belge üreten aracın içeriği. Yeni kayıt gerekçesiz eklenmez; liste yalnızca küçülür. Bulguları
// kayıt biçiminde yazdırmak için: TRLIT_LIST=1 go test -run TestNoTurkishLiterals ./internal
var turkishLiteralAllowed = map[string]string{
	// Kod listesi dosyalarının (internal/lang/codes) biçim anahtarları.
	"internal/lang/codes/definition.go:bağlamsal":    "definitions.json'daki \"tür\" alanının değeri (dosya biçimi)",
	"internal/lang/codes/definition.go:özellik":      "definitions.json'daki \"tür\" alanının değeri (dosya biçimi)",
	"internal/lang/codes/definition.go:adlı_argüman": "definitions.json'daki \"tür\" alanının değeri (dosya biçimi)",
	"internal/lang/codes/definition.go:%s adlı: %v":  "definitions.json'un alan adı (dosya biçimi); geliştirici hatasında anılır",
	"internal/lang/codes/definition.go:%s dönüş: %v": "definitions.json'un alan adı (dosya biçimi); geliştirici hatasında anılır",
	"internal/lang/codes/definition.go:%s: \"tipler\", \"param\", \"adlı\" and \"dönüş\" must not be written when \"imzalar\" is present": "definitions.json'un alan adı (dosya biçimi); geliştirici hatasında anılır",
	"internal/lang/codes/definition.go:ilişkili": "definitions.json'un alan adı (dosya biçimi); geliştirici hatasında anılır",
	"internal/lang/codes/definition.go:taşındı":  "definitions.json'un alan adı (dosya biçimi); geliştirici hatasında anılır",
	"internal/lang/codes/json_parse.go:yabancı":  "dil dosyasının (tr.json) alan adı (dosya biçimi)",
	"internal/lang/codes/json_parse.go:açıklama": "dil dosyasının (tr.json) alan adı (dosya biçimi)",
	"internal/lang/codes/json_parse.go:örnek":    "dil dosyasının (tr.json) alan adı (dosya biçimi)",
	"internal/lang/codes/json_parse.go:%s entry has unknown field %q (valid fields: ad, eski, yabancı, aile, açıklama, örnek)": "dil dosyasının (tr.json) alan adı (dosya biçimi)",
	"internal/devtools/profiler/setup.go:canlı":                                                                                                 "profil görüntüsü dosyasının 'tür' alanı değeri (dosya biçimi; değiştirmek eski dosyaları okunmaz yapardı)",
	"internal/stdlib/checkpoint/format.go:parametreler iyileştirici durumlu günlük değerler":                                                    "kontrol noktası dosyasındaki kayıt türü adları (dosya biçimi; değiştirmek eski kontrol noktalarını okunmaz yapardı)",
	"internal/stdlib/checkpoint/plain.go:ondalık sözlük":                                                                                        "kontrol noktasındaki düz değer etiketleri (dosya biçimi; eski dosyalar okunmaz olurdu)",
	"internal/stdlib/dataset/state.go:tür toplu bağlam tohum küme dönem sıra pencere sayaç bitti örnekleyici sıralı":                            "veri kümesi durum dosyasının alan adları (dosya biçimi; eski durum dosyaları okunmaz olurdu)",
	"internal/stdlib/logging/log.go:tür zaman sürüm program toplam ayarlar adım hız süre_sn mesaj ölçüler kalan_sn bitti başlangıç bitiş devam": "günlük dosyasının alan adları (dosya biçimi; eski günlükler okunmaz olurdu)",
}

// turkishLiteralSkipDirs taranmayan klasörlerdir: Türkçenin dilbilgisi paketi (ek ve harf
// tabloları dilin kendisidir) ve depo dışı klasörler.
var turkishLiteralSkipDirs = []string{"internal/lang/turkish", ".git", ".claude", ".codegraph", "node_modules", "testdata"}

func TestNoTurkishLiterals(t *testing.T) {
	root := reporoot.Root()
	found := map[string][]string{}
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			for _, s := range turkishLiteralSkipDirs {
				if rel == s || strings.HasSuffix(rel, "/"+s) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		for _, lit := range turkishLiterals(t, path) {
			if _, ok := turkishLiteralAllowed[rel]; ok {
				used[rel] = true
				continue
			}
			if _, ok := turkishLiteralAllowed[rel+":"+lit]; ok {
				used[rel+":"+lit] = true
				continue
			}
			found[rel] = append(found[rel], lit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	total := 0
	for f, l := range found {
		files = append(files, f)
		total += len(l)
	}
	sort.Slice(files, func(i, j int) bool { return len(found[files[i]]) > len(found[files[j]]) })
	if os.Getenv("TRLIT_LIST") == "1" {
		// Bulguları izin listesine yapıştırılacak biçimde yazar; gerekçeyi siz yazarsınız.
		for _, f := range files {
			for _, lit := range found[f] {
				fmt.Printf("\t%q: \"\",\n", f+":"+lit)
			}
		}
	}
	for _, f := range files {
		t.Errorf("%s: %d metin: %s", f, len(found[f]), strings.Join(quoteAll(found[f], 6), ", "))
	}
	if total > 0 {
		t.Logf("%d dosyada %d Türkçe metin sabiti: adı ad koduna, iletiyi ileti koduna taşıyın; bilinçli bir yerse gerekçesiyle turkishLiteralAllowed'a yazın", len(files), total)
	}
	for k := range turkishLiteralAllowed {
		if !used[k] {
			t.Errorf("izin listesinde kullanılmayan kayıt: %q; silin (liste yalnızca küçülür)", k)
		}
	}
}

// turkishLiterals dosyadaki Türkçe harfli metin sabitleridir; yapı alanı etiketleri (veri biçimi
// anahtarları, `json:"adım"`) sayılmaz. Derleme etiketine bakılmaz: her işletim sisteminin
// dosyası taranır.
func turkishLiterals(t *testing.T, path string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if fld, ok := n.(*ast.Field); ok && fld.Tag != nil {
			tags[fld.Tag] = true
		}
		return true
	})
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || tags[lit] {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			s = lit.Value
		}
		if strings.ContainsAny(s, turkishLetters) {
			out = append(out, s)
		}
		return true
	})
	return out
}

func quoteAll(list []string, max int) []string {
	var out []string
	for i, s := range list {
		if i == max {
			out = append(out, fmt.Sprintf("… (+%d)", len(list)-max))
			break
		}
		if len([]rune(s)) > 40 {
			s = string([]rune(s)[:40]) + "…"
		}
		out = append(out, strconv.Quote(s))
	}
	return out
}
