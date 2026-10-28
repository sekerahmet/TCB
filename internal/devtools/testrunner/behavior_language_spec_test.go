package testrunner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/lib"
)

// Faz 3 kapısı: "her kural bir testte". Dil tanımındaki (docs/spec/NN_*.md) her kural (**4.2**
// gibi) altındaki "- *Test:*" satırında neyle sınandığını söyler (Ahmet, 2026-10-27): "TR testi"
// tests/ klasöründe adı o numarayla başlayan bir test ister; "Go: `TestX`" Go testini anar
// (varlığını internal.TestRuleTestsExist denetler); "gerekmez" nedeniyle yazılır. Test satırı
// olan bölümde her kural bir Test satırı ister; Test satırı henüz olmayan bölümde (gözden
// geçirilmemiş) her kural bir TR testi ister. Tersine, her test numarası tanımda bir kurala
// karşılık gelmeli. Ardından tüm testler geçmelidir.

// Kural numaraları harfli olabilir (**2.1a**, **3.7a**); testin numarası da harfiyle
// birlikte okunur ve tanımdaki bir kurala tam olarak karşılık gelmelidir (test "3.7a ...",
// kural **3.7a**). Harf yalnızca harfli kurallar içindir; aynı kuralın birden çok testi aynı
// numarayı taşır.
var (
	ruleRe = regexp.MustCompile(`\*\*(\d+\.\d+[a-z]?)\*\*`)
	testRe = regexp.MustCompile(`(?m)^(?:test|TR_test) "(\d+\.\d+[a-z]?)\s`) // kodlu dosyada TR_ctx_test
	// Kuralın tanımı satır başında yazılır (**4.2** ...); metnin içindeki **13.1** gibi
	// geçişler başka bir bölümdeki kurala başvurudur.
	ruleDefRe     = regexp.MustCompile(`(?m)^\*\*(\d+)\.\d+[a-z]?\*\*`)
	sectionFileRe = regexp.MustCompile(`^(\d\d)_[a-z0-9_]+\.md$`)
	ruleHeadRe    = regexp.MustCompile(`^\*\*(\d+\.\d+[a-z]?)\*\*`)
	testLineRe    = regexp.MustCompile(`^- \*Test:\*\s*(.*)$`)
	testLikeRe    = regexp.MustCompile(`(?i)^\s*[-*+]\s+\**\s*test\s*\**\s*:`)
	goTestRe      = regexp.MustCompile("`Test[A-Z0-9_]\\w*`")
)

