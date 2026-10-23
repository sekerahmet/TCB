package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// auditModule küçük bir Go modülü yazar: dosya adı -> kaynak.
func auditModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "go.mod", "module example.com/audit\n\ngo 1.21\n")
	for name, body := range files {
		testkit.WriteFile(t, dir, name, body)
	}
	return dir
}

func auditJSON(t *testing.T, cmd, root string, args ...string) map[string]any {
	t.Helper()
	var out, errb bytes.Buffer
	full := append([]string{cmd, "--root", root, "--json"}, args...)
	if code := run(full, &out, &errb); code != exitOK {
		t.Fatalf("%s: exit %d: %s", cmd, code, errb.String())
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return res
}

// auditText bir komutun düz metin çıktısını verir.
func auditText(t *testing.T, cmd, root string, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(append([]string{cmd, "--root", root}, args...), &out, &errb); code != exitOK {
		t.Fatalf("%s: exit %d: %s", cmd, code, errb.String())
	}
	return out.String()
}
