package ogrenme

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

	"tr_dili/internal/hesap/tensor"
)

// Tensör başına tür: oluşturucular, .tür, .türe ve yazdırma.
func TestVeriTuruSecimi(t *testing.T) {
	out := mustCalistir(t, `değişken a = tensör([[1.001, 2], [3, 4]], tür: "bf16")
yazdır(a.tür, a.değer)
yazdır(a)
yazdır(tensör(2.5, tür: "fp16"), tensör(2.5))
yazdır(sıfırlar([2], tür: "fp16").tür, birler([2], tür: "bf16").tür, rastgele_normal([2], tohum: 1, tür: "bf16").tür)
yazdır(tensör(a).tür, tensör(a, tür: "float32").tür, (a @ a).tür, (a + tensör([1.0, 1.0])).tür)
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
	if e := Yeni(tensor.NewOf(tensor.BFloat16, []float32{1, 2, 3, 4, 5, 6}, 2, 3)).IzlemeEtiketi(); e != "Tensör [2×3] bf16" {
		t.Errorf("izleme etiketi: %q", e)
	}
	if e := Yeni(tensor.New([]float32{1, 2}, 2)).IzlemeEtiketi(); e != "Tensör [2]" {
		t.Errorf("izleme etiketi (float32): %q", e)
	}
	for _, d := range []struct{ kod, beklenen string }{
		{`tensör([1], tür: "bfloat16")`, `tensör: bilinmeyen veri türü "bfloat16"; geçerli türler: "float32", "bf16", "fp16". Bunu mu demek istediniz: "bf16"?`},
		{`tensör([1], tür: "bf61")`, `Bunu mu demek istediniz: "bf16"?`},
		{`birler([1], tür: "half")`, `Bunu mu demek istediniz: "fp16"?`},
		{`sıfırlar([1], tür: "float64")`, `sıfırlar: bilinmeyen veri türü "float64"`},
		{`tensör([1.0]).türe("fp32")`, `türe: bilinmeyen veri türü "fp32"; geçerli türler: "float32", "bf16", "fp16". Bunu mu demek istediniz: "float32"?`},
		{`tensör([1.0]).türe()`, `türe("bf16") 1 parametre (tür adı) bekler`},
		{`tensör([1], tür: yok)`, `tür metin olmalıdır`},
		{`tensör([1], dtype: "bf16")`, `'dtype' adında bir parametre almaz`},
		{`tensör([1.0]).tür_`, `Bunu mu demek istediniz: 'tür'?`},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}

// karışık_hassasiyet: blok değeri, iç içe bloklar, hatada geri dönüş ve geri yayılım.
func TestKarisikHassasiyetBlok(t *testing.T) {
	out := mustCalistir(t, `değişken x = tensör([[1.001]])
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
	want := "[[1.0]] float32\n[[1.0009766]]\n[[1.001]]\n[1.0]\n[[3.0]] [[1.001]]\nSatır 11: dur\n[[1.001]]\n5 yok\n"
	if out != want {
		t.Fatalf("karışık_hassasiyet:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct{ kod, beklenen string }{
		{`karışık_hassasiyet("bf16")`, `karışık_hassasiyet("bf16") bir blok bekler: karışık_hassasiyet("bf16") { ... }`},
		{`karışık_hassasiyet("bf17") { 1 }`, `bilinmeyen veri türü "bf17"`},
		{`karışık_hassasiyet("bf16", 3)`, `ikinci parametre bir blok olmalıdır, Tam verildi`},
		{`karışık_hassasiyet() { 1 }`, `bir tür adı ve bir blok bekler`},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}

// Blok içindeki gradyanlar float32 gradyanlarına yakındır; ağırlıklar float32 kalır.
func TestKarisikHassasiyetGradyan(t *testing.T) {
	out := mustCalistir(t, `fonksiyon hesap(karışık) {
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
func TestKarisikHassasiyetGorevIzolasyonu(t *testing.T) {
	out := mustCalistir(t, `değişken x = tensör([[1.001]])
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
func TestTensorKaydetSurum2(t *testing.T) {
	klasor := t.TempDir()
	yol := filepath.ToSlash(filepath.Join(klasor, "turler.trt"))
	out := mustCalistir(t, `değişken p = {"a": tensör([[1.5, -2.25], [3.0, 0.1]], tür: "bf16"), "b": tensör([0.1, 65504.0, 0.00001], tür: "fp16"), "c": tensör([0.1])}
tensör_kaydet("`+yol+`", p)
değişken y = tensör_yükle("`+yol+`", türev_izle: doğru)
yazdır(y["a"].tür, y["b"].tür, y["c"].tür)
yazdır(y["a"] == p["a"], y["b"] == p["b"], y["c"] == p["c"], y["a"].değer)
yazdır(y["b"].gradyan, y["b"])`)
	want := "bf16 fp16 float32\n" +
		"doğru doğru doğru [[1.5, -2.25], [3.0, 0.100097656]]\n" +
		"yok Tensör[3] fp16 (türev izleniyor) [9.9976e-02, 6.5504e+04, 1.0014e-05]\n"
	if out != want {
		t.Fatalf("sürüm 2 gidiş-dönüş:\n%q\nbeklenen:\n%q", out, want)
	}
	veri, err := os.ReadFile(filepath.FromSlash(yol))
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(veri[8:10]) != 2 {
		t.Fatalf("sürüm 2 bekleniyordu: % x", veri[:10])
	}
	// Başlık 14 + her girdi (4 + ad + 2 + 8·boyut + veri) + sağlama 4.
	beklenenBoy := 14 + (4 + 1 + 2 + 16 + 4*2) + (4 + 1 + 2 + 8 + 3*2) + (4 + 1 + 2 + 8 + 4) + 4
	if len(veri) != beklenenBoy {
		t.Fatalf("dosya boyu %d, beklenen %d (16 bitlik veri 2 bayt yazılmalı)", len(veri), beklenenBoy)
	}
	// İlk girdinin tür kodu: başlık (14) + ad uzunluğu (4) + "a".
	if veri[14+4+1] != 2 {
		t.Fatalf("bf16 tür kodu 2 olmalı, %d", veri[14+4+1])
	}

	// Yalnızca sürüm 2 okunur; başka sürüm açık bir hatayla reddedilir.
	v1 := surum1Dosyasi(t, "w", []int{2}, []float32{0.5, -1.25}, 1)
	v1yol := filepath.ToSlash(filepath.Join(klasor, "eski.trt"))
	if err := os.WriteFile(filepath.FromSlash(v1yol), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := calistir(t, `tensör_yükle("`+v1yol+`")`); err == nil || !strings.Contains(err.Error(), "dosya biçimi sürümü 1; bu TR sürümü yalnızca sürüm 2 okur") {
		t.Errorf("sürüm 1 dosyası reddedilmeli: %v", err)
	}
	// Eksik 16 bitlik veri bayt sayısıyla bildirilir.
	if _, err := calistir(t, `tensör_yükle("`+kirpikYaz(t, klasor, veri, 14+4+1+2+16+4)+`")`); err == nil || !strings.Contains(err.Error(), "8 bayt veri bekliyor") {
		t.Errorf("eksik bf16 verisi: %v", err)
	}
}

// surum1Dosyasi tek girdili, artık okunmayan eski sürüm 1 biçiminde bir dosya üretir.
func surum1Dosyasi(t *testing.T, ad string, sekil []int, veri []float32, turKod byte) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("TRTENSOR")
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(1))
	binary.Write(&b, binary.LittleEndian, uint32(len(ad)))
	b.WriteString(ad)
	b.WriteByte(turKod)
	b.WriteByte(byte(len(sekil)))
	for _, d := range sekil {
		binary.Write(&b, binary.LittleEndian, uint64(d))
	}
	for _, x := range veri {
		binary.Write(&b, binary.LittleEndian, math.Float32bits(x))
	}
	binary.Write(&b, binary.LittleEndian, crc32.ChecksumIEEE(b.Bytes()))
	return b.Bytes()
}

// kirpikYaz verinin ilk n baytını geçici bir dosyaya yazar ve yolunu verir.
func kirpikYaz(t *testing.T, klasor string, veri []byte, n int) string {
	t.Helper()
	p := filepath.Join(klasor, "kirpik"+strconv.Itoa(n)+".trt")
	if err := os.WriteFile(p, veri[:n], 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(p)
}

// İyileştiricilerin ana_kopya seçeneği SetMasterWeights'e bağlanır.
func TestIyilestiriciAnaKopya(t *testing.T) {
	out := mustCalistir(t, `fonksiyon eğit(ad, ana) {
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
	if _, err := calistir(t, `AdamW([tensör([1.0], türev_izle: doğru)], ana_kopya: "evet")`); err == nil || !strings.Contains(err.Error(), "ana_kopya doğru ya da yanlış olmalıdır") {
		t.Errorf("ana_kopya türü: %v", err)
	}
}
