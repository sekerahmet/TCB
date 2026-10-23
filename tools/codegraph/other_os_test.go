package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: bu yapılandırmada derlenmeyen bir Go dosyası (başka işletim sistemi) grafikte sessizce yok olmaz: dosya ve
// yalnızca orada tanımlı işlev düğüm olur ("not built for ..."), oradaki ileti kodu kullanımı INFERRED kenarla gelir,
// `build` uyarısı hangi yapılandırmanın tiplerle okunduğunu söyler, boş kalan bir Go cevabı ("no uses") o dosyaları
// not eder (bağımsız denetim Y4: who-uses H2154 file_error_unix.go'yu görmüyordu).
func TestFilesOfOtherBuildConfigsAreReadWithoutTypes(t *testing.T) {
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WriteFile(t, root, "shape/elsewhere_plan9.go", `//go:build plan9

package shape

import "example.com/mini/internal/lang/messages"

// OnlyElsewhere yalnızca plan9'da derlenir.
func OnlyElsewhere() error { return messages.New(messages.H0002) }

var tool = "go"
`)
	testkit.WriteFile(t, root, "shape/elsewhere_plan9_test.go", `//go:build plan9

package shape

import "testing"

func TestElsewhere(t *testing.T) { _ = OnlyElsewhere(); _ = Describe(nil) }
`)
	graph := filepath.Join(root, ".codegraph", "graph.json")
	var so, se bytes.Buffer
	if code := run([]string{"build", "--root", root, "--out", graph, "--warnings"}, &so, &se); code != exitOK {
		t.Fatalf("build: exit %d: %s", code, se.String())
	}
	testkit.MustContain(t, so.String()+se.String(), "read without types", "shape/elsewhere_plan9.go")
	_, out, _ := commandIn("who-uses", root, graph, "--no-rebuild", "H0002")
	testkit.MustContain(t, out, "shape.OnlyElsewhere", "shape/elsewhere_plan9.go:8", "[INFERRED]")
	_, out, _ = commandIn("query", root, graph, "--no-rebuild", "shape.OnlyElsewhere")
	testkit.MustContain(t, out, "not built for ")
	// Derlenmeyen testteki düz çağrı zincire girer: başka işletim sisteminin testi bu yapılandırmadaki işlevi sınar.
	_, out, _ = commandIn("tests-for", root, graph, "--no-rebuild", "shape.Describe")
	testkit.MustContain(t, out, "shape.TestElsewhere")
	// Ad derlenmeyen dosyada geçiyorsa cevap o yerleri bildirime göre söyler (testler işaretli); geçmiyorsa söz yok
	// (DENETİM doğrulaması Y4: not ilgisiz boş cevaplara da düşüyordu).
	_, out, _ = commandIn("who-uses", root, graph, "--no-rebuild", "shape.OnlyElsewhere")
	testkit.MustContain(t, out, "in other build configurations", "shape.TestElsewhere [func] at shape/elsewhere_plan9_test.go:7  [test]")
	// Dosya adı bir bildirim adı değildir: "shape.go"nun son parçası "go", derlenmeyen dosyadaki "go" sözcüğüyle eşleşmez.
	if _, out, _ = commandIn("query", root, graph, "--no-rebuild", "shape/shape.go"); strings.Contains(out, "other build configurations") {
		t.Errorf("a file name is not a declaration name:%s", out)
	}
	if _, out, _ = commandIn("who-uses", root, graph, "--no-rebuild", "shape.Square.Area"); strings.Contains(out, "other build configurations") {
		t.Errorf("a name the other files never mention gets no note:\n%s", out)
	}
}
