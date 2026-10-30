package internal_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
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
// anahtarları (kontrol noktası, günlük, veri kümesi durumu, profil dosyası) ve Türkçe
// belge üreten aracın içeriği. Yeni kayıt gerekçesiz eklenmez; liste yalnızca küçülür. Bulguları
// kayıt biçiminde yazdırmak için: TRLIT_LIST=1 go test -run TestNoTurkishLiterals ./internal
var turkishLiteralAllowed = map[string]string{
	// UTS #39 karıştırılabilir karakter tablosu (Unicode verisinden üretilir): harfler veridir.
	"internal/lang/lexer/confusables_generated.go": "Unicode'un karıştırılabilir karakter tablosu; harfler veridir, ad ya da ileti değil",
	// Dil tanımı atıf tarayıcısı Türkçe belgeleri okur: "PyTorch 2.14", "sürümü 2.1" gibi sürüm numaraları madde atfı sayılmaz.
	// Kitaplığın 4c'yi bekleyen tipsiz yerleri adla tutulur (6.1, 7.1): adlar lib/ dosyalarındaki kullanıcı adlarıdır; liste 4c'de kalkar.
	"internal/lang/checker/library_exceptions.go": "gömülü kitaplığın istisnalı yerlerinin adları (lib/ dosyalarındaki kullanıcı adları); 4c'de kalkar",
	"tools/internal/specref/scan.go:\\b(PyTorch|NumPy|JAX|Go|go|CUDA|cuDNN|cuBLAS|cuBLASLt|Python|Rust|Swift|Kotlin|LSP|CommonMark|Unicode|IEEE|Triton|TensorFlow|Hugging Face|sürüm|sürümü|version)\\b[^.!?]*$": "belgedeki sürüm numarasını tanıyan desen; Türkçe belge sözcüğü, ad ya da ileti değil",
}

// turkishLiteralSkipDirs taranmayan klasörlerdir: Türkçenin dilbilgisi paketi (ek ve harf
// tabloları dilin kendisidir) ve depo dışı klasörler.
var turkishLiteralSkipDirs = []string{"internal/lang/turkish", ".git", ".claude", ".codegraph", "node_modules", "testdata"}

func TestNoTurkishLiterals(t *testing.T) {
	root := reporoot.Root()
	found := map[string][]string{}
	used := map[string]bool{}
	err := reporoot.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
	var out []string
	err := testkit.GoStringLiterals(path, func(s string, _ int) {
		if strings.ContainsAny(s, turkishLetters) {
			out = append(out, s)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
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
