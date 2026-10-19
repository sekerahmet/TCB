package debugger

import "github.com/sekerahmet/TCB/internal/lang/interp"

// Koddaki kesme_noktası deyimi çalıştığında yorumlayıcı henüz bağlı bir hata ayıklayıcı
// yoksa interp.NewDebugger ile bir tane açar. Yorumlayıcı bu paketi içe aktarmaz;
// bağlantıyı bu paket yüklenirken kurar. Tam dili hata ayıklayıcıyla çalıştıran programlar
// (cmd/tr, tr test) bu paketi içe aktarır.
func init() {
	interp.NewDebugger = func(sourceFile, sourceCode string) interp.Debugger {
		return NewDebugger(sourceFile, sourceCode, true)
	}
}
