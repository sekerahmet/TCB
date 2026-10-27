package main

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// checkArguments argüman verilmiş bir programın komut satırını okuyabildiğini denetler (1.4):
// argüman verildiği hâlde programın en dışından ulaşılabilen bir komut_satırı çağrısı yoksa
// program çalışmaz, kullanım hatasıdır (çıkış 2). Böyle bir programa yalnız -h verilirse
// programın seçeneği olmadığı yazılır, çıkış 0. Ulaşılabilirlik kesin bilinemiyorsa çağrı var
// sayılır (interp.VM.ReadsCommandLine). Her çalıştırma yolu (çalıştır, hata ayıkla, izle, profil,
// derlenmiş uygulama) çalışmadan önce bunu çağırır; başlatıcıların argümanları programın
// sayılmaz (interp.VM.SetArgs).
func checkArguments(machine *interp.VM, prog *ast.Program) {
	arguments := machine.Args()
	if len(arguments) == 0 || machine.ReadsCommandLine(prog) {
		return
	}
	if len(arguments) == 1 && arguments[0] == "-h" {
		fmt.Println(messages.Text(messages.H8665, machine.ProgramCommand()))
		exit(0)
	}
	cleanup()
	exitUsage(messages.Text(messages.H8664, len(arguments), codes.NameRef(codes.FnCommandLine)))
}
