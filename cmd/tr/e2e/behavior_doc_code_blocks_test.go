package e2e

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Belgelerdeki ve eklenti şablonlarındaki TR kodu bozulmasın: her ```tr kod bloğu
// `tr kontrol`dan geçmelidir (hakem mimari 6: kılavuzdaki veritabanı örneği derlenmiyordu).
// Bilerek hatalı yazılmış örnekler ```tr hata ile işaretlenir; onların kontrolden
// GEÇMEMESİ beklenir (belge hatayı gösteriyorsa hata gerçekten oluşmalı).

// generatedDoc kod listesinden üretilen başvuru belgesidir; içindeki örnekler kod
// listesinin testlerinde (internal/stdlib) çalıştırılır, burada yalnızca bağlantıları denetlenir.
const generatedDoc = "docs/codes.md"

// belgeListesi depodaki izlenen bütün .md dosyalarıdır (depo köküne göre, bölü çizgisiyle);
// tarihli arşiv (docs/archive/) ve testlerin bilerek bozuk verisi (testdata/) dışarıda kalır. Yeni bir belge nereye eklenirse eklensin kendiliğinden
// denetime girer (2026-10-29: elle liste examples/README.md'yi ve paket README'lerini dışarıda
// bırakıyordu; codegraph'ın gördüğü iki kırık bağlantı oradaydı).
func belgeListesi(t *testing.T) []string {
	t.Helper()
	var list []string
	err := reporoot.WalkDir(reporoot.Root(), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(reporoot.Root(), p)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if d.IsDir() && (relative == "docs/archive" || d.Name() == "testdata") {
			return fs.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(relative, ".md") {
			list = append(list, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 100 {
		t.Fatalf("yalnızca %d belge bulundu; depo yapısı değişmiş olabilir", len(list))
	}
	return list
}

var trBlockRe = regexp.MustCompile("(?ms)^```tr( hata)?[ \t]*\r?\n(.*?)^```")

// trCheck kodu geçici bir dosyaya yazıp `tr kontrol` ile denetler.
func trCheck(t *testing.T, ad, code string) (bool, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), ad+".tr")
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, err := exec.Command(trBinary, "kontrol", file).CombinedOutput()
	return err == nil, string(stdout)
}

func TestDocTRCodeBlocks(t *testing.T) {
	total := 0
	var failing []string
	for _, b := range belgeListesi(t) {
		if b == generatedDoc || strings.HasPrefix(b, "docs/archive/") {
			continue // arşiv geçmiş biçimi olduğu gibi saklar
		}
		data, err := os.ReadFile(reporoot.Path(b))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, m := range trBlockRe.FindAllStringSubmatchIndex(text, -1) {
			total++
			line := strings.Count(text[:m[0]], "\n") + 1
			errorExpected := m[2] >= 0
			code := text[m[4]:m[5]]
			ad := fmt.Sprintf("%s_%d", strings.NewReplacer("/", "_", ".", "_").Replace(b), line)
			passed, stdout := trCheck(t, ad, code)
			switch {
			case !errorExpected && !passed:
				failing = append(failing, fmt.Sprintf("%s:%d", b, line))
				t.Logf("%s:%d: kod bloğu `tr kontrol`dan geçmiyor:\n%s", b, line, stdout)
			case errorExpected && passed:
				failing = append(failing, fmt.Sprintf("%s:%d hata", b, line))
				t.Logf("%s:%d: ```tr hata bloğu hata vermeli ama `tr kontrol`dan geçiyor", b, line)
			}
		}
	}
	if total < 20 {
		t.Fatalf("yalnızca %d kod bloğu bulundu; belge biçimi değişmiş olabilir", total)
	}
	header := "# Dil tanımının henüz uygulanmamış kurallarını gösteren kod blokları (TestDocTRCodeBlocks): belge:satır.\n" +
		"# Kuralı uygulayan katman kendi satırlarını siler; liste yalnızca küçülür (-update).\n"
	testkit.CheckKnownList(t, "testdata/doc_blocks_known.txt", header, failing, *updateDisplayKnown)
}

var (
	templateDefaultRe = regexp.MustCompile(`\$\{\d+:([^${}]*)\}`)
	templateEmptyRe   = regexp.MustCompile(`\$\{\d+\}|\$\d+`)
)

// openTemplate bir düzenleyici şablonunu varsayılan değerleriyle açar (Tab'a hiç basılmamış gibi).
func openTemplate(body string) string {
	for {
		updated := templateDefaultRe.ReplaceAllString(body, "$1")
		if updated == body {
			break
		}
		body = updated
	}
	return templateEmptyRe.ReplaceAllString(body, "")
}

// namePlaceholderRe yer tutucunun varsayılanı tek bir ad mı (eğer ${1:koşul})?
var namePlaceholderRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// definePlaceholders şablonun kullanıcının dolduracağı ad yer tutucularını (eğer
// ${1:koşul}, eşle ${1:değer}) şablon kendisi tanımlamıyorsa kodun başında tanımlar:
// tanımsız ad çalışmadan önce hatadır (5.14), şablon ise bu adların kullanıcıda var
// olduğunu varsayar.
func definePlaceholders(body, code string) string {
	defined, _ := checker.TopLevelNames(parser.New(source.TextLexer(code)).ParseProgram())
	var on strings.Builder
	seen := map[string]bool{}
	for _, m := range templateDefaultRe.FindAllStringSubmatch(body, -1) {
		ad := m[1]
		if !namePlaceholderRe.MatchString(ad) || seen[ad] || slices.Contains(defined, ad) || lexer.LookupIdent(ad) != lexer.TokIdent {
			continue
		}
		seen[ad] = true
		fmt.Fprintf(&on, "TR_var %s = 1\n", ad)
	}
	return on.String() + code
}

// Eklentinin kod şablonları varsayılan değerleriyle açıldığında derlenebilir kod üretmelidir
// (hakem mimari 3: eşle şablonu derlenmeyen kod üretiyordu).
func TestExtensionTemplatesBuild(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("editors/vscode/snippets/snippets.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Türkçe şablonlar kaynaktır; öbür dillerinki ondan üretilir ve tools/gen/snippets testlerinde
	// kendi yazımlarında ayrıştırılır.
	var languages map[string][]struct {
		Label string   `json:"label"`
		Body  []string `json:"body"`
	}
	if err := json.Unmarshal(data, &languages); err != nil {
		t.Fatalf("şablon dosyası geçerli JSON değil: %v", err)
	}
	templates := map[string]struct{ Body []string }{}
	for _, s := range languages["tr"] {
		templates[s.Label] = struct{ Body []string }{s.Body}
	}
	if len(templates) < 10 {
		t.Fatalf("yalnızca %d şablon bulundu", len(templates))
	}
	for ad, s := range templates {
		body := strings.Join(s.Body, "\n")
		code := openTemplate(body)
		if strings.HasPrefix(code, ".") {
			// Metot şablonları (.filtrele { o }) bir listenin ardına yazılır
			code = "TR_var liste = [1, 2, 3]\nliste" + code
		}
		if strings.HasPrefix(code, "[") {
			// İfade şablonu (liste üreteci) bir atamanın sağına yazılır (2.8).
			code = "TR_var sonuç_listesi = " + code
		}
		code = definePlaceholders(body, code)
		// Türkçe şablon Türkçe görünümde yazılır; dosyaya kodlanarak girer (17).
		coded, err := source.EncodeIn(code, source.Turkish())
		if err != nil {
			t.Errorf("%q şablonu kodlanamadı: %v\n%s", ad, err, code)
			continue
		}
		if source.HeaderLines(coded) == 0 {
			coded = source.Header + "\n" + coded
		}
		if passed, stdout := trCheck(t, "sablon", coded); !passed {
			t.Errorf("%q şablonu derlenmiyor:\n%s\n---\n%s", ad, code, stdout)
		}
	}
}
