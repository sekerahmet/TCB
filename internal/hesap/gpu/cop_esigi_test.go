package gpu

import (
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// TestCopEsigiSec eşiğin kart belleğine göre seçildiğini ve ortam değişkeniyle
// değiştirilebildiğini denetler.
func TestCopEsigiSec(t *testing.T) {
	for _, d := range []struct {
		toplam uint64
		ortam  string
		esik   uint64
	}{
		{96 << 30, "", 1536 << 20},   // 1/64
		{24 << 30, "", 384 << 20},    // 1/64
		{4 << 30, "", 256 << 20},     // en az 256 MB
		{1 << 40, "", 2 << 30},       // en çok 2 GB
		{96 << 30, "512", 512 << 20}, // ortam
		{96 << 30, "0", 0},           // kapalı
		{96 << 30, "x", 1536 << 20},  // geçersiz ortam yok sayılır
	} {
		if e := copEsigiSec(d.toplam, d.ortam); e != d.esik {
			t.Errorf("toplam %d, ortam %q: eşik %d, beklenen %d", d.toplam, d.ortam, e, d.esik)
		}
	}
}

// TestCopEsigiToplamaBaslatir eşiği aşan ayırmanın çöp toplamayı başlattığını ve
// ulaşılamaz depolamaların havuza döndüğünü denetler (benzetim: 1 MB eşik).
func TestCopEsigiToplamaBaslatir(t *testing.T) {
	g := yeniArkaUc(tensor.GPU(7), yeniBenzetim(64<<20))
	g.copEsigi = 1 << 20
	once := g.havuz.olcum().CopToplama
	for i := 0; i < 16; i++ {
		g.Alloc(128<<10, tensor.Float32) // 512 KB, hemen ulaşılamaz
	}
	o := g.havuz.olcum()
	if o.CopToplama-once < 4 {
		t.Fatalf("16 × 512 KB ayırmada %d çöp toplama, en az 4 bekleniyordu", o.CopToplama-once)
	}
	if o.Ayrilmis > 8<<20 {
		t.Fatalf("havuz şişti: %d bayt ayrılmış (ulaşılamaz depolamalar geri dönmeli)", o.Ayrilmis)
	}
}
