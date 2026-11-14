package dataset_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
)

func TestDatasetAndStateSystemErrorsCarryNoSystemText(t *testing.T) {
	testDatasetMissingPathError(t)
	testDatasetCacheRenameLinkError(t)
	testDatasetLoadStateWrapperRootCode(t)
}

func testDatasetMissingPathError(t *testing.T) {
	t.Helper()
	missing := trPath(filepath.Join(t.TempDir(), "olmayan_kume"))
	out, _ := mustRun(t, `
TR_print(TR_error_code { TR_Dataset("`+missing+`") })
TR_try {
    TR_Dataset("`+missing+`")
} TR_catch e {
    TR_print(e.TR_code)
    TR_print(TR_String(e.TR_value))
    TR_print(e.TR_text("tr"))
    TR_print(e.TR_text("en"))
}
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 || lines[0] != "H2132" || lines[1] != "H2132" || lines[2] != `Message("H2132")` {
		t.Fatalf("unexpected missing-dataset catch output: %q", out)
	}
	if lines[3] != messages.TextIn("tr", messages.H2132) || lines[4] != messages.TextIn("en", messages.H2132) {
		t.Fatalf("unexpected localized text: tr=%q en=%q", lines[3], lines[4])
	}
}

func testDatasetCacheRenameLinkError(t *testing.T) {
	t.Helper()
	dsDir := buildDataset(t)
	cacheDir := filepath.Join(t.TempDir(), "onb")
	mustRun(t, `TR_Dataset("`+trPath(dsDir)+`", TR_cache: "`+trPath(cacheDir)+`", TR_quota_gb: 0.001).TR_sequential(TR_batch_size: 1, TR_context: 8).TR_next()`)
	dataset.CloseAll(time.Second)

	entries, err := reporoot.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	replaced := 0
	for _, g := range entries {
		if !strings.HasSuffix(g.Name(), ".cache.npy") {
			continue
		}
		p := filepath.Join(cacheDir, g.Name())
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(p, "engel"), 0o755); err != nil {
			t.Fatal(err)
		}
		replaced++
	}
	if replaced == 0 {
		t.Fatal("no cached shard found to replace with directory")
	}

	readExpr := `TR_Dataset("` + trPath(dsDir) + `", TR_cache: "` + trPath(cacheDir) + `", TR_quota_gb: 0.001).TR_sequential(TR_batch_size: 1, TR_context: 8).TR_next()`
	_, _, runErr := run(t, readExpr)
	if !messages.Has(runErr, messages.H2000) || !messages.Has(runErr, messages.H5013) {
		t.Fatalf("expected H2000 wrapping H5013, got: %v", runErr)
	}
	if root, ok := messages.Root(runErr); !ok || root.Code != messages.H2130 {
		t.Fatalf("expected Root H2130, got %v (ok=%v): %v", root, ok, runErr)
	}
	dataset.CloseAll(time.Second)

	out2, _ := mustRun(t, `
TR_print(TR_error_code { `+readExpr+` })
TR_try {
    `+readExpr+`
} TR_catch e {
    TR_print(e.TR_code)
    TR_print(TR_String(e.TR_value))
    TR_print(e.TR_text("tr"))
    TR_print(e.TR_text("en"))
}
`)
	dataset.CloseAll(time.Second)
	lines2 := strings.Split(strings.TrimSpace(out2), "\n")
	if len(lines2) != 5 || lines2[0] != "H2130" || lines2[1] != "H2130" || lines2[2] != `Message("H2130")` {
		t.Fatalf("unexpected LinkError catch output: %q", out2)
	}
	if lines2[3] != messages.TextIn("tr", messages.H2130) || lines2[4] != messages.TextIn("en", messages.H2130) {
		t.Fatalf("unexpected localized LinkError text: tr=%q en=%q", lines2[3], lines2[4])
	}
}

func testDatasetLoadStateWrapperRootCode(t *testing.T) {
	t.Helper()
	dsDir := buildDataset(t)
	prog := `
TR_var k = TR_Dataset("` + trPath(dsDir) + `")
TR_var o = k.TR_sequential(TR_batch_size: 1, TR_context: 8)
TR_var d = o.TR_state
d["position"] = 99
o.TR_load_state(d)
`
	_, _, err := run(t, prog)
	if !messages.Has(err, messages.H5540) {
		t.Fatalf("expected outer H5540, got %v", err)
	}
	if root, ok := messages.Root(err); !ok || root.Code != messages.H5079 {
		t.Fatalf("expected Root H5079, got %v (ok=%v): %v", root, ok, err)
	}
	out, _ := mustRun(t, `
TR_var k = TR_Dataset("`+trPath(dsDir)+`")
TR_var o = k.TR_sequential(TR_batch_size: 1, TR_context: 8)
TR_var d = o.TR_state
d["position"] = 99
TR_print(TR_error_code { o.TR_load_state(d) })
`)
	if strings.TrimSpace(out) != "H5079" {
		t.Fatalf("expected TR_error_code H5079, got %q", out)
	}
}
