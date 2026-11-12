package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosya TR_replay kaydındaki cihaz içi hata bayrağını (15.45) iletiye bağlar.
//
// Kayıtta hatası öğeye bağlı işlem (bugün tam sayıya dönüşüm) sonucu beklemez: arka uç hatanın
// yuvasını kayda ekler (gpu/fault_slot.go), convertKind yuvanın yerini (yerleşiğin adı ve
// satırı) buraya yazar. Her oynatmadan sonra yuvalar okunur; kayıt sırasındaki ilk dolu yuva
// hatadır, ileti o işlemin kayıttaki satırını söyler (messages.At: satırlı hata yorumlayıcıda
// ikinci kez sarılmaz). Hata verilen oynatmada yerinde değiştirilen argümanlar çağırana geri
// yazılmaz (15.45: içerik tanımsız).

// recordedFault kaydın bir hata yuvasının iletideki yeri.
type recordedFault struct {
	fn   any
	line int
}

// recordFaultSite kayıtta site'inci yuvanın yerini yazar. m nil ise çağrı işleç kancasından
// gelir ve satırı bilinmez (T1 dilim 2a'nın A12'si): karar gelene kadar kayıt bugünkü gibi
// açık hatayla biter (H4065).
func recordFaultSite(m *interp.VM, fn any, site int) error {
	if m == nil {
		return messages.New(messages.H4065)
	}
	e := programCaptureOf(m).entry
	if e == nil || site != len(e.sites) {
		return messages.Internalf("fault slot %d outside its recording", site)
	}
	e.sites = append(e.sites, recordedFault{fn: fn, line: m.BuiltinLine()})
	return nil
}

// firstFault oynatmadan sonra kaydın yuvalarını okur; ilk hatanın iletisi kaydedilen satırla.
func (e *replayEntry) firstFault() error {
	fr, ok := e.rec.(tensor.FaultRecording)
	if !ok || len(e.sites) == 0 {
		return nil
	}
	site, f := fr.FirstFault()
	if f == nil {
		return nil
	}
	if site >= len(e.sites) {
		return messages.Internalf("fault slot %d without a place", site)
	}
	s := e.sites[site]
	return messages.At(s.line, 0, faultError(s.fn, "", f))
}
