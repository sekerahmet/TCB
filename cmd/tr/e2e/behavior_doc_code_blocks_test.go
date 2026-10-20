package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Belgelerdeki ve eklenti şablonlarındaki TR kodu bozulmasın: her ```tr kod bloğu
// `tr kontrol`dan geçmelidir (hakem mimari 6: kılavuzdaki veritabanı örneği derlenmiyordu).
// Bilerek hatalı yazılmış örnekler ```tr hata ile işaretlenir; onların kontrolden
// GEÇMEMESİ beklenir (belge hatayı gösteriyorsa hata gerçekten oluşmalı).

// generatedDoc kod listesinden üretilen başvuru belgesidir; içindeki örnekler kod
// listesinin testlerinde (internal/stdlib) çalıştırılır, burada yalnızca bağlantıları denetlenir.
const generatedDoc = "docs/codes.md"

// belgeListesi depodaki belgelerdir (depo köküne göre, bölü çizgisiyle): kökteki README,
// SECURITY ve CONTRIBUTING, docs/ altındaki bütün .md dosyaları, kitaplığın, eklentinin ve
// kıyas setinin README'leri, sözlük isteği konu şablonu. Yeni bir
// belge docs/ altına eklendiğinde kendiliğinden denetime girer.
func belgeListesi(t *testing.T) []string {
	t.Helper()
	list := []string{"README.md", "SECURITY.md", "CONTRIBUTING.md", "lib/README.md",
		"editors/vscode/README.md", "tools/bench/README.md", ".github/ISSUE_TEMPLATE/name_request.md"}
	for _, pattern := range []string{"docs/*.md", "docs/*/*.md"} {
		d, err := filepath.Glob(reporoot.Path(pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, filePath := range d {
			relative, err := filepath.Rel(reporoot.Root(), filePath)
			if err != nil {
				t.Fatal(err)
			}
			list = append(list, filepath.ToSlash(relative))
		}
	}
	if len(list) < 20 {
		t.Fatalf("yalnızca %d belge bulundu; docs/ yapısı değişmiş olabilir", len(list))
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
	for _, b := range belgeListesi(t) {
		if b == generatedDoc {
			continue
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
				t.Errorf("%s:%d: kod bloğu `tr kontrol`dan geçmiyor:\n%s", b, line, stdout)
			case errorExpected && passed:
				t.Errorf("%s:%d: ```tr hata bloğu hata vermeli ama `tr kontrol`dan geçiyor", b, line)
			}
		}
	}
	if total < 20 {
		t.Fatalf("yalnızca %d kod bloğu bulundu; belge biçimi değişmiş olabilir", total)
	}
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
	defined, _ := checker.TopLevelNames(parser.New(lexer.New(code)).ParseProgram())
	var on strings.Builder
	seen := map[string]bool{}
	for _, m := range templateDefaultRe.FindAllStringSubmatch(body, -1) {
		ad := m[1]
		if !namePlaceholderRe.MatchString(ad) || seen[ad] || slices.Contains(defined, ad) || lexer.LookupIdent(ad) != lexer.TokIdent {
			continue
		}
		seen[ad] = true
		fmt.Fprintf(&on, "değişken %s = 1\n", ad)
	}
	return on.String() + code
}

// Eklentinin kod şablonları varsayılan değerleriyle açıldığında derlenebilir kod üretmelidir
// (hakem mimari 3: eşle şablonu derlenmeyen kod üretiyordu).
func TestExtensionTemplatesBuild(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("editors/vscode/snippets/tr.code-snippets"))
	if err != nil {
		t.Fatal(err)
	}
	var templates map[string]struct {
		Prefix []string `json:"prefix"`
		Body   []string `json:"body"`
	}
	if err := json.Unmarshal(data, &templates); err != nil {
		t.Fatalf("şablon dosyası geçerli JSON değil: %v", err)
	}
	if len(templates) < 10 {
		t.Fatalf("yalnızca %d şablon bulundu", len(templates))
	}
	for ad, s := range templates {
		body := strings.Join(s.Body, "\n")
		code := openTemplate(body)
		if strings.HasPrefix(code, ".") {
			// Metot şablonları (.filtrele { o }) bir listenin ardına yazılır
			code = "değişken liste = [1, 2, 3]\nliste" + code
		}
		code = definePlaceholders(body, code)
		if passed, stdout := trCheck(t, "sablon", code); !passed {
			t.Errorf("%q şablonu derlenmiyor:\n%s\n---\n%s", ad, code, stdout)
		}
	}
}

// Kaldırılan kodların adları (ve eski adları) belgelerde kod bloğu dışında da geçmemeli: ```tr
// blokları zaten `tr kontrol`dan geçer (kaldırılan ad orada hata verir), ama düzyazıdaki
// satır içi kod (`model_yükle`, `x.kaydet()`) ve çağrı biçimi (ad(...)) denetlenmiyordu (hakem
// O14: kılavuz kaldırılmış sinir_ağı'nı ve model.kaydet'i anlatıyordu). Ad listesi kod
// paketinden gelir (kod.OluBul ve kod.Kaldirildi). Adın hangi ad alanında arandığı
// yazımından anlaşılır: '.ad' üye, 'ad:' adlı argüman, 'ad(' genel; satır içi kodda tek başına
// yazılmış `ad` genel ya da tip. Canlı bir kodla aynı yazılan ad (ör. 'tip') denetlenmez.

// removedNameAllowances meşru tarihçe cümleleridir: belge → izin verilen ad → gerekçe.
var removedNameAllowances = map[string]map[string]string{
	"docs/spec/02_sozcuk_yapisi.md": nameAllowance("kaldırılan özelliklerin listesi: bu adlar yazılınca 'kaldırıldı' hatası verilir (kural 2.9a)",
		"veritabanı_aç", "tablo_sorgula", "web_al", "web_gönder", "web_sunucu", "arayüz_oluştur",
		"sinir_ağı", "model_yükle", "eğit", "tahmin_et", "ağırlıklar", "kayıp_mse", "yz_sor",
		"normalize_et", "nokta_çarpım", "kosinüs_benzerliği", "öklid_uzaklığı"),
}

func nameAllowance(reason string, names ...string) map[string]string {
	m := map[string]string{}
	for _, a := range names {
		m[a] = reason
	}
	return m
}

var (
	inlineCodeRe = regexp.MustCompile("`([^`\n]+)`")
	identifierRe = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)
	cagriRe      = regexp.MustCompile(`([\p{L}_][\p{L}\p{N}_]*)\(`)
)

