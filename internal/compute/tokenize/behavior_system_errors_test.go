package tokenize_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestLoadSystemErrorsCarryNoSystemText(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "olmayan.json")
	_, err := tokenize.Load(missing)
	if !messages.Has(err, messages.H5272) {
		t.Fatalf("H5272 bekleniyordu: %v", err)
	}
	root, ok := messages.Root(err)
	if !ok || root.Code != messages.H2132 {
		t.Fatalf("kök kod H2132 bekleniyordu, gelen: %v (%v)", root, ok)
	}
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		t.Fatalf("*fs.PathError zincirde korunmalı: %v", err)
	}
	if tr := strings.ToLower(messages.ErrorIn(err, "tr")); strings.Contains(tr, "no such file") || strings.Contains(tr, "cannot find the file") {
		t.Fatalf("TR iletisinde ham sistem metni: %q", tr)
	}

	dir := t.TempDir()
	_, dirErr := tokenize.Load(dir)
	if !messages.Has(dirErr, messages.H5272) || !messages.HasSystemError(dirErr) {
		t.Fatalf("klasör yolunda H5272 + sistem hatası bekleniyordu: %v", dirErr)
	}
	for _, lang := range []string{"tr", "en"} {
		low := strings.ToLower(messages.ErrorIn(dirErr, lang))
		if strings.Contains(low, "is a directory") || strings.Contains(low, "access is denied") || strings.Contains(low, "incorrect function") {
			t.Fatalf("%s iletisinde ham işletim sistemi metni: %q", lang, low)
		}
	}
}