// languageSpec docs/spec/ altındaki bölüm dosyalarını (NN_ad.md) sırayla okur.
func languageSpec(t *testing.T) map[string]string {
	t.Helper()
	files, err := reporoot.Glob(reporoot.Path("docs/spec/*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sections := map[string]string{}
	for _, d := range files {
		if filepath.Base(d) == "README.md" {
			continue
		}
		if !sectionFileRe.MatchString(filepath.Base(d)) {
			t.Errorf("docs/spec/%s: bölüm dosyalarının adı NN_ad.md biçiminde olmalı", filepath.Base(d))
			continue
		}
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		sections[filepath.Base(d)] = string(data)
	}
	if len(sections) == 0 {
		t.Fatal("docs/spec/ altında bölüm dosyası bulunamadı")
	}
	return sections
}

// Her kural kendi bölümünün dosyasında tanımlanır: **4.2**, docs/spec/04_*.md'dedir. Böylece
// kural numarası dosyayı da söyler.
func TestRuleInItsOwnSection(t *testing.T) {
	for ad, text := range languageSpec(t) {
		no := sectionFileRe.FindStringSubmatch(ad)[1]
		for _, m := range ruleDefRe.FindAllStringSubmatch(text, -1) {
			n, _ := strconv.Atoi(m[1])
			if fmt.Sprintf("%02d", n) != no {
				t.Errorf("docs/spec/%s: §%d kuralı tanımlanıyor; kural docs/spec/%02d_*.md dosyasına taşınmalı", ad, n, n)
			}
		}
	}
}

// sectionTestProblems bir bölümün tanımladığı kuralları sırasıyla ve Test satırı sorunlarını
// verir. Test satırı kuralın tanım satırından sonraki maddelerde, satır başında "- *Test:*" ile
// yazılır; bir sonraki kural ya da başlık kuralı kapatır, kod çitinin (``` ya da ~~~) içine
// bakılmaz. Bölümde Test satırı varsa her kural bir Test satırı ister; yoksa (gözden geçirilmemiş
// bölüm) her kural tests/ altında bir TR testi ister. withTests numarasıyla başlayan TR testi olan
// kurallardır.
func sectionTestProblems(name, text string, withTests map[string]bool) (defined, problems []string) {
	lines := map[string]string{}
	cur, inFence := "", false
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
		case inFence:
		case strings.HasPrefix(line, "#"):
			cur = ""
		case ruleHeadRe.MatchString(line):
			cur = ruleHeadRe.FindStringSubmatch(line)[1]
			defined = append(defined, cur)
		case testLineRe.MatchString(line) && cur == "":
			problems = append(problems, fmt.Sprintf("docs/spec/%s:%d: Test satırı bir kuralın altında değil", name, i+1))
		case testLineRe.MatchString(line):
			if _, dup := lines[cur]; dup {
				problems = append(problems, fmt.Sprintf("docs/spec/%s: **%s** kuralının birden çok Test satırı var", name, cur))
			}
			lines[cur] = testLineRe.FindStringSubmatch(line)[1]
		case testLikeRe.MatchString(line):
			problems = append(problems, fmt.Sprintf("docs/spec/%s:%d: Test satırı biçime uymuyor, satır başında \"- *Test:*\" olmalı: %s", name, i+1, trimmed))
		}
	}
	for _, k := range defined {
		line, ok := lines[k]
		switch {
		case ok:
			if p := testLineProblem(line, withTests[k]); p != "" {
				problems = append(problems, fmt.Sprintf("**%s** Test satırı: %s", k, p))
			}
		case len(lines) > 0:
			problems = append(problems, fmt.Sprintf("**%s**: Test satırı yok (docs/spec/%s Test satırı biçiminde)", k, name))
		case !withTests[k]:
			problems = append(problems, fmt.Sprintf("**%s**: Test satırı da tests/ altında testi de yok", k))
		}
	}
	return defined, problems
}

// testLineProblem kuralın Test satırındaki sorunu verir (yoksa ""); trTest kuralın tests/
// altında numarasıyla başlayan bir testi olup olmadığıdır. Satır "TR testi", "Go:" ya da
// "gerekmez" ile başlar.
func testLineProblem(line string, trTest bool) string {
	saysTR := strings.HasPrefix(line, "TR testi")
	if rest, ok := strings.CutPrefix(line, "gerekmez"); ok {
		switch {
		case trTest:
			return `"gerekmez" diyor ama tests/ altında testi var`
		case strings.Trim(rest, " ,;:.") == "":
			return `"gerekmez" nedenini yazmalı`
		}
		return ""
	}
	switch {
	case saysTR && !trTest:
		return `"TR testi" diyor ama tests/ altında numarasıyla başlayan test yok`
	case trTest && !saysTR:
		return `tests/ altında testi var; Test satırı "TR testi" diye anmalı`
	case !saysTR && !(strings.HasPrefix(line, "Go:") && goTestRe.MatchString(line)):
		return "Test satırı \"TR testi\", \"Go: `TestX`\" ya da \"gerekmez: neden\" ile başlamalı"
	}
	return ""
}

