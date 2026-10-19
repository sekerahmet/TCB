package ml

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTensorSaveLoad(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.ToSlash(filepath.Join(dir, "model.trt"))
	out := mustRun(t, `değişken p = {"gömme": tensör([[0.1, 0.2], [0.3, 0.4]]), "sapma": tensör([1.5]), "ölçek": toplam(tensör([2.0, 3.0])), "boş": sıfırlar([0, 3])}
tensör_kaydet("`+fpath+`", p)
değişken y = tensör_yükle("`+fpath+`")
yazdır(y.anahtarlar())
yazdır(y["gömme"].değer, y["sapma"].değer, y["ölçek"].değer, y["boş"].şekil)
yazdır(y["gömme"].değer[0][0] == 0.1, y["sapma"].gradyan)
değişken z = tensör_yükle("`+fpath+`", türev_izle: doğru)
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
	data, err := os.ReadFile(filepath.FromSlash(fpath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:8]) != "TRTENSOR" || binary.LittleEndian.Uint16(data[8:10]) != 2 || binary.LittleEndian.Uint32(data[10:14]) != 4 {
		t.Fatalf("başlık hatalı: % x", data[:14])
	}

	write := func(ad string, b []byte) string {
		p := filepath.Join(dir, ad)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(p)
	}
	broken := append([]byte{}, data...)
	broken[len(broken)-10] ^= 0xff
	version := append([]byte{}, data...)
	version[8] = 9
	dtype := append([]byte{}, data...)
	// İlk girdinin tür baytı: başlık (14) + ad uzunluğu (4) + "gömme" (6 bayt).
	dtype[14+4+len("gömme")] = 7
	cases := []struct{ fpath, expected string }{
		{write("eksik.trt", data[:len(data)-20]), "dosya bozuk ya da eksik"},
		{write("bozuk.trt", broken), "sağlama toplamı tutmuyor"},
		{write("baslik.trt", []byte("merhaba dünya, bu bir tensör dosyası değil")), "bir TR tensör dosyası değil"},
		{write("kisa.trt", []byte("TRT")), "bir TR tensör dosyası değil"},
		{write("surum.trt", version), "dosya biçimi sürümü 9; bu TR sürümü yalnızca sürüm 2 okur"},
		{write("tur.trt", dtype), "desteklenmeyen veri türü kodu 7"},
		{write("fazla.trt", append(append([]byte{}, data...), 0)), "fazladan veri"},
		{filepath.ToSlash(filepath.Join(dir, "yok.trt")), "yüklenemedi: böyle bir dosya ya da klasör yok"},
		{filepath.ToSlash(dir), "bir klasör"},
	}
	for _, d := range cases {
		_, err := run(t, `tensör_yükle("`+d.fpath+`")`)
		if err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", filepath.Base(d.fpath), err, d.expected)
		}
	}
	for _, d := range []struct{ code, expected string }{
		{`tensör_kaydet("` + fpath + `", {"a": 1})`, "\"a\" değeri bir tensör olmalıdır, Tam verildi"},
		{`tensör_kaydet("` + fpath + `", {1: tensör([1])})`, "sözlük anahtarları metin olmalıdır"},
		{`tensör_kaydet("` + fpath + `", [tensör([1])])`, "adları tensör olan bir sözlük olmalıdır"},
		{`tensör_kaydet("", {})`, "dosya yolu boş"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
}
