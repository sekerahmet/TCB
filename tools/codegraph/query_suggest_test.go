package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: bir üyenin eski tam adı (alan başka bir tipe taşınmış) "not found"dan sonra aynı paketteki aynı kısa adlı üyeleri
// önerir; çıkış 4 kalır (gaps, "Taşınan alanın eski tam adı öneri vermiyor": `fields interp.vmShared.gilSince` düz "not
// found" veriyordu, alan interp.VM'e taşınmıştı). Başka paketin aynı adlı üyesi önerilmez.
func TestAMovedMembersOldFullNameSuggestsItsNewPlace(t *testing.T) {
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WriteFile(t, root, "paint/paint.go", "package paint\n\n// Brush has a field named like shape's.\ntype Brush struct{ n int }\n")
	graph := buildInto(t, root)
	code, out, errOut := commandIn("who-uses", root, graph, "--no-rebuild", "shape.Moved.n", "--mode", "write")
	if code != exitNotFound {
		t.Errorf("exit %d, want %d (the name is still not found)", code, exitNotFound)
	}
	testkit.MustContain(t, out+errOut, `"shape.Moved.n" not found`, "did you mean:", "shape.Other.n [field]", "shape.Counter.n [field]")
	if strings.Contains(out+errOut, "paint.Brush.n") {
		t.Errorf("a member of another package is not suggested:\n%s", out+errOut)
	}
}