// Bölümün Test satırları okunurken kaçabilecek yazımlar yakalanır: biçime uymayan ya da girintili
// Test satırı, çitin içindeki satır, biçime geçmiş bölümde Test satırı unutulmuş kural.
func TestSectionTestProblems(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string // sorunlarda geçmesi gereken parçalar, sırayla
	}{
		{"biçime geçmiş bölüm",
			"**9.1** a\n- *Test:* TR testi.\n\n**9.2** b\n- *Emsal:* yok.\n", []string{"**9.2**: Test satırı yok"}},
		{"yanlış yazılmış satır, TR testi var",
			"**9.1** a\n- **Test:** gerekmez.\n", []string{"biçime uymuyor"}},
		{"girintili satır",
			"**9.1** a\n- *Test:* TR testi.\n\n**9.2** b\n  - *Test:* TR testi.\n", []string{"biçime uymuyor", "**9.2**: Test satırı yok"}},
		{"çitin içi okunmaz",
			"**9.1** a\n- *Test:* TR testi.\n~~~\n- *Test:* gerekmez.\n~~~\n", nil},
		{"başlıktan sonra kurala bağlı değil",
			"**9.1** a\n- *Test:* TR testi.\n## Başlık\n- *Test:* TR testi.\n", []string{"bir kuralın altında değil"}},
		{"gözden geçirilmemiş bölüm TR testiyle geçer",
			"**9.1** a\n\n**9.2** b\n", []string{"**9.2**: Test satırı da tests/ altında testi de yok"}},
	}
	withTests := map[string]bool{"9.1": true}
	for _, c := range cases {
		_, problems := sectionTestProblems("09_x.md", strings.ReplaceAll(c.text, "\n", "\r\n"), withTests)
		if len(problems) != len(c.want) {
			t.Errorf("%s: %d sorun, beklenen %d: %q", c.name, len(problems), len(c.want), problems)
			continue
		}
		for i, w := range c.want {
			if !strings.Contains(problems[i], w) {
				t.Errorf("%s: %q, beklenen parça %q", c.name, problems[i], w)
			}
		}
	}
}

// reviewedRe gözden geçirilmiş (Test satırı biçimine geçmiş) bölümü tanır.
var reviewedRe = regexp.MustCompile(`(?m)^- \*Test:\*`)

// statusDoc dil tanımına göre bugünkü durumu ve yapılacakları tutan docs/status.md'yi okur.
func statusDoc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("docs/status.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// statusSectionRe docs/status.md'deki bölüm başlığını (## 1. Program) tanır.
var statusSectionRe = regexp.MustCompile(`(?m)^## (\d+)\. `)

// Gözden geçirilmiş bölüm yalnız kuralı yazar (Ahmet, 2026-10-27: "bunlar bizim belirlediğimiz
// kurallar. bu kurallara göre mevcut durum analizi ayrı bir konu"; "status.md gibi tek bir
// dosya"): bölümde *Durum:* satırı yoktur; bugünkü durum ve yapılacaklar docs/status.md'de
// bölümün başlığı altındadır.
func TestReviewedSectionsHoldOnlyRules(t *testing.T) {
	headings := map[int]bool{}
	for _, m := range statusSectionRe.FindAllStringSubmatch(statusDoc(t), -1) {
		n, _ := strconv.Atoi(m[1])
		headings[n] = true
	}
	for name, text := range languageSpec(t) {
		if !reviewedRe.MatchString(text) {
			continue
		}
		for i, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "- *Durum:*") {
				t.Errorf("docs/spec/%s:%d: Durum satırı; bugünkü durum docs/status.md'ye yazılır", name, i+1)
			}
		}
		n, _ := strconv.Atoi(sectionFileRe.FindStringSubmatch(name)[1])
		if !headings[n] {
			t.Errorf("docs/spec/%s gözden geçirilmiş, ama docs/status.md'de \"## %d. ...\" başlığı yok", name, n)
		}
	}
}

