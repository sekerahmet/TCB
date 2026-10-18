package yurut

import (
	"bytes"
	"errors"
	"testing"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
)

// sahteCanliSatir silinip çizildiğini ortak çıktıya işaretleyen bir canlı satırdır.
type sahteCanliSatir struct{ w *bytes.Buffer }

func (s *sahteCanliSatir) Sil() { s.w.WriteString("[sil]") }
func (s *sahteCanliSatir) Ciz() { s.w.WriteString("[çiz]") }

func kodCalistir(t *testing.T, vmx *VM, kod string) {
	t.Helper()
	p := ayristir.New(sozcuk.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	if _, err := vmx.Run(prog); err != nil {
		t.Fatal(err)
	}
}

// Etkin bir canlı satır (ilerleme çubuğu) varken yazdır, girdi istemi ve hata çıktısı
// önce satırı siler, metni yazar, satırı yeniden çizer; canlı satır kaldırılınca düz yazılır.
func TestCanliSatirKancasi(t *testing.T) {
	var cikti bytes.Buffer
	vmx := New()
	vmx.Out, vmx.Err = &cikti, &cikti
	vmx.Stdin = bytes.NewBufferString("ali\n")
	vmx.CanliSatirAyarla(&sahteCanliSatir{w: &cikti})
	kodCalistir(t, vmx, "yazdır(\"a\", 1)\ndeğişken ad = girdi(\"ad: \")\n")
	vmx.reportAsyncError(3, errors.New("bozuk"))
	beklenen := "[sil]a 1\n[çiz][sil]ad: [çiz][sil]⚠️  Eşzamanlı görev hatası (satır 3'te başlatılan): bozuk\n[çiz]"
	if cikti.String() != beklenen {
		t.Errorf("çıktı\n%q\nbeklenen\n%q", cikti.String(), beklenen)
	}
	cikti.Reset()
	vmx.CanliSatirAyarla(nil)
	kodCalistir(t, vmx, "yazdır(\"b\")\n")
	if cikti.String() != "b\n" {
		t.Errorf("canlı satır kaldırılınca düz yazılmalı: %q", cikti.String())
	}
	// Canlı satırın kendi çizimi de aynı kilit altında
	vmx.CanliSatirGuncelle(func() { cikti.WriteString("çubuk") })
	if cikti.String() != "b\nçubuk" {
		t.Errorf("CanliSatirGuncelle: %q", cikti.String())
	}
}
