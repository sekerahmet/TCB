package gpuprobe_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/lib"
)

// Dilin kitaplığındaki GPT ve TR ile yazılmış iyileştiriciler (tr/katman, tr/iyilestirici)
// GPU'da CPU ile aynı kaybı verir: küçük bir GPT aynı veriyle önce CPU'da, sonra
// varsayılan_cihaz("gpu") ile GPU'da Muon + AdamW (ve NorMuon) ile birkaç adım eğitilir.
// Gerçek GPU yoksa PTX benzetimidir (yavaş; şekiller küçük).
const gpuLibraryProgram = `
aktar "tr/katman"
aktar "tr/iyilestirici"
fonksiyon eğit(cihaz, normuon) {
    varsayılan_cihaz(cihaz)
    değişken m = GPT(16, bağlam: 4, boyut: 8, katman: 1, bakış_sayısı: 2, kv_bakış_sayısı: 1, tohum: 3)
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

func TestGPULibraryGPT(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	p := parser.New(lexer.New(gpuLibraryProgram))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hatası: %v\nçıktı:\n%s", err, out.String())
	}
	m.WaitAsync()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 10 {
		t.Fatalf("beklenmeyen çıktı:\n%s", out.String())
	}
	for block := 0; block < 2; block++ {
		s := lines[block*5:]
		if s[0] != "cpu cpu cpu" || s[1] != "gpu gpu gpu" {
			t.Fatalf("ağırlıklar ve iyileştirici durumu seçilen cihazda olmalı:\n%s", out.String())
		}
		for _, line := range s[2:5] {
			f := strings.Fields(line)
			c, _ := strconv.ParseFloat(f[0], 64)
			g, _ := strconv.ParseFloat(f[1], 64)
			if d := c - g; d > 1e-4*c || d < -1e-4*c {
				t.Fatalf("kayıplar uyuşmuyor (CPU, GPU):\n%s", out.String())
			}
		}
	}
}
