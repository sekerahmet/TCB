package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestLogLenFileError(t *testing.T) {
	secretDir := filepath.Join(t.TempDir(), "gizli_alt_klasor")
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &Log{
		path:     "egitim.jsonl",
		fullPath: filepath.Join(secretDir, "olmayan.jsonl"),
	}
	_, err := g.Len()
	if !messages.Has(err, messages.H2100) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("H2100 ve ErrNotExist bekleniyordu: %v", err)
	}
	root, ok := messages.Root(err)
	if !ok || root.Code != messages.H2132 {
		t.Fatalf("kök kod H2132 bekleniyordu, gelen: %v (%v)", root, ok)
	}
	for _, lang := range []string{"tr", "en"} {
		text := messages.ErrorIn(err, lang)
		if !strings.Contains(text, "egitim.jsonl") || strings.Contains(text, "gizli_alt_klasor") {
			t.Fatalf("%s iletisi kullanıcı yolunu göstermeli, tam disk yolunu sızdırmamalı: %q", lang, text)
		}
	}
	if low := strings.ToLower(messages.ErrorIn(err, "tr")); strings.Contains(low, "no such file") || strings.Contains(low, "cannot find") {
		t.Fatalf("TR iletisinde ham sistem metni: %q", low)
	}
}
