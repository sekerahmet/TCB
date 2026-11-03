package main

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// checkArguments argüman verilmiş bir programın komut satırını okuyabildiğini denetler (1.4):
// argüman verildiği hâlde programın en dışından ulaşılabilen bir komut_satırı çağrısı yoksa
// program çalışmaz, kullanım hatasıdır (çıkış 2); böyle bir programa yalnız -h verilirse
// programın seçeneği olmadığı yazılır, çıkış 0. Karar ve metin interp'tedir (CheckArguments):
// hata ayıklayıcının başlatma yolu da aynısını çağırır. Her çalıştırma yolu (çalıştır, hata
// ayıkla, izle, profil, derlenmiş uygulama) çalışmadan önce bunu çağırır.
func checkArguments(machine *interp.VM, prog *ast.Program) {
	switch verdict := machine.CheckArguments(prog); verdict {
	case interp.ArgumentsNoOptions:
		fmt.Println(machine.ArgumentsMessage(verdict))
		exit(0)
	case interp.ArgumentsUnread:
		cleanup()
		exitUsage(machine.ArgumentsMessage(verdict))
	}
}
