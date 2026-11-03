package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// TestHelperSkippedReasons alt süreç yardımcısıdır: verilen klasörde dizini istenen kipte kurar ya da önbellekten
// okur ve atlanan dosyaların nedenlerini yazar.
func TestHelperSkippedReasons(t *testing.T) {
	out := os.Getenv("CODEINDEX_SKIPPED_OUT")
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	dir := os.Getenv("CODEINDEX_SKIPPED_DIR")
	mode := os.Getenv("CODEINDEX_SKIPPED_MODE")
	var (
		idx *Index
		st  Stats
		err error
	)
	switch mode {
	case "build":
		idx, err = Build(dir, Options{})
	case "build_cached":
		idx, st, err = BuildCached(dir, Options{})
		if err == nil && !st.Full {
			t.Fatalf("expected full cache build, got %+v", st)
		}
	case "from_cache":
		idx, st, err = BuildCached(dir, Options{})
		if err == nil && !st.FromCache {
			t.Fatalf("expected FromCache hit, got %+v", st)
		}
	case "load_cached":
		c := loadCache(dir, Options{})
		if c == nil {
			t.Fatal("loadCache returned nil")
		}
		idx = c.index
	default:
		t.Fatalf("unknown mode %q", mode)
	}
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, s := range idx.Skipped {
		lines = append(lines, s.File+"\t"+s.Reason)
	}
	if err := os.WriteFile(filepath.Clean(out), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runSkippedHelper(t *testing.T, dir, mode, lang string) string {
	t.Helper()
	t.Setenv("CODEINDEX_SKIPPED_DIR", dir)
	t.Setenv("CODEINDEX_SKIPPED_MODE", mode)
	t.Setenv("TR_LANG", lang)
	return string(testkit.RunWithLanguageFile(t, "TestHelperSkippedReasons", "CODEINDEX_SKIPPED_OUT", lang))
}

// Kural (DENETİM istek 66): önbellek atlanan dosya nedeninin metnini değil, ileti kodunu ve parametrelerini tutar;
// neden metni önbelleği okuyanın dilinde (TR_LANG) kurulur. Sürümü eski önbellek okunmaz, yeniden kurulur.
func TestCachedSkippedReasonFollowsReadersLanguage(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"ok.tr":       "# TR 4\nTR_const X = 1\n",
		"unclosed.tr": "# TR 4\nTR_func f() {\n",
		"syntax.tr":   "# TR 4\nTR_var = 1\n",
		"noheader.tr": "TR_print(1)\n",
		"view.tr":     "# TR 4 view=tr\nyazdır(1)\n",
		"classed.tr":  "# TR 4\nTR_fn_print(1)\n",
	} {
		testkit.WriteFile(t, dir, name, content)
	}

	wantTR := runSkippedHelper(t, dir, "build", "tr")
	wantEN := runSkippedHelper(t, dir, "build", "en")
	if wantTR == "" || wantEN == "" || wantTR == wantEN {
		t.Fatalf("expected distinct non-empty Turkish and English reasons:\nTurkish:\n%s\nEnglish:\n%s", wantTR, wantEN)
	}

	// TR_LANG="tr" ile kurulan önbellek TR_LANG="en" ile okununca İngilizce metin üretir; diskte metin yoktur.
	if got := runSkippedHelper(t, dir, "build_cached", "tr"); got != wantTR {
		t.Fatalf("build_cached Turkish mismatch:\ngot:\n%s\nwant:\n%s", got, wantTR)
	}
	rawCache, err := os.ReadFile(filepath.Join(dir, CacheDir, indexName))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Satır ", "Sütun ", "Line ", "Column ", `"reason"`} {
		if strings.Contains(string(rawCache), forbidden) {
			t.Errorf("index cache stores localized reason text %q:\n%s", forbidden, rawCache)
		}
	}
	for _, mode := range []string{"from_cache", "load_cached"} {
		gotEN := runSkippedHelper(t, dir, mode, "en")
		if gotEN != wantEN {
			t.Errorf("Turkish-built cache read in English (%s):\ngot:\n%s\nwant:\n%s", mode, gotEN, wantEN)
		}
		if strings.Contains(gotEN, "Satır ") || !strings.Contains(gotEN, "Line ") || !strings.Contains(gotEN, "TR_fn_print") {
			t.Errorf("English output from Turkish cache (%s) missing expected English/verbatim text:\n%s", mode, gotEN)
		}
	}

	// Tersi: TR_LANG="en" ile kurulan önbellek TR_LANG="tr" ile okununca Türkçe metin üretir.
	if _, err := ClearCache(dir); err != nil {
		t.Fatal(err)
	}
	if got := runSkippedHelper(t, dir, "build_cached", "en"); got != wantEN {
		t.Fatalf("build_cached English mismatch:\ngot:\n%s\nwant:\n%s", got, wantEN)
	}
	for _, mode := range []string{"from_cache", "load_cached"} {
		gotTR := runSkippedHelper(t, dir, mode, "tr")
		if gotTR != wantTR {
			t.Errorf("English-built cache read in Turkish (%s):\ngot:\n%s\nwant:\n%s", mode, gotTR, wantTR)
		}
		if strings.Contains(gotTR, "Line ") || !strings.Contains(gotTR, "Satır ") || !strings.Contains(gotTR, "TR_fn_print") {
			t.Errorf("Turkish output from English cache (%s) missing expected Turkish/verbatim text:\n%s", mode, gotTR)
		}
	}

	// Eski IndexVersion önbelleği okunmaz, baştan kurulur.
	manPath := filepath.Join(dir, CacheDir, manifestName)
	manBytes, err := os.ReadFile(manPath)
	if err != nil {
		t.Fatal(err)
	}
	oldVer := strings.Replace(string(manBytes), fmt.Sprintf(`"index_version":%d`, IndexVersion), fmt.Sprintf(`"index_version":%d`, IndexVersion-1), 1)
	if err := os.WriteFile(manPath, []byte(oldVer), 0o644); err != nil {
		t.Fatal(err)
	}
	if loadCache(dir, Options{}) != nil {
		t.Error("loadCache accepted an older IndexVersion")
	}
	if got := runSkippedHelper(t, dir, "build_cached", "tr"); got != wantTR {
		t.Errorf("stale version rebuild mismatch:\ngot:\n%s\nwant:\n%s", got, wantTR)
	}
}

func TestSkippedReadErrorRoundTripsAsCodedMessage(t *testing.T) {
	s := newSkippedRead(t.TempDir(), "missing.tr", os.ErrNotExist)
	if s.Message == nil || s.Message.Code == "" || s.Reason == "" {
		t.Fatalf("newSkippedRead = %+v, want coded message and non-empty reason", s)
	}
	data, err := EncodeIndex(&Index{Schema: SchemaVersion, Version: IndexVersion, Skipped: []Skipped{s}})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeIndex(data)
	if err != nil || len(dec.Skipped) != 1 || dec.Skipped[0].Reason != s.Reason {
		t.Fatalf("DecodeIndex = %+v, %v; want reason %q", dec, err, s.Reason)
	}
}
