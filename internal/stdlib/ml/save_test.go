package ml

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	cases := []struct {
		fpath string
		want  messages.Code
	}{
		{write("eksik.trt", data[:len(data)-20]), messages.H6601},
		{write("bozuk.trt", broken), messages.H6627},
		{write("baslik.trt", []byte("merhaba dünya, bu bir tensör dosyası değil")), messages.H6610},
		{write("kisa.trt", []byte("TRT")), messages.H6610},
		{write("surum.trt", version), messages.H6611},
		{write("tur.trt", dtype), messages.H6616},
		{write("fazla.trt", append(append([]byte{}, data...), 0)), messages.H6628},
		{filepath.ToSlash(filepath.Join(dir, "yok.trt")), messages.H2120},
		{filepath.ToSlash(dir), messages.H6629},
	}
	for _, d := range cases {
		_, err := run(t, `tensör_yükle("`+d.fpath+`")`)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", filepath.Base(d.fpath), err, d.want)
		}
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`tensör_kaydet("` + fpath + `", {"a": 1})`, messages.H6608},
		{`tensör_kaydet("` + fpath + `", {1: tensör([1])})`, messages.H6606},
		{`tensör_kaydet("` + fpath + `", [tensör([1])])`, messages.H6605},
		{`tensör_kaydet("", {})`, messages.H6603},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}
