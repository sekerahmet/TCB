package ml

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tensör başına tür: oluşturucular, .tür, .türe ve yazdırma.
func TestDTypeChoice(t *testing.T) {
	out := mustRun(t, `değişken a = tensör([[1.001, 2], [3, 4]], tür: "bf16")
yazdır(a.tür, a.değer)
yazdır(a)
yazdır(tensör(2.5, tür: "fp16"), tensör(2.5))
yazdır(sıfırlar([2], tür: "fp16").tür, birler([2], tür: "bf16").tür, rastgele_normal([2], tohum: 1, tür: "bf16").tür)
yazdır(tensör(a).tür, a.türe("float32").tür, (a @ a).tür, (a + tensör([1.0, 1.0])).tür)
değişken w = tensör([1.0, 3.0], türev_izle: doğru)
değişken h = w.türe("bf16")
yazdır(h.tür, h)
geri_yayılım(toplam(h * h))
yazdır(w.gradyan.değer, w.gradyan.tür)`)
	want := "bf16 [[1.0, 2.0], [3.0, 4.0]]\n" +
		"Tensör[2×2] bf16 [\n  [1.0000, 2.0000],\n  [3.0000, 4.0000]\n]\n" +
		"Tensör(2.5000) fp16 Tensör(2.5000)\n" +
		"fp16 bf16 bf16\n" +
		"bf16 float32 bf16 float32\n" +
		"bf16 Tensör[2] bf16 (türev izleniyor) [1.0000, 3.0000]\n" +
		"[2.0, 6.0] float32\n"
	if out != want {
		t.Fatalf("veri türleri:\n%q\nbeklenen:\n%q", out, want)
	}
	// İzleme Masası etiketi de türü gösterir (yalnızca float32 dışında).
	if e := New(tensor.NewOf(tensor.BFloat16, []float32{1, 2, 3, 4, 5, 6}, 2, 3)).WatchLabel(); e != "Tensör [2×3] bf16" {
		t.Errorf("izleme etiketi: %q", e)
	}
	if e := New(tensor.New([]float32{1, 2}, 2)).WatchLabel(); e != "Tensör [2]" {
		t.Errorf("izleme etiketi (float32): %q", e)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`tensör([1], tür: "bfloat16")`, messages.H6811},
		{`tensör([1], tür: "bf61")`, messages.H6811},
		{`birler([1], tür: "half")`, messages.H6811},
		{`sıfırlar([1], tür: "float64")`, messages.H6811},
		{`tensör([1.0]).türe("fp32")`, messages.H6811},
		{`tensör([1.0]).türe()`, messages.H6806},
		{`tensör([1], tür: yok)`, messages.H6803},
		{`tensör([1], dtype: "bf16")`, messages.H7901},
		{`tensör([1.0]).tür_`, messages.H7903},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// karışık_hassasiyet: blok değeri, iç içe bloklar, hatada geri dönüş ve geri yayılım.
func TestMixedPrecisionBlock(t *testing.T) {
	out := mustRun(t, `değişken x = tensör([[1.001]])
değişken y = tensör([[1.0]])
değişken r = karışık_hassasiyet("bf16") {
    yazdır((x @ y).değer, (x @ y).tür)
    karışık_hassasiyet("fp16") { yazdır((x @ y).değer) }
    karışık_hassasiyet("float32") { yazdır((x @ y).değer) }
    yazdır(([1.001] @ y).değer)
    (x @ y) * 3
}
yazdır(r.değer, (x @ y).değer)
dene { karışık_hassasiyet("bf16") { fırlat "dur" } } yakala h { yazdır(h) }
yazdır((x @ y).değer)
yazdır(karışık_hassasiyet("bf16") { 5 }, karışık_hassasiyet("bf16") {})`)
	want := "[[1.0]] bf16\n[[1.0009766]]\n[[1.001]]\n[1.0]\n[[3.0]] [[1.001]]\ndur\n[[1.001]]\n5 yok\n" // yakalanan fırlat öneksiz (8.2)
	if out != want {
		t.Fatalf("karışık_hassasiyet:\n%q\nbeklenen:\n%q", out, want)
	}
	expectCodes(t, []codeCase{
		{`karışık_hassasiyet("bf16")`, messages.H6807},
		{`karışık_hassasiyet("bf17") { 1 }`, messages.H6811},
		{`karışık_hassasiyet("bf16", 3)`, messages.H6809},
		{`karışık_hassasiyet() { 1 }`, messages.H6808},
	})
}

// Blok içindeki gradyanlar float32 gradyanlarına yakındır; ağırlıklar float32 kalır.
func TestMixedPrecisionGradient(t *testing.T) {
	out := mustRun(t, `fonksiyon hesap(karışık) {
    değişken x = rastgele_normal([16, 32], tohum: 1)
    değişken w = tensör(rastgele_normal([32, 8], tohum: 2) * 0.2, türev_izle: doğru)
    değişken g = tensör(birler([8]), türev_izle: doğru)
    fonksiyon ileri() {
        değişken h = katman_normu(gelu(x @ w), g)
        sonuç ortalama(log_softmax(h, eksen: -1) * -1)
    }
    değişken k = yok
    eğer karışık { k = karışık_hassasiyet("bf16") { ileri() } } değilse { k = ileri() }
    geri_yayılım(k)
    sonuç [k.değer, w.gradyan.değer, g.gradyan.değer, w.tür, k.tür]
}
değişken a = hesap(yanlış)
değişken b = hesap(doğru)
yazdır(a[3], b[3], b[4])
değişken en = 0.0
değişken ölçek = 0.0
döngü i içinde 0..32 {
    döngü j içinde 0..8 {
        en = maks(en, mutlak(a[1][i][j] - b[1][i][j]))
        ölçek = maks(ölçek, mutlak(a[1][i][j]))
    }
}
yazdır(mutlak(a[0] - b[0]) < 0.01, en < 0.05 * ölçek)`)
	if out != "float32 float32 float32\ndoğru doğru\n" {
		t.Fatalf("karışık hassasiyet gradyanı:\n%s", out)
	}
}

// Karışık hassasiyet durumu göreve bağlıdır: blok GIL'i bırakan bir çağrıdayken (bekle)
// çalışan başka görev bloktan etkilenmez; görev beklemeden sonra kendi türünü görür.
func TestMixedPrecisionTaskIsolation(t *testing.T) {
	out := mustRun(t, `değişken x = tensör([[1.001]])
değişken y = tensör([[1.0]])
değişken görev = []
değişken başla = []
eşzamanlı {
    karışık_hassasiyet("bf16") {
        görev.ekle((x @ y).değer[0][0])
        bekle(0.3)
        görev.ekle((x @ y).değer[0][0])
    }
}
eşzamanlı {
    // Bu görev bloğun dışında: float32.
    döngü görev.uzunluk == 0 { bekle(0.01) }
    başla.ekle((x @ y).değer[0][0])
}
döngü görev.uzunluk == 0 { bekle(0.01) }
değişken ana = [(x @ y).değer[0][0], karışık_hassasiyet("fp16") { bekle(0.05); (x @ y).değer[0][0] }]
bekle_hepsi()
yazdır(görev, başla, ana, (x @ y).değer[0][0])`)
	if out != "[1.0, 1.0] [1.001] [1.001, 1.0009766] 1.001\n" {
		t.Fatalf("görev izolasyonu:\n%s", out)
	}
}

// Kayıt sürüm 2: tür kodu başlıkta, 16 bitlik veri 2 bayt; başka sürüm reddedilir.
func TestTensorSaveVersion2(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.ToSlash(filepath.Join(dir, "turler.trt"))
	out := mustRun(t, `değişken p = {"a": tensör([[1.5, -2.25], [3.0, 0.1]], tür: "bf16"), "b": tensör([0.1, 65504.0, 0.00001], tür: "fp16"), "c": tensör([0.1])}
tensör_kaydet("`+fpath+`", p)
değişken y = tensör_yükle("`+fpath+`", türev_izle: doğru)
yazdır(y["a"].tür, y["b"].tür, y["c"].tür)
yazdır(y["a"] == p["a"], y["b"] == p["b"], y["c"] == p["c"], y["a"].değer)
yazdır(y["b"].gradyan, y["b"])`)
	want := "bf16 fp16 float32\n" +
		"doğru doğru doğru [[1.5, -2.25], [3.0, 0.100097656]]\n" +
		"yok Tensör[3] fp16 (türev izleniyor) [9.9976e-02, 6.5504e+04, 1.0014e-05]\n"
	if out != want {
		t.Fatalf("sürüm 2 gidiş-dönüş:\n%q\nbeklenen:\n%q", out, want)
	}
	data, err := os.ReadFile(filepath.FromSlash(fpath))
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(data[8:10]) != 2 {
		t.Fatalf("sürüm 2 bekleniyordu: % x", data[:10])
	}
	// Başlık 14 + her girdi (4 + ad + 2 + 8·boyut + veri) + sağlama 4.
	wantLen := 14 + (4 + 1 + 2 + 16 + 4*2) + (4 + 1 + 2 + 8 + 3*2) + (4 + 1 + 2 + 8 + 4) + 4
	if len(data) != wantLen {
		t.Fatalf("dosya boyu %d, beklenen %d (16 bitlik veri 2 bayt yazılmalı)", len(data), wantLen)
	}
	// İlk girdinin tür kodu: başlık (14) + ad uzunluğu (4) + "a".
	if data[14+4+1] != 2 {
		t.Fatalf("bf16 tür kodu 2 olmalı, %d", data[14+4+1])
	}

	// Yalnızca sürüm 2 okunur; başka sürüm açık bir hatayla reddedilir. Öğüt yapılabilir olandır
	// (B18): eski dosya okunamadığı için tensör_kaydet ile yeniden yazılamaz, veri yeniden üretilir.
	v1 := version1File(t, "w", []int{2}, []float32{0.5, -1.25}, 1)
	v1Path := filepath.ToSlash(filepath.Join(dir, "eski.trt"))
	if err := os.WriteFile(filepath.FromSlash(v1Path), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, `tensör_yükle("`+v1Path+`")`); err == nil || !messages.Has(err, messages.H6611) || strings.Contains(err.Error(), "tensör_kaydet") || !strings.Contains(err.Error(), "yeniden üretin") {
		t.Errorf("sürüm 1 dosyası reddedilmeli, öğüt veriyi yeniden üretmek: %v", err)
	}
	// Eksik 16 bitlik veri bayt sayısıyla bildirilir.
	if _, err := run(t, `tensör_yükle("`+writeClipped(t, dir, data, 14+4+1+2+16+4)+`")`); err == nil || !messages.Has(err, messages.H6623) {
		t.Errorf("eksik bf16 verisi: %v", err)
	}
}

// version1File tek girdili, artık okunmayan eski sürüm 1 biçiminde bir dosya üretir.
func version1File(t *testing.T, ad string, shape []int, data []float32, dtypeCodeVal byte) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("TRTENSOR")
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(1))
	binary.Write(&b, binary.LittleEndian, uint32(len(ad)))
	b.WriteString(ad)
	b.WriteByte(dtypeCodeVal)
	b.WriteByte(byte(len(shape)))
	for _, d := range shape {
		binary.Write(&b, binary.LittleEndian, uint64(d))
	}
	for _, x := range data {
		binary.Write(&b, binary.LittleEndian, math.Float32bits(x))
	}
	binary.Write(&b, binary.LittleEndian, crc32.ChecksumIEEE(b.Bytes()))
	return b.Bytes()
}