// Gözden geçirilmiş (Test satırı biçimine geçmiş) bölüm dilin sözcüklerini kimlikle yazar
// (Ahmet, 2026-10-27: "niye kodlar ile konuşmuyorsun"; K3): madde ve Test satırlarında ve
// durum belgesinin (docs/status.md) her satırında ters tırnak içindeki bir sözcük bir kodun
// herhangi bir gösterim dilindeki adı olamaz (`aktar`, `import` değil `TR_import`). Kimlik kapısı:
// TR_ ile yazılan her sözcük bir kimliktir (identityWord; sınıflı eski yazım `TR_fn_str` değildir),
// onaylı yeni adlar ve bilerek yanlış örnekler identityExceptions'tadır.
// Emsal ve Gerekçe satırları başka dillerin kodunu bilerek yazar (Python'un `if`'i, C#'ın
// `return`'ü), bakılmaz; kod çitleri belgelerin kodlu saklanmasıyla (E4) gelecek. Bölümler
// gözden geçirildikçe kapsama girer.
func TestReviewedSectionsWriteIdentities(t *testing.T) {
	var langs []*codes.Language
	for _, l := range codes.Languages() {
		lang, err := codes.LoadLanguage(l)
		if err != nil {
			t.Fatal(err)
		}
		langs = append(langs, lang)
	}
	// Kimliği olmayan bir ad kimlikle yazılamaz; kimliği gelince buradan silinir.
	noIdentity := map[string]string{"tr test": "tr test komutunun kimliği yok (birikme listesi D1-12)"}
	spans := regexp.MustCompile("`([^`\n]+)`")
	words := regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)
	// check bir belgenin seçilen satırlarında kimlik yerine yazılmış adları yakalar.
	check := func(path, text string, selected func(line string) bool) {
		inFence := false
		for i, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = !inFence
				continue
			}
			if inFence || !selected(line) {
				continue
			}
			for _, m := range spans.FindAllStringSubmatch(line, -1) {
				if _, ok := noIdentity[m[1]]; ok {
					continue
				}
				for _, w := range words.FindAllString(m[1], -1) {
					if strings.HasPrefix(w, codes.IdentityPrefix) {
						if w != codes.IdentityPrefix && !identityWord(w) && identityExceptions[w] == "" {
							t.Errorf("%s:%d: `%s` bir kimlik değil: dilin sözcüğü TR_ ve kimliğin adıyla yazılır (2.3a); onaylı yeni ad ya da bilerek yanlış örnekse identityExceptions'a gerekçesiyle eklenir", path, i+1, w)
						}
						continue
					}
					for _, lang := range langs {
						for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType, codes.NamespaceMember, codes.NamespaceCommand} {
							if k, ok := lang.Find(ns, w); ok {
								t.Errorf("%s:%d: `%s` bir kodun adı (%s); kimliğiyle yazılır", path, i+1, w, k)
							}
						}
					}
				}
			}
		}
	}
	for name, text := range languageSpec(t) {
		if !reviewedRe.MatchString(text) {
			continue
		}
		check("docs/spec/"+name, text, func(line string) bool {
			return ruleHeadRe.MatchString(line) || strings.HasPrefix(line, "- *Test:*")
		})
	}
	check("docs/status.md", statusDoc(t), func(string) bool { return true })
}

// identityWord w'nin bir kimlik olduğudur: programın bir yerinde (öbür, üye, adlı argüman) geçerli
// yazım (TR_print, TR_map, TR_lr) ya da programın dışındaki sınıflı ad (TR_cmd_test, 2.3a). Bir parçanın
// (`TR_lr: 0.003`) yeri eksik olabilir; yeri yazımın kendisi değil kuralın metni söyler.
func identityWord(w string) bool {
	for _, p := range []codes.Place{codes.PlaceOther, codes.PlaceMember, codes.PlaceArg} {
		if _, ok := codes.FromSpelling(p, w); ok {
			return true
		}
	}
	if k, err := codes.Parse(w); err == nil && k != codes.None {
		_, inProgram := codes.PlaceOf(k)
		return !inProgram
	}
	return false
}

