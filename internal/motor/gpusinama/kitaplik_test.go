package gpusinama_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/gpu"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/kitaplik"
)

// Dilin kitaplığındaki GPT ve TR ile yazılmış iyileştiriciler (tr/katman, tr/iyilestirici)
// GPU'da CPU ile aynı kaybı verir: küçük bir GPT aynı veriyle önce CPU'da, sonra
// varsayılan_cihaz("gpu") ile GPU'da Muon + AdamW (ve NorMuon) ile birkaç adım eğitilir.
// Gerçek GPU yoksa PTX benzetimidir (yavaş; şekiller küçük).
const gpuKitaplikProgrami = `
aktar "tr/katman"
aktar "tr/iyilestirici"
fonksiyon eğit(cihaz, normuon) {
    varsayılan_cihaz(cihaz)
    değişken m = GPT(16, bağlam: 4, boyut: 8, katman: 1, baş: 2, kv_baş: 1, tohum: 3)
    değişken op = MuonVeAdamW(m, normuon: normuon, adamw_oranı: 0.01)
    değişken girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    değişken hedefler = [[[1, 4, 1, 5], [9, 2, 6, 5]], [[3, 5, 8, 9], [7, 9, 3, 2]], [[3, 8, 4, 6], [2, 6, 4, 3]]]
    değişken kayıplar = []
    döngü adım içinde 0..3 {
        op.çarpan_ayarla(oran_planı(adım, 3, ısınma: 1))
        op.sıfırla()
        değişken kayıp = m.kayıp(tensör(girdiler[adım]), tensör(hedefler[adım]))
        geri_yayılım(kayıp)
        op.adım()
        kayıplar.ekle(kayıp.değer)
    }
    değişken c = m.gömme.tablo.cihaz + " " + m.bloklar[0].dikkat.sorgu_katmanı.ağırlık.cihaz + " " + op.muon.durum.momentum[0].cihaz
    varsayılan_cihaz("cpu")
    sonuç [kayıplar, c]
}
döngü normuon içinde [yanlış, doğru] {
    değişken c = eğit("cpu", normuon)
    değişken g = eğit("gpu", normuon)
    yazdır(c[1])
    yazdır(g[1])
    döngü i içinde 0..3 { yazdır(c[0][i], g[0][i]) }
}
`

func TestGPUdaKitaplikGPTsi(t *testing.T) {
	gpu.BenzetimiKaydet() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	yurut.KitaplikBagla(kitaplik.Dosyalar)
	defer yurut.KitaplikBagla(nil)
	p := ayristir.New(sozcuk.New(gpuKitaplikProgrami))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := yurut.New()
	var out bytes.Buffer
	m.Out = &out
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hatası: %v\nçıktı:\n%s", err, out.String())
	}
	m.WaitAsync()
	satirlar := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(satirlar) != 10 {
		t.Fatalf("beklenmeyen çıktı:\n%s", out.String())
	}
	for blok := 0; blok < 2; blok++ {
		s := satirlar[blok*5:]
		if s[0] != "cpu cpu cpu" || s[1] != "gpu gpu gpu" {
			t.Fatalf("ağırlıklar ve iyileştirici durumu seçilen cihazda olmalı:\n%s", out.String())
		}
		for _, satir := range s[2:5] {
			f := strings.Fields(satir)
			c, _ := strconv.ParseFloat(f[0], 64)
			g, _ := strconv.ParseFloat(f[1], 64)
			if d := c - g; d > 1e-4*c || d < -1e-4*c {
				t.Fatalf("kayıplar uyuşmuyor (CPU, GPU):\n%s", out.String())
			}
		}
	}
}
