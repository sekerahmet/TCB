package testkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: WriteFile ara klasörleri açar, bölü çizgili adı işletim sisteminin ayracına çevirir ve
// yazdığı dosyanın yolunu verir.
func TestWriteFileCreatesParentFolders(t *testing.T) {
	dir := t.TempDir()
	path := WriteFile(t, dir, "a/b/c.tr", "yazdır(1)\n")
	if want := filepath.Join(dir, "a", "b", "c.tr"); path != want {
		t.Fatalf("yol %q, beklenen %q", path, want)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "yazdır(1)\n" {
		t.Fatalf("dosya içeriği %q, hata %v", data, err)
	}
}

// Kural: HasJSONName boş olmayan bir json adı ya da "-" etiketi arar; adsız etiket ("omitempty"
// tek başına) ve etiketsiz alan yoktur.
func TestHasJSONNameNeedsANameOrDash(t *testing.T) {
	src := "package p\ntype T struct {\n\tA int `json:\"a\"`\n\tB int `json:\"-\"`\n\tC int `json:\",omitempty\"`\n\tD int\n\tE int `yaml:\"e\"`\n}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []bool
	ast.Inspect(f, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok && len(field.Names) > 0 {
			got = append(got, HasJSONName(field))
		}
		return true
	})
	want := []bool{true, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alan %d: %v, beklenen %v", i, got[i], want[i])
		}
	}
}

// Kural: RunBinary çıktıları ve çıkış kodunu verir; sıfırdan farklı çıkış testi düşürmez (Wait
// denetler).
func TestRunBinaryReportsOutputsAndExitCode(t *testing.T) {
	exe, err := os.Executable() // bu test ikilisinin kendisi: -test.run ile var olmayan teste çalışır
	if err != nil {
		t.Fatal(err)
	}
	ok := RunBinary(t, exe, "", "", nil, "-test.run=^$")
	Wait(t, ok, 0, []string{"PASS"}, nil)
	bad := RunBinary(t, exe, "", "", nil, "-test.bilinmeyen-bayrak")
	if bad.Code == 0 || bad.Stderr == "" {
		t.Fatalf("geçersiz bayrak sıfırdan farklı kodla ve hata çıktısıyla dönmeli: %+v", bad)
	}
}

// Kural: ortak test yardımcıları yalnızca _test.go dosyalarından içe aktarılır (üretim koduna
// sızmaz) ve testkit reporoot dışında iç paket bilmez; trrun yalnızca dil çekirdeğini bilir.
func TestHelpersAreImportedOnlyByTests(t *testing.T) {
	root := reporoot.Root()
	const mod = "github.com/sekerahmet/TCB/"
	for _, top := range []string{"cmd", "internal", "lib", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				switch {
				case strings.HasPrefix(p, mod+"internal/testkit") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "internal/testkit/"):
					t.Errorf("%s: %s yalnızca testlerden içe aktarılır", rel, p)
				case strings.HasPrefix(rel, "internal/testkit/") && !strings.HasPrefix(rel, "internal/testkit/trrun/") &&
					strings.HasPrefix(p, mod) && p != mod+"internal/reporoot":
					t.Errorf("%s: testkit reporoot dışında iç paket bilmez (%s)", rel, p)
				case strings.HasPrefix(rel, "internal/testkit/trrun/") && strings.HasPrefix(p, mod) && !strings.HasPrefix(p, mod+"internal/lang/"):
					t.Errorf("%s: trrun yalnızca dil çekirdeğini bilir (%s)", rel, p)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