// identityExceptions kimlik listesinde (henüz) olmayan ama tanımda anılan TR_ sözcükleridir: onaylı
// yeni adlar (kodu kendi katmanıyla gelir; gelince buradan silinir) ve bilerek yanlış yazılan örnekler.
var identityExceptions = map[string]string{
	"TR_JSON":      "onaylı yeni ad (3.7a)",
	"TR_Bytes":     "onaylı yeni ad (3.3)",
	"TR_NoneType":  "onaylı yeni ad (3.11)",
	"TR_enum":      "onaylı yeni ad (7.x, durumlu tip)",
	"TR_interface": "onaylı yeni ad (7.x, arayüz)",
	"TR_while":     "bilerek yanlış: kalkan sözcük (2.9)",
	"TR_default":   "bilerek yanlış: kalkan sözcük (2.9)",
	"TR_elif":      "bilerek yanlış: başka dilin yazımı (5.5)",
	"TR_prnt":      "bilerek yanlış: önerinin örneği (2.3a)",
	"TR_Flaot":     "bilerek yanlış: önerinin örneği (2.3a)",
}

// Test satırının üç biçimi tanınır, eksik ya da çelişkili satır yakalanır.
func TestRuleTestLineForms(t *testing.T) {
	cases := []struct {
		line   string
		trTest bool
		ok     bool
	}{
		{"TR testi.", true, true},
		{"TR testi.", false, false},
		{"Go: `TestExampleErrors`; hatadan önceki satırın çalışmadığı da sınanır.", false, true},
		{"Go: `TestA`, `TestB`.", true, false},
		{"TR testi ve Go: `TestA`.", true, true},
		{"Go: `TestExampleErrors`; TR testi gerekmez.", false, true},
		{"gerekmez; davranış süreye bağlı.", false, true},
		{"gerekmez.", false, false},
		{"gerekmez, henüz uygulanmadı.", true, false},
		{"Go testi var.", false, false},
		{"", false, false},
	}
	for _, c := range cases {
		if got := testLineProblem(c.line, c.trTest) == ""; got != c.ok {
			t.Errorf("%q (TR testi var: %v): geçerli %v, beklenen %v", c.line, c.trTest, got, c.ok)
		}
	}
}

