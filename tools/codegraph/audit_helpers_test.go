package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// auditModule küçük bir Go modülü yazar: dosya adı -> kaynak.
func auditModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "go.mod", "module example.com/audit\n\ngo 1.21\n")
	for name, body := range files {
		testkit.WriteFile(t, dir, name, auditFileText(name, body))
	}
	return dir
}

// auditFileText auditModule'ün dosyaya yazdığı metindir: depodaki .tr dosyası başlıkla başlar (dil tanımı 2.1), başlıksız
// verilen gövdenin başına `# TR 4` konur; öbür dosyalar olduğu gibi.
func auditFileText(name, body string) string {
	if strings.HasSuffix(name, ".tr") && !strings.HasPrefix(body, source.Header) {
		return source.Header + "\n" + body
	}
	return body
}

// auditLine auditModule'ün yazdığı dosyada metnin ilk geçtiği satırdır (1'den, başlık dahil): beklenen satır kaynaktan
// hesaplanır.
func auditLine(t *testing.T, name, body, text string) int {
	t.Helper()
	for i, l := range strings.Split(auditFileText(name, body), "\n") {
		if strings.Contains(l, text) {
			return i + 1
		}
	}
	t.Fatalf("%s lacks %q", name, text)
	return 0
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