// removedName ad, verilen ad alanında (AlanYok: genel ya da tip) yerine geçen olmadan
// kaldırılmış bir kodun adı ya da eski adıysa o kodu verir.
func removedName(take codes.Namespace, ad string) (codes.Code, bool) {
	fields := []codes.Namespace{take}
	if take == codes.NamespaceNone {
		fields = []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType}
	}
	for _, a := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType, codes.NamespaceMember, codes.NamespaceParam} {
		if _, live := codes.Find(a, ad); live && (a == take || take == codes.NamespaceNone) {
			return codes.None, false
		}
	}
	for _, a := range fields {
		if k, ok := codes.FindDead(a, ad); ok && codes.Removed(k) {
			return k, true
		}
	}
	return codes.None, false
}

// inlineNames satır içi kod parçasındaki adları ad alanlarıyla verir.
func inlineNames(part string) map[string]codes.Namespace {
	names := map[string]codes.Namespace{}
	if p := strings.TrimSpace(part); identifierRe.FindString(p) == p {
		names[p] = codes.NamespaceNone
		return names
	}
	for _, m := range identifierRe.FindAllStringIndex(part, -1) {
		ad, before, after := part[m[0]:m[1]], part[:m[0]], part[m[1]:]
		switch {
		case strings.HasSuffix(before, "."):
			names[ad] = codes.NamespaceMember
		case strings.HasPrefix(after, ":"):
			names[ad] = codes.NamespaceParam
		case strings.HasPrefix(after, "("):
			names[ad] = codes.NamespaceGlobal
		}
	}
	return names
}

func TestRemovedNamesAbsentFromDocs(t *testing.T) {
	if len(codes.RemovedCodes()) == 0 {
		t.Skip("kaldırılmış kod yok")
	}
	for _, b := range belgeListesi(t) {
		if b == generatedDoc { // codes.md kaldırılan kodları bilerek listeler
			continue
		}
		data, err := os.ReadFile(reporoot.Path(b))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range uncodedLines(string(data)) {
			bulunan := map[string]codes.Namespace{}
			for _, m := range inlineCodeRe.FindAllStringSubmatch(line, -1) {
				for ad, take := range inlineNames(m[1]) {
					bulunan[ad] = take
				}
			}
			flat := inlineCodeRe.ReplaceAllString(line, "")
			for _, m := range cagriRe.FindAllStringSubmatchIndex(flat, -1) {
				if m[0] == 0 || flat[m[0]-1] != '.' {
					bulunan[flat[m[2]:m[3]]] = codes.NamespaceGlobal
				}
			}
			for ad, take := range bulunan {
				k, ok := removedName(take, ad)
				if !ok {
					continue
				}
				if _, allowed := removedNameAllowances[b][ad]; allowed {
					continue
				}
				t.Errorf("%s:%d: %q kaldırılmış bir kodun adı (%s); belgeyi bugünkü dile göre düzeltin ya da meşru bir tarihçe cümlesiyse gerekçesiyle kaldirilanAdIzinleri'ne ekleyin", b, i+1, ad, k)
			}
		}
	}
}
