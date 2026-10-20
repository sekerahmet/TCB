package debugger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func frameSourcePath(f interface{}) string {
	src, _ := f.(map[string]interface{})["source"].(map[string]interface{})
	p, _ := src["path"].(string)
	return p
}

// Ana dosyanın kesme noktası 'aktar' ile yüklenen modülün aynı numaralı satırında durmaz;
// modülün içinde durulunca çağrı yığını her çerçeveyi kendi dosyasıyla bildirir.
func TestDAPModuleSourceLines(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "m.tr")
	if err := os.WriteFile(module, []byte("fonksiyon f(x) {\n    değişken y = x\n    sonuç y\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "program.tr")
	if err := os.WriteFile(path, []byte("aktar \"m.tr\" olarak m\ndeğişken a = 1\ndeğişken b = m.f(a)\nyazdır(b)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c := startDAP(t)
	c.launch(path, false, map[string]interface{}{"line": 2}, map[string]interface{}{"line": 3})
	c.event("stopped")
	if line := c.topFrameLine(); line != 2 {
		t.Fatalf("2. satırda durması bekleniyordu, %d", line)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	if line := c.topFrameLine(); line != 3 {
		t.Fatalf("3. satırda durması bekleniyordu (modülün 2. satırında değil), %d", line)
	}
	c.request("stepIn", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	frames := c.request("stackTrace", map[string]interface{}{"threadId": 1})["stackFrames"].([]interface{})
	if len(frames) != 2 || !samePath(frameSourcePath(frames[0]), module) || !samePath(frameSourcePath(frames[1]), path) {
		t.Fatalf("modülün çerçevesi modülün, çağıranınki programın dosyasıyla bildirilmeli: %v", frames)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("terminated")
}

// Gizli değer bir protokol adıyla aynı olsa da (burada "output": çıktı olayının alanı ve
// adı) ileti bozulmaz: zarf alanları, anahtarlar ve protokolün sabit değerleri maskelenmez.
func TestDAPSecretDoesNotBreakProtocol(t *testing.T) {
	path := writeProgram(t, `değişken ayar = komut_satırı({hf_token: ""})
yazdır("belirteç:", ayar.hf_token)
yazdır("bitti")
`)
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "args": []string{"--hf_token", "output"}})
	c.request("setBreakpoints", map[string]interface{}{"source": map[string]interface{}{"path": path}, "breakpoints": []interface{}{}})
	c.request("configurationDone", nil)
	c.event("terminated")
	if out := c.output.String(); !strings.Contains(out, "bitti") || strings.Contains(out, "belirteç: output") {
		t.Fatalf("çıktı olayları düzenleyiciye ulaşmalı, gizli değer maskelenmeli: %q", out)
	}
}

// Göreli program yolu düzenleyicinin bildirdiği çalışma klasöründe ('cwd') çözülür.
func TestDAPRelativeProgramInCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "göreli.tr"), []byte("yazdır(\"göreli çalıştı\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	c := startDAP(t)
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": "göreli.tr", "cwd": dir})
	c.request("configurationDone", nil)
	c.event("terminated")
	if !strings.Contains(c.output.String(), "göreli çalıştı") {
		t.Fatalf("program çalışmalıydı: %q", c.output.String())
	}
}
