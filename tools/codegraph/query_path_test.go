package main

import (
	"strings"
	"testing"
)

var pathFiles = map[string]string{
	"p/p.go": `package p

type T struct{}

func (T) Soft() {}

// A yalnızca T tipini kullanır, Soft'u çağırmaz.
func A(t T) {}

// B Soft'u çağırır.
func B(t T) { t.Soft() }
`,
}

// Kural: `path` varsayılan olarak yalnızca yönlü yol arar ve yoksa "no directed path" der (çıkış 4); "A ve B aynı türü
// kullanıyor" bir çağrı yolu değildir. `--undirected` yönü yok sayar ama tür, paket ve dosya düğümlerinden geçmez;
// `--through-hubs` onlardan da geçer ve işaretler.
func TestPathAnswersDirectedAndNeverSilentlyThroughATypeHub(t *testing.T) {
	root := auditModule(t, pathFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	ask := func(args ...string) (int, string, string) {
		out.Reset()
		errb.Reset()
		code := run(append(append([]string{"path"}, args...), "--root", root, "--no-rebuild"), &out, &errb)
		return code, out.String(), errb.String()
	}
	if code, got, msg := ask("p.B", "p.T.Soft"); code != exitOK || !strings.Contains(got, "(directed, 1 steps)") {
		t.Errorf("B calls Soft: exit %d\n%s%s", code, got, msg)
	}
	code, got, msg := ask("p.A", "p.T.Soft")
	if code != exitNotFound || got != "" || !strings.Contains(msg, "no directed path from p.A to p.T.Soft") {
		t.Errorf("A does not call Soft, and the type they share must not make it a path: exit %d\n%s%s", code, got, msg)
	}
	code, got, msg = ask("p.A", "p.T.Soft", "--undirected")
	if code != exitNotFound || !strings.Contains(msg, "avoids types, packages and files") {
		t.Errorf("--undirected must not go through the type either: exit %d\n%s%s", code, got, msg)
	}
	code, got, _ = ask("p.A", "p.T.Soft", "--through-hubs")
	if code != exitOK || !strings.Contains(got, "p.T [type]") || !strings.Contains(got, "[hub:") {
		t.Errorf("--through-hubs shows the hub and marks it: exit %d\n%s", code, got)
	}
}
