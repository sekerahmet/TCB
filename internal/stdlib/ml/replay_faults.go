package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Bu dosya TR_replay kaydındaki cihaz içi hata bayrağını (15.45) iletiye bağlar.
//
// Kayıtta hatası öğeye bağlı işlem (tam sayıya dönüşüm; gömmenin indeksi, dilim yazmanın
// başlangıcı) sonucu beklemez: arka uç hatanın yuvasını kayda ekler (gpu/fault_slot.go),
// yerleşik yuvanın yerini (satırı ve iletiyi kuran işlevi) buraya yazar. Her oynatmadan sonra yuvalar okunur; kayıt sırasındaki ilk dolu yuva
// hatadır, ileti o işlemin kayıttaki satırını söyler (messages.At: satırlı hata yorumlayıcıda
// ikinci kez sarılmaz). Hata verilen oynatmada yerinde değiştirilen argümanlar çağırana geri
// yazılmaz (15.45: içerik tanımsız).

// recordedFault kaydın bir hata yuvasının yeri: satırı ve iletiyi yuvanın hatasından kuran işlev.
type recordedFault struct {
	line int
	msg  func(*tensor.ElemFault) error
}

// recordFaultSite kayıtta site'inci yuvanın yerini yazar: satır line (atama kancaları) ya da 0
// ise yerleşiğin satırı (VM.BuiltinLine). m nil ise görev bilinmez, yuvanın yeri yazılamaz:
// kayıt açık hatayla biter (H4065; yorumlayıcı atama kancalarına görevi verir, T1 dilim 2a A12).
func recordFaultSite(m *interp.VM, line int, fn any, site int) error {
	return recordSite(m, line, site, func(f *tensor.ElemFault) error { return faultError(fn, "", f) })
}

// recordFault indeks işlemlerinin hatasını bağlar: f FaultRecorded ise yuvanın yeri yazılır
// (ileti oynatmadan sonra msg'den), değilse (kayıt dışında) ileti hemen döner.
func recordFault(m *interp.VM, f *tensor.ElemFault, msg func(*tensor.ElemFault) error) error {
	if f.Kind != tensor.FaultRecorded {
		return msg(f)
	}
	return recordSite(m, 0, f.Index, msg)
}

// recordSite site'inci yuvanın yerini kayda yazar (recordFaultSite'ın satır kuralıyla).
func recordSite(m *interp.VM, line, site int, msg func(*tensor.ElemFault) error) error {
	if m == nil {
		return messages.New(messages.H4065)
	}
	e := programCaptureOf(m).entry
	if e == nil || site != len(e.sites) {
		return messages.Internalf("fault slot %d outside its recording", site)
	}
	if line <= 0 {
		line = m.BuiltinLine()
	}
	e.sites = append(e.sites, recordedFault{line: line, msg: msg})
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
	return messages.At(s.line, 0, s.msg(f))
}
