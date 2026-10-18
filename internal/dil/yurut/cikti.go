package yurut

import (
	"io"
)

// Çıktı noktaları: insanın göreceği her çıktı (yazdır, girdi istemi, hata mesajları,
// uyarılar; ileride günlük ve ilerleme satırı) CiktiYaz'dan geçer. Canlı satır kancası:
// terminalin son satırında yeniden çizilen bir çıktı (ilerleme çubuğu) etkinse, metin
// yazılmadan önce silinir ve sonra yeniden çizilir; çıktılar çubuğa karışmaz.
//
// Kanca kurulmamışsa maliyet bir atomik okumadır.

// CanliSatir terminalin son satırında yeniden çizilen bir çıktıdır (ör. ilerleme çubuğu).
// Sil ve Ciz kendi akışına (çoğunlukla standart hata) yazar; VM onları çıktı kilidi altında
// çağırır.
type CanliSatir interface {
	Sil() // satırı ekrandan siler
	Ciz() // satırı yeniden çizer
}

// CanliSatirAyarla etkin canlı satırı ayarlar (nil: kaldırır). Aynı anda tek canlı satır
// vardır; yenisi eskisinin yerine geçer.
func (v *VM) CanliSatirAyarla(c CanliSatir) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	v.canli = c
	v.canliVar.Store(c != nil)
}

// CanliSatirGuncelle fn'i çıktı kilidi altında çalıştırır: canlı satırın kendisini çizmesi
// başka bir çıktıyla iç içe geçmesin.
func (v *VM) CanliSatirGuncelle(fn func()) {
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	fn()
}

// CiktiYaz insanın göreceği bir metni w'ye yazar: etkin canlı satır önce silinir, sonra
// yeniden çizilir.
func (v *VM) CiktiYaz(w io.Writer, metin string) {
	if !v.canliVar.Load() {
		_, _ = io.WriteString(w, metin)
		return
	}
	v.ciktiMu.Lock()
	defer v.ciktiMu.Unlock()
	c := v.canli
	if c != nil {
		c.Sil()
	}
	_, _ = io.WriteString(w, metin)
	if c != nil {
		c.Ciz()
	}
}
