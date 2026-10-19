// Package gpuprobe_test, GPU arka ucunu dil düzeyinde uçtan uca sınar (ayrı süreçte koşar:
// benzetimin kaydı ve varsayılan cihaz süreç geneli durumdur).
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
	_ "github.com/sekerahmet/TCB/internal/stdlib" // bütün motorlar
)

// GPU'da eğitimin dil düzeyinde uçtan uca sınaması: kıyas modelinin (tools/gpu/kiyas_gpu.tr)
// küçültülmüşü aynı veri ve tohumla önce CPU'da, sonra varsayılan_cihaz("gpu") ile GPU'da
// eğitilir; kayıplar karşılaştırılır. Gerçek GPU yoksa GPU PTX benzetimidir (yavaş; şekiller
// küçük).
const gpuEgitimProgrami = `
değişken B = 2
değişken T = 4
değişken D = 8
değişken H = 2
değişken Dh = 4
değişken FF = 8
değişken V = 16
değişken tohum_sayacı = 0
fonksiyon ağırlık(şekil, ölçek) {
    tohum_sayacı += 1
    sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: tohum_sayacı) * ölçek), türev_izle: doğru)
}
fonksiyon başlar(x) { sonuç x.şekillendir([B, T, H, Dh]).eksenleri_sırala([0, 2, 1, 3]) }
fonksiyon eğit(cihaz) {
    varsayılan_cihaz(cihaz)
    tohum_sayacı = 0
    değişken E = ağırlık([V, D], 0.02)
    değişken g1 = tensör(birler([D]), türev_izle: doğru)
    değişken Wq = ağırlık([D, D], 0.2)
    değişken Wk = ağırlık([D, D], 0.2)
    değişken Wv = ağırlık([D, D], 0.2)
    değişken Wg = ağırlık([D, FF], 0.2)
    değişken Wu = ağırlık([D, FF], 0.2)
    değişken W2 = ağırlık([FF, D], 0.2)
    değişken ağırlıklar = [E, g1, Wq, Wk, Wv, Wg, Wu, W2]
    değişken op = AdamW(ağırlıklar, öğrenme_oranı: 0.01, ağırlık_çürümesi: 0.0)
    // Toplu işler varsayılan cihazda kurulur (veri kümesi örnekleyicisi de böyle yapar).
    değişken girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    değişken hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    değişken kayıplar = []
    değişken son_x = yok
    değişken son_k = yok
    döngü adım içinde 0..3 {
        değişken xg = tensör(girdiler[adım])
        değişken yh = tensör(hedefler[adım])
        op.sıfırla()
        değişken x = gömme(E, xg)
        değişken h = rms_normu(x, g1)
        değişken y = dikkat(başlar(h @ Wq), başlar(h @ Wk), başlar(h @ Wv), nedensel: doğru)
        değişken s = x + y.eksenleri_sırala([0, 2, 1, 3]).şekillendir([B, T, D])
        değişken a = s @ Wg
        s = s + (a * sigmoid(a) * (s @ Wu)) @ W2
        değişken kayıp = çapraz_entropi(s @ E.devrik(), yh)
        geri_yayılım(kayıp)
        gradyan_kırp(ağırlıklar, en_fazla: 1.0)
        op.adım()
        kayıplar.ekle(kayıp.değer)
        son_x = xg
        son_k = kayıp
    }
    değişken son_cihaz = E.cihaz + " " + son_x.cihaz + " " + son_k.cihaz
    varsayılan_cihaz("cpu")
    sonuç [kayıplar, son_cihaz]
}
değişken c = eğit("cpu")
değişken g = eğit("gpu")
yazdır(c[1])
yazdır(g[1])
döngü i içinde 0..3 { yazdır(c[0][i], g[0][i]) }
`

func TestGPUdaEgitimDilDuzeyinde(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	kod := gpuEgitimProgrami
	p := parser.New(lexer.New(kod))
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
	satirlar := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(satirlar) != 5 || satirlar[0] != "cpu cpu cpu" || satirlar[1] != "gpu gpu gpu" {
		t.Fatalf("beklenmeyen çıktı:\n%s", out.String())
	}
	for _, s := range satirlar[2:] {
		f := strings.Fields(s)
		c, _ := strconv.ParseFloat(f[0], 64)
		g, _ := strconv.ParseFloat(f[1], 64)
		if d := c - g; d > 1e-4*c || d < -1e-4*c {
			t.Fatalf("kayıplar uyuşmuyor (CPU, GPU):\n%s", out.String())
		}
	}
}
