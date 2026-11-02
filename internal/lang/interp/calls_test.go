package interp

import (
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

func TestUnknownNameErrorHasColumnAndSuggestion(t *testing.T) {
	for _, tc := range []struct {
		code      string
		line, col int    // 0: konum denetlenmez
		want      string // öneri ipucunun metni (biçimi)
	}{
		{"TR_var adet = 1\nTR_print(adt + 1)", 2, len("TR_print(") + 1, "Bunu mu demek istediniz: 'adet'?"},
		{"TR_var değişken_adı = 1\ndegisken_adi = 2", 0, 0, "Bunu mu demek istediniz: 'değişken_adı'?"},
		{"TR_var sayaç = 1\nsayac += 1", 2, 1, "Bunu mu demek istediniz: 'sayaç'?"},
		{"yazdırr(1)", 0, 0, "Bunu mu demek istediniz: 'TR_print'?"},
	} {
		_, _, err := runTRWithin(t, 5*time.Second, tc.code)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q içeren hata bekleniyordu, alınan: %v", tc.want, err)
			continue
		}
		if line, col, ok := messages.Pos(err); tc.line != 0 && (!ok || line != tc.line || col != tc.col) {
			t.Errorf("konum %d:%d bekleniyordu, alınan %d:%d (%v)", tc.line, tc.col, line, col, ok)
		}
	}
}

// deferringDebugger, gövdesinde 'ertele' olmayan fonksiyonun bir satırında (her satırda bir kez)
// ortama ExecStatements ile ertelenen iş ekler: kabuktaki ve hata ayıklayıcıdaki değerlendirme gibi.
type deferringDebugger struct {
	t     *testing.T
	lines map[int]string
}

func (d *deferringDebugger) BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int {
	name, ok := d.lines[stmt.Line()]
	if !ok {
		return 0
	}
	delete(d.lines, stmt.Line())
	prog := parser.New(bodyLexer(`TR_defer { TR_print("` + name + ` ertelenen") }`)).ParseProgram()
	if _, err := v.ExecStatements(prog.Statements, env); err != nil {
		d.t.Error(err)
	}
	return 0
}

func (d *deferringDebugger) Breakpoint(string) {}

// 'ertele'siz fonksiyona sonradan eklenen ertelenen iş, çağrı dönerken de hatayla çıkarken de çalışır.
func TestDeferredWorkAddedToAFunctionWithoutDeferRuns(t *testing.T) {
	code := "TR_func f() {\n" + // 1
		"    TR_var s = 1\n" + // 2
		"    TR_return s + 1\n" + // 3
		"}\n" + // 4
		"TR_func g() {\n" + // 5
		"    TR_var s = 1\n" + // 6
		"    TR_assert(TR_false)\n" + // 7
		"}\n" + // 8
		"TR_print(f())\n" + // 9
		"TR_try { g() } TR_catch h { TR_print(h.TR_code) }\n" // 10
	prog := parser.New(bodyLexer(code)).ParseProgram()
	var out strings.Builder
	machine := New()
	machine.Out = &out
	machine.Debugger = &deferringDebugger{t: t, lines: map[int]string{2: "f", 6: "g"}}
	if _, err := machine.Run(prog); err != nil {
		t.Fatal(err)
	}
	if want := "f ertelenen\n2\ng ertelenen\nH2457\n"; out.String() != want {
		t.Fatalf("beklenen %q, alınan %q", want, out.String())
	}
}
