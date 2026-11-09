package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: derlenmeyen dosyadaki sözcük paket sınırına uyar (DENETİM 358 Ş1). Ad kendi paketinin dosyasında kısa adıyla
// eşleşir (dataload.mmapFile'ın Linux tanımı gibi); başka paketin dosyasında yalnız dışa açık ad ve yalnız dosya paketi
// içe aktarıyorsa, niteleyicisiyle (`shape.Describe`, takma adla `sh.Describe`). gaps'in örneği: Linux'ta impact.load
// (tools/impact'in dışa kapalı işlevi) sysmem'in memory_windows.go'sundaki `load` alanıyla eşleşiyordu.
func TestOtherConfigMentionsKeepPackageBounds(t *testing.T) {
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WriteFile(t, root, "shape/leaf_plan9.go", `//go:build plan9

package shape

func leafOnPlan9() { leaf() }
`)
	testkit.WriteFile(t, root, "paint/paint.go", "package paint\n\n// Brush derlenen dosyadır.\nfunc Brush() {}\n")
	testkit.WriteFile(t, root, "paint/paint_plan9.go", `//go:build plan9

package paint

import sh "example.com/mini/shape"

type state struct{ leaf int }

func Describe() {}

func drawPlan9() { _ = sh.Describe(nil); Describe() }
`)
	testkit.WriteFile(t, root, "paint/plain_plan9.go", `//go:build plan9

package paint

func plainPlan9() { Describe() }
`)
	graph := filepath.Join(root, ".codegraph", "graph.json")
	var so, se bytes.Buffer
	if code := run([]string{"build", "--root", root, "--out", graph}, &so, &se); code != exitOK {
		t.Fatalf("build: exit %d: %s", code, se.String())
	}
	// Dışa kapalı ad: kendi paketinin derlenmeyen dosyası eşleşir, başka paketin aynı sözcüğü (alan adı) eşleşmez.
	_, out, _ := commandIn("who-uses", root, graph, "--no-rebuild", "shape.leaf")
	testkit.MustContain(t, out, "in other build configurations", "shape/leaf_plan9.go:5")
	if strings.Contains(out, "paint/") {
		t.Errorf("an unexported name does not match another package's file:\n%s", out)
	}
	// Dışa açık ad: paketi içe aktaran dosyada niteleyicisiyle eşleşir; aynı adlı yerel çağrı (`Describe()`) ve paketi
	// aktarmayan dosya eşleşmez.
	_, out, _ = commandIn("who-uses", root, graph, "--no-rebuild", "shape.Describe")
	testkit.MustContain(t, out, "paint.drawPlan9 [func] at paint/paint_plan9.go:11")
	if strings.Contains(out, "plain_plan9.go") || strings.Contains(out, "paint_plan9.go:9") {
		t.Errorf("an exported name matches only qualified, in a file importing its package:\n%s", out)
	}
}