func TestEveryRuleHasATest(t *testing.T) {
	spec := languageSpec(t)
	rules := map[string]bool{}
	var names []string
	for name, text := range spec {
		names = append(names, name)
		for _, m := range ruleRe.FindAllStringSubmatch(text, -1) {
			rules[m[1]] = true
		}
	}
	sort.Strings(names)
	if len(rules) < 50 {
		t.Fatalf("tanımda yalnızca %d kural bulundu; biçim değişmiş olabilir", len(rules))
	}

	// Olumsuz fikstürün '# TEST 2.6 ...' satırı da tests/ altındaki bir TR testidir
	// (TestNegativeFixtures).
	files, _ := reporoot.Glob(reporoot.Path("tests/*_test.tr"))
	fixtures, _ := reporoot.Glob(reporoot.Path("tests/negative/*.tr"))
	withTests := map[string]bool{}
	for _, d := range append(files, fixtures...) {
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		re := testRe
		if filepath.Base(filepath.Dir(d)) == "negative" {
			re = fixtureTestRe
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			withTests[m[1]] = true
			if !rules[m[1]] {
				t.Errorf("%s: %s numaralı test tanımda bir kurala karşılık gelmiyor", filepath.Base(d), m[1])
			}
		}
	}

	defined := map[string]bool{}
	for _, name := range names {
		ks, problems := sectionTestProblems(name, spec[name], withTests)
		for _, k := range ks {
			defined[k] = true
		}
		for _, p := range problems {
			t.Error(p)
		}
	}
	// Tanımı olmayan bir numaranın kalın yazıldığı anma (**13.1**) da bir test ister: kural ya
	// tanımlanır ya da anma düzeltilir.
	var missing []string
	for k := range rules {
		if !defined[k] && !withTests[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("tanımı ve testi olmayan kural numaraları: %v", missing)
	}
	// Harfli kurallar kapıdan kaçmamalı (eski düzenli ifade onları hiç saymıyordu).
	for _, k := range []string{"2.1a", "3.7a"} {
		if !rules[k] {
			t.Errorf("harfli kural %s tanımda bulunamadı; kapının düzenli ifadesi harfli kuralları kapsamalı", k)
		}
	}
}

// Olumsuz fikstürler (tests/negative/*.tr) bilerek hatalı dosyalardır. Beklenti dosyanın içinde,
// hatalı satırın sonunda yorumla yazılır: `# ERROR H1106` denetimin (sözdizimi ya da tip) o satırda
// verdiği hata, `# WARNING H1012` uyarı, `# RUN-ERROR H2478` denetimden geçen dosyanın çalışırken
// o satırda verdiği hata; bir satırda birden çok kod boşlukla yazılır. Her fikstürde en az bir
// `# TEST 2.6 açıklama` satırı sınadığı kuralı söyler (TestEveryRuleHasATest onu tests/ altındaki
// bir TR testi sayar). Aktarmalar dosyanın başında olduğundan (2.1) bir TR testi hatalı bir modülü
// içeride aktarıp kodunu yakalayamaz.
var (
	fixtureLabelRe = regexp.MustCompile(`#\s*(ERROR|WARNING|RUN-ERROR)((?:\s+H\d{4})+)\s*$`)
	fixtureBareRe  = regexp.MustCompile(`#\s*(ERROR|WARNING|RUN-ERROR)\b`)
	fixtureTestRe  = regexp.MustCompile(`(?m)^# TEST (\d+\.\d+[a-z]?)\s`)
)

// fixtureDiag bir fikstürün beklenen ya da alınan tanısıdır: tür (ERROR, WARNING, RUN-ERROR),
// satır ve kod.
type fixtureDiag struct {
	kind string
	line int
	code string
}

func (d fixtureDiag) String() string { return fmt.Sprintf("%d: %s %s", d.line, d.kind, d.code) }

// fixtureLabels fikstürün etiketlerini satır sırasıyla okur; problems biçime uymayan etiketlerdir.
func fixtureLabels(text string) (want []fixtureDiag, problems []string) {
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		m := fixtureLabelRe.FindStringSubmatch(line)
		switch {
		case m == nil && fixtureBareRe.MatchString(line):
			problems = append(problems, fmt.Sprintf("%d. satır: etiket satırın sonunda '# ERROR H1234' biçiminde yazılır", i+1))
		case m == nil:
		case strings.TrimSpace(line) == strings.TrimSpace(m[0]):
			problems = append(problems, fmt.Sprintf("%d. satır: etiket tek başına yazılmaz, hatalı satırın sonuna yazılır", i+1))
		default:
			for _, code := range strings.Fields(m[2]) {
				want = append(want, fixtureDiag{m[1], i + 1, code})
			}
		}
	}
	return want, problems
}

// fixtureDiags tanıları fikstürün etiketleriyle karşılaştırılacak biçime çevirir: kod en içteki
// nedenin kodudur (TR_error_code gibi), satır dosyanın satırıdır.
func fixtureDiags(kind string, errs []error) []fixtureDiag {
	var out []fixtureDiag
	for _, err := range errs {
		line, _, _ := messages.Pos(err)
		var me *interp.ModuleError
		if errors.As(err, &me) {
			line = me.ImportLine
		}
		code := "?"
		if m, ok := messages.Root(err); ok {
			code = m.Code.String()
		}
		out = append(out, fixtureDiag{kind, line, code})
	}
	return out
}

