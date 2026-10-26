package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/internal/testkit"
	_ "github.com/sekerahmet/TCB/lib"
)

// Hata örnekleri: examples/tr/hatalar. Her dosya bir hatayı bilerek yapar; ilk yorum satırlarından biri
// beklenen ileti kodunu söyler ("İleti kodu: H1010"). Testler ileti metnine değil koda ve çıkış koduna
// bakar (iletiler gösterim diline göre değişir).

var messageCodeLine = regexp.MustCompile(`(?m)^// (?:İleti kodu|Message code): (H\d{4})$`)

// errorExamples dil klasöründeki hata örneklerini numarasıyla verir.
func errorExamples(t *testing.T, lang, dir string) map[string]string {
	t.Helper()
	files, err := reporoot.Glob(reporoot.Path("examples", lang, dir, "*.tr"))
	if err != nil || len(files) == 0 {
		t.Fatalf("examples/%s/%s boş: %v", lang, dir, err)
	}
	out := map[string]string{}
	for _, f := range files {
		m := exampleNumber.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			t.Errorf("%s: ad NN_ad.tr biçiminde değil", f)
			continue
		}
		out[m[1]] = f
	}
	return out
}

func expectedCode(t *testing.T, path string) messages.Code {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := messageCodeLine.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatalf("%s: 'İleti kodu: H…' (ya da 'Message code: H…') yorum satırı yok", path)
	}
	code, err := messages.Parse(m[1])
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return code
}

// producedErrors dosyayı süreç içinde hazırlar ve çalıştırır; sözdizimi, tip ya da çalışma zamanı
// hatalarının hepsini verir.
func producedErrors(t *testing.T, path string) []error {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := source.Prepare(string(data), source.Options{TypeCheck: true})
	if r.Stage != "" {
		return r.Errors
	}
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = r.Text
	if _, err := machine.Run(r.Program); err != nil {
		return []error{err}
	}
	return nil
}

// Her hata örneği süreç içinde söylediği ileti kodunu gerçekten üretir; ikili her gösterim dilinde 1
// koduyla çıkar ve hatayı standart hataya yazar.
func TestExampleErrors(t *testing.T) {
	examples := errorExamples(t, "tr", "hatalar")
	if len(examples) < 20 {
		t.Fatalf("yalnızca %d hata örneği var", len(examples))
	}
	for n, p := range examples {
		t.Run(n, func(t *testing.T) {
			want := expectedCode(t, p)
			found := false
			for _, e := range producedErrors(t, p) {
				found = found || messages.Has(e, want)
			}
			if !found {
				t.Errorf("%s: %s kodlu bir ileti üretmedi", filepath.Base(p), want)
			}
			rel, _ := filepath.Rel(reporoot.Path(), p)
			for _, display := range []string{"tr", "en"} {
				s := testkit.RunBinary(t, trBinary, reporoot.Path(), "", []string{"TR_DIL=" + display}, filepath.ToSlash(rel))
				if s.Code != 1 || !strings.Contains(s.Stderr, "❌") {
					t.Errorf("%s (TR_DIL=%s): çıkış kodu %d (beklenen 1), hata çıktısı:\n%s", rel, display, s.Code, s.Stderr)
				}
			}
		})
	}
}
