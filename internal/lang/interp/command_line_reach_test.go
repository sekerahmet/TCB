package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// reads programın en dışından ulaşılabilen bir komut_satırı çağrısı olup olmadığını verir; main
// ana programdır, files yanındaki modüllerdir (yol → içerik).
func reads(t *testing.T, main string, files map[string]string) bool {
	t.Helper()
	dir := t.TempDir()
	mainPath := testkit.WriteFile(t, dir, "ana.tr", main)
	for name, content := range files {
		testkit.WritePath(t, filepath.Join(dir, filepath.FromSlash(name)), content)
	}
	v := New()
	// stdlib'in kaydettiği yerleşik yerine komut_satırı adıyla bağlı bir yerleşik
	v.RegisterBuiltinCode(codes.FnCommandLine, func(*VM, []Value, map[string]Value) (Value, error) { return Null, nil })
	v.SourceFile = mainPath
	return v.ReadsCommandLine(mustParseProgram(t, main))
}

// Argüman verilen bir programda en dıştan ulaşılan komut_satırı çağrısı aranır (1.4): doğrudan,
// çağrılan fonksiyonlarda ve aktarılan modüllerde; çağrılmayan fonksiyondaki çağrı ve test
// blokları sayılmaz; kesin bilinemeyen yerde (fonksiyon değeriyle çağrı, bulunamayan modül) çağrı
// var sayılır.
func TestReadsCommandLineReachability(t *testing.T) {
	const option = `TR_command_line({"adim": 1})`
	for _, tc := range []struct {
		name  string
		main  string
		files map[string]string
		want  bool
	}{
		{"çağrı yok", "# TR 4\nTR_print(\"a\")\n", nil, false},
		{"en dışta çağrı", "# TR 4\nTR_var ayar = " + option + "\n", nil, true},
		{"çağrılan fonksiyonda", "# TR 4\nTR_func oku() {\n    TR_var a = " + option + "\n}\noku()\n", nil, true},
		{"zincirle çağrılan fonksiyonda", "# TR 4\nTR_func iç() {\n    TR_var a = " + option + "\n}\nTR_func dış() {\n    iç()\n}\ndış()\n", nil, true},
		{"çağrılmayan fonksiyonda", "# TR 4\nTR_func oku() {\n    TR_var a = " + option + "\n}\nTR_print(\"a\")\n", nil, false},
		{"test bloğunda", "# TR 4\nTR_test \"t\" {\n    TR_var a = " + option + "\n}\n", nil, false},
		{"fonksiyon değeriyle çağrı", "# TR 4\nTR_var f = () => {\n    TR_print(\"a\")\n}\nf()\n", nil, true},
		{"modülün fonksiyonu çağrılır", "# TR 4\nTR_import \"ayar\" TR_as ayar\nayar.oku()\n",
			map[string]string{"ayar.tr": "# TR 4\nTR_func oku() {\n    TR_var a = " + option + "\n}\n"}, true},
		{"modül aktarılır ama çağrılmaz", "# TR 4\nTR_import \"ayar\" TR_as ayar\nTR_print(\"a\")\n",
			map[string]string{"ayar.tr": "# TR 4\nTR_func oku() {\n    TR_var a = " + option + "\n}\n"}, false},
		{"modülün en dışında", "# TR 4\nTR_import \"ayar\"\n",
			map[string]string{"ayar.tr": "# TR 4\nTR_var a = " + option + "\n"}, true},
		{"bulunamayan modül", "# TR 4\nTR_import \"yok_modul\"\n", nil, true},
		{"dilin kitaplığı komut satırını okumaz", "# TR 4\nTR_import \"tr/katman\"\nTR_print(\"a\")\n", nil, false},
	} {
		if got := reads(t, tc.main, tc.files); got != tc.want {
			t.Errorf("%s: %v, %v bekleniyordu", tc.name, got, tc.want)
		}
	}
}

// Dilin kitaplık modülleri komut satırını okumaz: ulaşılabilirlik incelemesi (ReadsCommandLine)
// gömülü kitaplığı taramaz ve bu varsayım burada korunur (1.4). Kitaplığa komut_satırı çağrısı
// girerse ya inceleme kitaplığı da taramalı ya da çağrı kitaplıktan çıkmalıdır.
func TestLibraryModulesDoNotReadTheCommandLine(t *testing.T) {
	v := New()
	v.RegisterBuiltinCode(codes.FnCommandLine, func(*VM, []Value, map[string]Value) (Value, error) { return Null, nil })
	scan := &reachScan{vm: v}
	root := filepath.Join(reporoot.Root(), "lib")
	files := 0
	err := reporoot.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".tr") || strings.HasSuffix(path, "_test.tr") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files++
		prepared := source.Prepare(string(data), source.Options{Module: true})
		ast.Walk(prepared.Program, func(n any) bool {
			if id, ok := n.(*ast.Identifier); ok && scan.isCommandLine(id.Value) {
				t.Errorf("%s: kitaplık modülü komut satırını okuyor (satır %d)", path, id.Line())
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("kitaplık dosyası bulunamadı")
	}
}
