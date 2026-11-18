package tokenizer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestLoadTokenizerSystemErrors(t *testing.T) {
	missing := filepath.ToSlash(filepath.Join(t.TempDir(), "yok.json"))
	m := mustRun(t, `
TR_var code = ""
TR_var valText = ""
TR_var trText = ""
TR_var enText = ""
TR_try {
    TR_load_tokenizer("`+missing+`")
} TR_catch e {
    code = e.TR_code
    valText = TR_String(e)
    trText = e.TR_text("tr")
    enText = e.TR_text("en")
}
`)
	if got := al(t, m, "code").Inspect(); got != "H2132" {
		t.Fatalf("eksik dosyada kod = %s, H2132 bekleniyordu", got)
	}
	for _, k := range []string{"valText", "trText"} {
		low := strings.ToLower(al(t, m, k).Inspect())
		if strings.Contains(low, "no such file") || strings.Contains(low, "cannot find the file") {
			t.Fatalf("%s içinde ham sistem metni: %s", k, low)
		}
	}
	if lowEn := strings.ToLower(al(t, m, "enText").Inspect()); strings.Contains(lowEn, "cannot find the file") {
		t.Fatalf("enText içinde Windows sistem metni: %s", lowEn)
	}
	if _, err := run(t, `TR_load_tokenizer("`+missing+`")`); !messages.Has(err, messages.H2107) {
		t.Fatalf("H2107 sarmalayıcısı korunmalı (15.10): %v", err)
	}

	dir := filepath.ToSlash(t.TempDir())
	mDir := mustRun(t, `
TR_var trDir = ""
TR_try {
    TR_load_tokenizer("`+dir+`")
} TR_catch e {
    trDir = TR_String(e) + " | " + e.TR_text("tr")
}
`)
	lowDir := strings.ToLower(al(t, mDir, "trDir").Inspect())
	if strings.Contains(lowDir, "is a directory") || strings.Contains(lowDir, "access is denied") || strings.Contains(lowDir, "incorrect function") {
		t.Fatalf("klasör okumasında ham sistem metni: %s", lowDir)
	}

	cutGGUF := filepath.Join(t.TempDir(), "yarim.gguf")
	if err := os.WriteFile(cutGGUF, []byte(gguf.Magic+"\x03\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	mGGUF := mustRun(t, `
TR_var ggufCode = ""
TR_var ggufText = ""
TR_try {
    TR_load_tokenizer("`+filepath.ToSlash(cutGGUF)+`")
} TR_catch e {
    ggufCode = e.TR_code
    ggufText = TR_String(e) + " | " + e.TR_text("tr")
}
`)
	if got := al(t, mGGUF, "ggufCode").Inspect(); got != "H5620" {
		t.Fatalf("yarım GGUF kodu = %s, H5620 bekleniyordu", got)
	}
	if low := strings.ToLower(al(t, mGGUF, "ggufText").Inspect()); strings.Contains(low, "unexpected eof") {
		t.Fatalf("GGUF hatasında ham Go metni: %s", low)
	}
}
