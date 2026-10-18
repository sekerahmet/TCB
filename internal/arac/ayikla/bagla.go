package ayikla

import "tr_dili/internal/dil/yurut"

// Koddaki kesme_noktası deyimi çalıştığında yorumlayıcı henüz bağlı bir hata ayıklayıcı
// yoksa yurut.YeniHataAyiklayici ile bir tane açar. Yorumlayıcı bu paketi içe aktarmaz;
// bağlantıyı bu paket yüklenirken kurar. Tam dili hata ayıklayıcıyla çalıştıran programlar
// (cmd/tr, tr test) bu paketi içe aktarır.
func init() {
	yurut.YeniHataAyiklayici = func(sourceFile, sourceCode string) yurut.HataAyiklayici {
		return NewDebugger(sourceFile, sourceCode, true)
	}
}
