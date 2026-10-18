package tensor

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaksIndeks(t *testing.T) {
	out := mustCalistir(t, `değişken x = tensör([[1, 9, 3], [7, 2, 7]])
yazdır(maks_indeks(x).değer, maks_indeks(x, eksen: 0).değer, maks_indeks(tensör([4, 8, 1])).değer)
yazdır(tam(maks_indeks(tensör([4, 8, 1])).değer), maks_indeks(x).şekil)
yazdır(gömme(tensör([[0, 0], [1, 1], [2, 2]]), maks_indeks(tensör([[0, 0, 5]]))).değer)
değişken harfler = ["a", "b", "c"]
değişken i = maks_indeks(tensör([[[0, 9, 1], [5, 0, 0]]]), eksen: -1)
yazdır(harfler[i.değer[0][0]] + harfler[i.değer[0][1]], tip(i.değer[0][1]), tip(maks_indeks(tensör([1, 3])).değer))
yazdır(i, maks_indeks(tensör([2, 1])))
yazdır(tip((i * 2).değer[0][0]), tip(tensör(i).değer[0][0]))`)
	want := "[1, 0] [1, 0, 1] 1\n1 [2]\n[[2.0, 2.0]]\n" +
		"ba Tam Tam\n" +
		"Tensör[1×2] [\n  [1, 0]\n] Tensör(0)\n" +
		"Ondalık Ondalık\n"
	if out != want {
		t.Fatalf("maks_indeks:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct{ kod, beklenen string }{
		{"maks_indeks([1, 2])", "maks_indeks: bir tensör bekler, Liste verildi"},
		{"maks_indeks(tensör([1, 2]), 0)", "maks_indeks(x, eksen: 0): eksen adlı verilir"},
		{"maks_indeks(tensör([1, 2]), eksen: 1)", "eksen 1, [2] şeklindeki 1 boyutlu tensör için geçersiz"},
		{"maks_indeks(sıfırlar([0]))", "boş"},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}

func TestGradyanKirp(t *testing.T) {
	out := mustCalistir(t, `değişken w = tensör([1.0, 1.0], türev_izle: doğru)
değişken b = tensör([1.0], türev_izle: doğru)
geri_yayılım(toplam(w * tensör([3.0, 0.0])) + toplam(b * 4))
yazdır(gradyan_kırp([w, b], en_fazla: 1.0))
yazdır(w.gradyan.değer, b.gradyan.değer)
yazdır(gradyan_kirp([w, b]))`)
	want := "5.0\n[0.6, 0.0] [0.8]\n1.0\n"
	if out != want {
		t.Fatalf("gradyan_kırp:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct{ kod, beklenen string }{
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], en_fazla: 0)", "en_fazla pozitif bir sayı olmalıdır"},
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], max_norm: 1)", "'max_norm' adında bir parametre almaz"},
		{"gradyan_kırp([sıfırlar([1])])", "türev izlemiyor"},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}

func TestTensorKaydetYukle(t *testing.T) {
	klasor := t.TempDir()
	yol := filepath.ToSlash(filepath.Join(klasor, "model.trt"))
	out := mustCalistir(t, `değişken p = {"gömme": tensör([[0.1, 0.2], [0.3, 0.4]]), "sapma": tensör([1.5]), "ölçek": toplam(tensör([2.0, 3.0])), "boş": sıfırlar([0, 3])}
tensör_kaydet("`+yol+`", p)
değişken y = tensör_yükle("`+yol+`")
yazdır(y.anahtarlar())
yazdır(y["gömme"].değer, y["sapma"].değer, y["ölçek"].değer, y["boş"].şekil)
yazdır(y["gömme"].değer[0][0] == 0.1, y["sapma"].gradyan)
değişken z = tensor_yukle("`+yol+`", türev_izle: doğru)
geri_yayılım(toplam(z["sapma"] * 2))
yazdır(z["sapma"].gradyan.değer)`)
	want := "[\"gömme\", \"sapma\", \"ölçek\", \"boş\"]\n" +
		"[[0.1, 0.2], [0.3, 0.4]] [1.5] 5.0 [0, 3]\n" +
		"doğru yok\n" +
		"[2.0]\n"
	if out != want {
		t.Fatalf("kaydet/yükle:\n%q\nbeklenen:\n%q", out, want)
	}

	// Biçim: başlık, sürüm ve tür alanı yerinde.
	veri, err := os.ReadFile(filepath.FromSlash(yol))
	if err != nil {
		t.Fatal(err)
	}
	if string(veri[:8]) != "TRTENSOR" || binary.LittleEndian.Uint16(veri[8:10]) != 1 || binary.LittleEndian.Uint32(veri[10:14]) != 4 {
		t.Fatalf("başlık hatalı: % x", veri[:14])
	}

	yaz := func(ad string, b []byte) string {
		p := filepath.Join(klasor, ad)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(p)
	}
	bozuk := append([]byte{}, veri...)
	bozuk[len(bozuk)-10] ^= 0xff
	surum := append([]byte{}, veri...)
	surum[8] = 9
	tur := append([]byte{}, veri...)
	// İlk girdinin tür baytı: başlık (14) + ad uzunluğu (4) + "gömme" (6 bayt).
	tur[14+4+len("gömme")] = 7
	durumlar := []struct{ yol, beklenen string }{
		{yaz("eksik.trt", veri[:len(veri)-20]), "dosya bozuk ya da eksik"},
		{yaz("bozuk.trt", bozuk), "sağlama toplamı tutmuyor"},
		{yaz("baslik.trt", []byte("merhaba dünya, bu bir tensör dosyası değil")), "bir TR tensör dosyası değil"},
		{yaz("kisa.trt", []byte("TRT")), "bir TR tensör dosyası değil"},
		{yaz("surum.trt", surum), "dosya biçimi sürümü 9; bu TR sürümü en fazla 1 okuyabilir"},
		{yaz("tur.trt", tur), "desteklenmeyen veri türü kodu 7"},
		{yaz("fazla.trt", append(append([]byte{}, veri...), 0)), "fazladan veri"},
		{filepath.ToSlash(filepath.Join(klasor, "yok.trt")), "dosya bulunamadı"},
		{filepath.ToSlash(klasor), "bir klasör"},
	}
	for _, d := range durumlar {
		_, err := calistir(t, `tensör_yükle("`+d.yol+`")`)
		if err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", filepath.Base(d.yol), err, d.beklenen)
		}
	}
	for _, d := range []struct{ kod, beklenen string }{
		{`tensör_kaydet("` + yol + `", {"a": 1})`, "\"a\" değeri bir tensör olmalıdır, Tam verildi"},
		{`tensör_kaydet("` + yol + `", {1: tensör([1])})`, "sözlük anahtarları metin olmalıdır"},
		{`tensör_kaydet("` + yol + `", [tensör([1])])`, "adları tensör olan bir sözlük olmalıdır"},
		{`tensör_kaydet("", {})`, "dosya yolu boş"},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}
