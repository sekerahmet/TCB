package testkit

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const langFileEnv = "TESTKIT_LANG_FILE_OUT"

// TestHelperWriteLanguage bir test değil, alt süreçtir: etkin TR_LANG değerini çıktı dosyasına yazar.
func TestHelperWriteLanguage(t *testing.T) {
	out := os.Getenv(langFileEnv)
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	if err := os.WriteFile(out, []byte(os.Getenv("TR_LANG")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Kural: RunWithLanguageFile yardımcı testi verilen TR_LANG ile ayrı bir süreçte çalıştırır ve yazdığı dosyayı döner.
func TestRunWithLanguageFileRunsTheHelperInThatLanguage(t *testing.T) {
	for _, lang := range []string{"tr", "en"} {
		if got := string(RunWithLanguageFile(t, "TestHelperWriteLanguage", langFileEnv, lang)); got != lang {
			t.Errorf("TR_LANG=%s: helper saw %q", lang, got)
		}
	}
}

// fatalRecorder testkit_test.go'daki recorder'a durdurmayan bir Fatalf ekler (az olgu yolunu sınamak için).
type fatalRecorder struct {
	*recorder
	fatals []string
}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}

// Kural: AssertSameFactSet aynı olgularda sessizdir; farkta yalnızca tr'de olan olguları yazar; az olguda karşılaştırmaz.
func TestAssertSameFactSetReportsOnlyTheDifference(t *testing.T) {
	r := &fatalRecorder{recorder: &recorder{TB: t}}
	AssertSameFactSet(r, "the index", 2, []string{"a", "b"}, []string{"a", "b"})
	if len(r.errors)+len(r.fatals) != 0 {
		t.Errorf("same facts must pass: %v %v", r.errors, r.fatals)
	}
	AssertSameFactSet(r, "the index", 2, []string{"a", "b", "c"}, []string{"a", "c"})
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "TR_LANG=en changes the index (3 facts vs 2); only in tr:\nb") {
		t.Errorf("difference: %v", r.errors)
	}
	AssertSameFactSet(r, "the index", 5, []string{"a"}, []string{"a"})
	if len(r.fatals) != 1 || !strings.HasPrefix(r.fatals[0], "too few facts in the index") {
		t.Errorf("too few facts: %v", r.fatals)
	}
}
