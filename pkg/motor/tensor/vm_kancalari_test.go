package tensor

import (
	"strings"
	"testing"
)

// VM kancaları: eşitlik, sağ işlenen, '**', karşılaştırma, tekli eksi ve indeksleme.
func TestVMKancalari(t *testing.T) {
	out := mustCalistir(t, `
		değişken t = tensör([[1.0, 2.0], [3.0, 4.0]])
		yazdır(tensör([1.00001]) == tensör([1.0]))
		yazdır(tensör([1.0, 2.0]) == tensör([1.0, 2.0]))
		yazdır(tensör([1.0, 2.0]) != tensör([[1.0, 2.0]]))
		yazdır((10 - t).değer)
		yazdır((12 / t).değer)
		yazdır((-t).değer)
		yazdır((t ** 2).değer)
		yazdır((t > 2).değer)
		yazdır((2 >= t).değer)
		yazdır(t[1].değer)
		yazdır(t[-1][0].değer)
		yazdır(t[0..1].şekil)
		yazdır([1, 2] @ tensör([[1.0], [1.0]]))
	`)
	want := []string{
		"yanlış", "doğru", "doğru",
		"[[9.0, 8.0], [7.0, 6.0]]",
		"[[12.0, 6.0], [4.0, 3.0]]",
		"[[-1.0, -2.0], [-3.0, -4.0]]",
		"[[1.0, 4.0], [9.0, 16.0]]",
		"[[0.0, 0.0], [1.0, 1.0]]",
		"[[1.0, 1.0], [0.0, 0.0]]",
		"[3.0, 4.0]",
		"3.0",
		"[1, 2]",
	}
	satirlar := strings.Split(strings.TrimSpace(out), "\n")
	for i, w := range want {
		if i >= len(satirlar) || satirlar[i] != w {
			t.Fatalf("%d. satır: beklenen %q\ntüm çıktı:\n%s", i+1, w, out)
		}
	}
}

// Kanca hataları satır bilgisi taşımalı ve yol göstermeli; türev sağ işlenen yolunda da akmalı.
func TestVMKancaHatalariVeTurev(t *testing.T) {
	_, err := calistir(t, "değişken t = tensör([1.0])\n\ndeğişken y = t % 2\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 3:") || !strings.Contains(err.Error(), "desteklenen işleçler") {
		t.Fatalf("satır bilgili, yol gösteren hata bekleniyordu: %v", err)
	}
	_, err = calistir(t, "değişken t = tensör([1.0, 2.0])\nyazdır(t[5])\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 2:") || !strings.Contains(err.Error(), "sınır dışı") {
		t.Fatalf("indeks hatası bekleniyordu: %v", err)
	}
	_, err = calistir(t, "değişken t = tensör([[1.0, 2.0]])\nyazdır(t @ tensör([[1.0, 2.0]]))\n")
	if err == nil || !strings.HasPrefix(err.Error(), "Satır 2:") {
		t.Fatalf("'@' hatası satır bilgisi taşımalı: %v", err)
	}
	out := mustCalistir(t, `
		değişken w = tensör([2.0, 4.0], türev_izle: doğru)
		değişken kayip = toplam(1 / w - w[0] * 3)
		geri_yayılım(kayip)
		yazdır(w.gradyan.değer)
	`)
	if strings.TrimSpace(out) != "[-6.25, -0.0625]" {
		t.Fatalf("türev: %q", out)
	}
}