// Her fikstür etiketlerinin söylediği tanıları, yalnız onları verir. Denetim dosyayı modül olarak
// okur (fikstürler eskiden aktarılarak sınanırdı; modülün en dışındaki TR_return hatadır). RUN-ERROR
// etiketli fikstür denetimden hatasız geçer, sonra çalıştırılır: hata o satırda gelir ve hatadan
// önce hiçbir çıktı yazılmaz (hatalı modülün hiçbir satırı çalışmaz).
func TestNegativeFixtures(t *testing.T) {
	fixtures, _ := reporoot.Glob(reporoot.Path("tests/negative/*.tr"))
	if len(fixtures) == 0 {
		t.Fatal("tests/negative/ altında fikstür bulunamadı")
	}
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	for _, f := range fixtures {
		name := "tests/negative/" + filepath.Base(f)
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		want, problems := fixtureLabels(text)
		for _, p := range problems {
			t.Errorf("%s: %s", name, p)
		}
		if len(want) == 0 {
			t.Errorf("%s: etiket yok; beklenen hata satırın sonunda '# ERROR H1234' diye yazılır", name)
		}
		if !fixtureTestRe.MatchString(text) {
			t.Errorf("%s: '# TEST 2.6 açıklama' satırı yok; fikstür sınadığı kuralı söyler", name)
		}
		run := false
		for _, w := range want {
			run = run || w.kind == "RUN-ERROR"
		}
		r := source.Prepare(text, source.Options{TypeCheck: true, Module: !run})
		got := append(fixtureDiags("ERROR", r.Errors), fixtureDiags("WARNING", r.Warnings)...)
		if run && r.Stage == "" {
			m := interp.New()
			var out bytes.Buffer
			m.Out, m.Err = &out, &out
			m.SourceFile, m.SourceCode = f, r.Text
			_, runErr := m.Run(r.Program)
			m.WaitAsync()
			if runErr != nil {
				got = append(got, fixtureDiags("RUN-ERROR", []error{runErr})...)
			}
			if out.Len() > 0 {
				t.Errorf("%s: hatadan önce çıktı yazıldı: %q", name, out.String())
			}
		}
		sortFixtureDiags(want)
		sortFixtureDiags(got)
		if fmt.Sprint(want) != fmt.Sprint(got) {
			t.Errorf("%s:\n  beklenen: %v\n  alınan:   %v", name, want, got)
		}
	}
}

func sortFixtureDiags(d []fixtureDiag) {
	sort.Slice(d, func(i, j int) bool {
		if d[i].line != d[j].line {
			return d[i].line < d[j].line
		}
		return d[i].kind+d[i].code < d[j].kind+d[j].code
	})
}

// Etiketin biçimi: satırın sonunda bir ya da birden çok kod; tek başına ya da kodsuz etiket
// yakalanır.
func TestFixtureLabels(t *testing.T) {
	want, problems := fixtureLabels("# TR 4\nTR_var a = \"\\q\" # ERROR H0008 H0008\nTR_var b = 1 # WARNING H1012\r\nTR_import \"x\" # RUN-ERROR H2478\n")
	if fmt.Sprint(want) != "[2: ERROR H0008 2: ERROR H0008 3: WARNING H1012 4: RUN-ERROR H2478]" || len(problems) != 0 {
		t.Errorf("etiketler: %v, sorunlar: %v", want, problems)
	}
	for _, text := range []string{"# ERROR H0008\nTR_var a = 1\n", "TR_var a = 1 # ERROR\n", "TR_var a = 1 # ERROR 0008\n"} {
		if _, problems := fixtureLabels(text); len(problems) != 1 {
			t.Errorf("%q: bir sorun bekleniyordu: %v", text, problems)
		}
	}
}

// Dil tanımının bütün testleri (tests/) yorumlayıcının içinden koşar. Tam denetim
// (tools/full_check.sh) Go testlerini -short ile koşar ve aynı takımı bir kez
// `tr test tests` ile koşar; takım iki derleme ayarında üç kez koşmasın diye burada atlanır.
func TestLanguageSpecTestsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: dil tanımı testleri tam denetimde `tr test tests` ile koşar")
	}
	// Komut satırındaki gibi dilin kitaplığı bağlanır (11.6: aktar "tr/...").
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	var out bytes.Buffer
	summary, err := Run(reporoot.Path("tests"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.OK() {
		t.Fatalf("dil tanımı testleri kaldı: %s\n%s", summary, out.String())
	}
}
