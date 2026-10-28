package debugger

import (
	"strings"
	"testing"
)

// Düzenleyicinin başlatma ayarındaki "args" programa iletilir. Argümanları okumayan programa
// argüman verilirse program çalışmaz: kullanım hatası hata ayıklama konsoluna yazılır (1.4).
func TestDAPArgumentsPassed(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_var ayar = TR_command_line({TR_step: 1})\nTR_print(\"adım\", ayar.TR_step)\n")
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "args": []string{"--step", "5"}})
	c.request("configurationDone", nil)
	c.event("terminated")
	if output := c.output.String(); !strings.Contains(output, "adım 5") {
		t.Fatalf("argüman programa iletilmeliydi: %q", output)
	}

	c2 := startDAP(t)
	unread := writeProgram(t, "TR_print(\"bitti\")\n")
	c2.request("initialize", map[string]interface{}{"adapterID": "tr"})
	seq := c2.send("launch", map[string]interface{}{"program": unread, "args": []string{"--adım", "5"}})
	resp := c2.next(func(m map[string]interface{}) bool {
		return m["type"] == "response" && int(m["request_seq"].(float64)) == seq
	}, "launch yanıtı")
	if output := c2.output.String(); resp["success"] != false || strings.Contains(output, "bitti") || !strings.Contains(output, "--adım") {
		t.Fatalf("argümanı okumayan program çalışmamalı, kullanım hatası yazılmalı: %v / %q", resp, output)
	}
}
