package gpuprobe_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/stdlib"
)

// GPU'da kontrol noktası: parametreler ve iyileştirici durumları (momentler, adım
// sayaçları) cihazdadır; kayıtta konağa indirilir, yüklemede aynı cihaza konur ve
// parametreleri_yükle iyileştiricinin tuttuğu cihaz tensörlerine yazar. Kesintisiz GPU eğitimi
// ile 3. adımda kaydedilip yeni nesnelerle sürdürülen GPU eğitimi bit düzeyinde aynıdır. Gerçek
// GPU yoksa PTX benzetimidir (yavaş; şekiller küçük).
const gpuKontrolProgrami = `
varsayılan_cihaz("gpu")
değişken KOŞU = %q
fonksiyon eğit(son) {
    değişken sayaç = 0
    fonksiyon ağırlık(şekil) {
        sayaç += 1
        sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * 0.2), türev_izle: doğru)
    }
    değişken model = {E: ağırlık([16, 8]), W: ağırlık([8, 16])}
    değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
    değişken izlenen = {kayıplar: []}
    değişken kn = kontrol_noktası(KOŞU + "/" + metin(son), {model: model, op: op, izlenen: izlenen}, her_dakika: 0)
    değişken girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    değişken hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    döngü adım içinde kn.kaldığı_adım..son {
        op.sıfırla()
        değişken kayıp = çapraz_entropi(gömme(model.E, tensör(girdiler[adım %% 3])).şekillendir([8, 8]) @ model.W, tensör(hedefler[adım %% 3]))
        geri_yayılım(kayıp)
        op.adım()
        izlenen.kayıplar.ekle(kayıp.değer)
        kn.güncelle(adım + 1)
    }
    sonuç [model, op, izlenen, kn]
}
`

func gpuCalistir(t *testing.T, kod string) string {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out, m.Err = &out, &out
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hatası: %v\nçıktı:\n%s", err, out.String())
	}
	m.WaitAsync()
	stdlib.Kapat()
	return out.String()
}

func TestGPUdaKontrolNoktasi(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	kosu := filepath.ToSlash(t.TempDir())
	kod := fmt.Sprintf(gpuKontrolProgrami, kosu) + `
değişken a = eğit(6)
// "kesilen" koşu: 3 adım; sonra aynı koşu klasöründe başka tohumlu yeni nesnelerle 6. adıma
// kadar sürdürülür (yükleme yeni nesnelerin değerlerinin üzerine yazar).
değişken b = eğit(3)
fonksiyon sürdür() {
    değişken sayaç = 0
    fonksiyon ağırlık(şekil) {
        sayaç += 1
        sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç + 100) * 0.2), türev_izle: doğru)
    }
    değişken model = {E: ağırlık([16, 8]), W: ağırlık([8, 16])}
    değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
    değişken izlenen = {kayıplar: []}
    değişken kn = kontrol_noktası(KOŞU + "/3", {model: model, op: op, izlenen: izlenen}, her_dakika: 0)
    değişken girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    değişken hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    yazdır("cihaz:", model.E.cihaz, op.durum["m.0"].cihaz, kn.kaldığı_adım)
    döngü adım içinde kn.kaldığı_adım..6 {
        op.sıfırla()
        değişken kayıp = çapraz_entropi(gömme(model.E, tensör(girdiler[adım % 3])).şekillendir([8, 8]) @ model.W, tensör(hedefler[adım % 3]))
        geri_yayılım(kayıp)
        op.adım()
        izlenen.kayıplar.ekle(kayıp.değer)
        kn.güncelle(adım + 1)
    }
    sonuç [model, op, izlenen]
}
değişken c = sürdür()
yazdır("parametre:", a[0].E.değer == c[0].E.değer, a[0].W.değer == c[0].W.değer)
değişken ayni = doğru
döngü ad içinde a[1].durum.anahtarlar() {
    eğer a[1].durum[ad].değer != c[1].durum[ad].değer { ayni = yanlış }
}
yazdır("durum:", ayni, a[1].durum.anahtarlar() == c[1].durum.anahtarlar())
yazdır("kayıp:", a[2].kayıplar == c[2].kayıplar, c[2].kayıplar.uzunluk)
// parametreleri_yükle CPU'daki tensörleri iyileştiricinin tuttuğu GPU tensörüne yazar.
değişken p = {w: tensör(birler([4]), türev_izle: doğru)}
değişken sgd = SGD(parametreler(p), öğrenme_oranı: 0.5)
parametreleri_yükle(p, {w: tensör([1.0, 2.0, 3.0, 4.0]).cihaza("cpu")})
geri_yayılım(toplam(p.w))
sgd.adım()
yazdır("yükle:", p.w.cihaz, p.w.cihaza("cpu").değer)
`
	cikti := gpuCalistir(t, kod)
	for _, beklenen := range []string{"cihaz: gpu gpu 3", "parametre: doğru doğru", "durum: doğru doğru", "kayıp: doğru 6", "yükle: gpu [0.5, 1.5, 2.5, 3.5]"} {
		if !strings.Contains(cikti, beklenen) {
			t.Fatalf("%q bekleniyordu:\n%s", beklenen, cikti)
		}
	}
}
