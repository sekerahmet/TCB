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

// Hata örnekleri: examples/tr/hatalar ve examples/en/errors. Her dosya bir hatayı bilerek yapar; ilk
// yorum satırlarından biri beklenen ileti kodunu söyler ("İleti kodu: H1010" / "Message code: H1010").
// Testler ileti metnine değil koda ve çıkış koduna bakar (iletiler dile göre değişir).

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

// Her hata örneğinin Türkçe ve İngilizce eşi vardır, aynı ileti kodunu söyler ve süreç içinde gerçekten
// o kodlu iletiyi üretir; ikili 1 koduyla çıkar ve hatayı standart hataya yazar.
func TestExampleErrors(t *testing.T) {
	turkish, english := errorExamples(t, "tr", "hatalar"), errorExamples(t, "en", "errors")
	if len(turkish) < 20 {
		t.Fatalf("yalnızca %d hata örneği var", len(turkish))
	}
	for n, trPath := range turkish {
		enPath, ok := english[n]
		if !ok {
			t.Errorf("%s'in examples/en/errors'da eşi yok", filepath.Base(trPath))
			continue
		}
		t.Run(n, func(t *testing.T) {
			want := expectedCode(t, trPath)
			if got := expectedCode(t, enPath); got != want {
				t.Errorf("%s ile %s farklı ileti kodu söylüyor: %s / %s", filepath.Base(trPath), filepath.Base(enPath), want, got)
			}
			for _, p := range []string{trPath, enPath} {
				found := false
				for _, e := range producedErrors(t, p) {
					found = found || messages.Has(e, want)
				}
				if !found {
					t.Errorf("%s: %s kodlu bir ileti üretmedi", filepath.Base(p), want)
				}
			}
			for lang, p := range map[string]string{"tr": trPath, "en": enPath} {
				rel, _ := filepath.Rel(reporoot.Path(), p)
				s := testkit.RunBinary(t, trBinary, reporoot.Path(), "", []string{"TR_DIL=" + lang}, filepath.ToSlash(rel))
				if s.Code != 1 || !strings.Contains(s.Stderr, "❌") {
					t.Errorf("%s: çıkış kodu %d (beklenen 1), hata çıktısı:\n%s", rel, s.Code, s.Stderr)
				}
			}
		})
	}
	for n, enPath := range english {
		if _, ok := turkish[n]; !ok {
			t.Errorf("%s'in examples/tr/hatalar'da eşi yok", filepath.Base(enPath))
		}
	}
}

// Kodlanabilen (sözdizimi hatası olmayan) hata örneklerinin iki yazımı aynı programdır.
func TestExampleErrorsSameStructure(t *testing.T) {
	turkish, english := errorExamples(t, "tr", "hatalar"), errorExamples(t, "en", "errors")
	for n, trPath := range turkish {
		data, _ := os.ReadFile(trPath)
		coded, err := source.Encode(string(data))
		if err != nil {
			continue // sözdizimi hatası olan dosya kodlanamaz
		}
		enData, _ := os.ReadFile(english[n])
		if i := firstDifference(structure(coded), structure(string(enData))); i >= 0 {
			t.Errorf("%s ile %s farklı program (%d. sözcük birimi)", filepath.Base(trPath), filepath.Base(english[n]), i)
		}
	}
}
