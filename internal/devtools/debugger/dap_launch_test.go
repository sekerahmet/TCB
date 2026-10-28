package debugger

import (
	"strings"
	"testing"
)

// Düzenleyicinin başlatma ayarındaki "args" programa iletilir; program okumazsa bitişte hata ayıklama
// konsoluna (stderr) uyarı yazılır.
func TestDAPArgumentsPassed(t *testing.T) {
	c := startDAP(t)
	path := writeProgram(t, "TR_print(\"bitti\")\n")
	c.request("initialize", map[string]interface{}{"adapterID": "tr"})
	c.event("initialized")
	c.request("launch", map[string]interface{}{"program": path, "args": []string{"--adım", "5"}})
	c.request("configurationDone", nil)
	c.event("terminated")
	output := c.output.String()
	if !strings.Contains(output, "bitti") || !strings.Contains(output, "programa verilen 2 argüman kullanılmadı (--adım 5)") {
		t.Fatalf("argüman uyarısı bekleniyordu: %q", output)
	}
}
