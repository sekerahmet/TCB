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
	if err := os.WriteFile(module, []byte(withHeader("TR_func f(x: TR_Int) -> TR_Int {\n    TR_var y = x\n    TR_return y\n}\n")), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "program.tr")
	if err := os.WriteFile(path, []byte(withHeader("TR_import \"m.tr\" TR_as m\nTR_var a = 1\nTR_var b = m.f(a)\nTR_print(b)\n")), 0644); err != nil {
		t.Fatal(err)
	}
	c := startDAP(t)
	c.launch(path, false, map[string]interface{}{"line": 3}, map[string]interface{}{"line": 4})
	c.event("stopped")
	if line := c.topFrameLine(); line != 3 {
		t.Fatalf("3. satırda durması bekleniyordu, %d", line)
	}
	c.request("continue", map[string]interface{}{"threadId": 1})
	c.event("stopped")
	if line := c.topFrameLine(); line != 4 {
		t.Fatalf("4. satırda durması bekleniyordu (modülün 3. satırında değil), %d", line)
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
	path := writeProgram(t, `TR_var ayar = TR_command_line({"hf_token": ""})
TR_print("belirteç:", ayar["hf_token"])
TR_print("bitti")
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
	if err := os.WriteFile(filepath.Join(dir, "göreli.tr"), []byte(withHeader("TR_print(\"göreli çalıştı\")\n")), 0644); err != nil {
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
