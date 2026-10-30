package testrunner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/lib"
)

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

// Her fikstür etiketlerinin söylediği tanıları, yalnız onları verir. Başka bir fikstürün aktardığı
// fikstür modül olarak (en dışında TR_return yazılmaz, yalnız tanım bulunur; 1.1, 11.4), öbürleri
// program olarak denetlenir: aynı dosya aktarılınca modüldür. RUN-ERROR etiketli fikstür denetimden
// hatasız geçer, sonra çalıştırılır: hata o satırda gelir ve hatadan önce hiçbir çıktı yazılmaz
// (hatalı modülün hiçbir satırı çalışmaz).
func TestNegativeFixtures(t *testing.T) {
	fixtures, _ := reporoot.Glob(reporoot.Path("tests/negative/*.tr"))
	if len(fixtures) == 0 {
		t.Fatal("tests/negative/ altında fikstür bulunamadı")
	}
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	modules := importedFixtures(t, fixtures)
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
		r := source.Prepare(text, source.Options{TypeCheck: true, Module: modules[filepath.Base(f)]})
		got := append(fixtureDiags("ERROR", r.Errors), fixtureDiags("WARNING", r.Warnings)...)
		if run && r.Stage == "" {
			m := interp.New()
			var out bytes.Buffer
			m.Out, m.Err = &out, &out
			m.SourceFile, m.SourceCode = f, r.Text
			runErr := interp.CheckImportedModules(f, r.Program)
			if runErr == nil {
				_, runErr = m.Run(r.Program)
				m.WaitAsync()
			}
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

// importedFixtures fikstürlerin aktardığı fikstürlerin adlarıdır (TR_import "tip_hatali_modul").
func importedFixtures(t *testing.T, fixtures []string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range fixtureImportRe.FindAllStringSubmatch(string(data), -1) {
			out[strings.TrimSuffix(m[1], ".tr")+".tr"] = true
		}
	}
	return out
}

var fixtureImportRe = regexp.MustCompile(`(?m)^TR_import "([^"/]+)"`)

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