// writeClipped verinin ilk n baytını geçici bir dosyaya yazar ve yolunu verir.
func writeClipped(t *testing.T, dir string, data []byte, n int) string {
	t.Helper()
	p := filepath.Join(dir, "kirpik"+strconv.Itoa(n)+".trt")
	if err := os.WriteFile(p, data[:n], 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(p)
}

// İyileştiricilerin ana_kopya seçeneği SetMasterWeights'e bağlanır.
func TestOptimizerMasterCopy(t *testing.T) {
	out := mustRun(t, `fonksiyon eğit(ad, ana) {
    değişken p = tensör([1.0], tür: "bf16", türev_izle: doğru)
    değişken o = yok
    eğer ad == "SGD" { o = SGD([p], öğrenme_oranı: 0.001, ana_kopya: ana) } değilse { o = AdamW([p], öğrenme_oranı: 0.001, ağırlık_çürümesi: 0.0, ana_kopya: ana) }
    döngü i içinde 0..20 {
        o.sıfırla()
        geri_yayılım(toplam(p))
        o.adım()
    }
    sonuç p.değer[0]
}
yazdır(eğit("SGD", yanlış), eğit("AdamW", yanlış), eğit("SGD", doğru) < 0.99, eğit("AdamW", doğru) < 0.99)`)
	if out != "1.0 1.0 doğru doğru\n" {
		t.Fatalf("ana_kopya: %q", out)
	}
	if _, err := run(t, `AdamW([tensör([1.0], türev_izle: doğru)], ana_kopya: "evet")`); err == nil || !messages.Has(err, messages.H7905) {
		t.Errorf("ana_kopya türü: %v", err)
	}
}
