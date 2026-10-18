package ogrenme

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

// Tensör üzerinde döngü ilk eksen boyunca gezer: 1 boyutluda sayılar (tam sayılı tensörde
// Tam), N boyutluda veriyi paylaşan, türev izlenen (N-1) boyutlu görünümler.
func TestTensorUzerindeDongu(t *testing.T) {
	out := mustCalistir(t, `
		değişken t = tensör([[1, 2], [3, 4], [5, 6]])
		döngü i, s içinde t { yazdır(i, s.şekil, s.değer) }
		yazdır([x * 2 döngü x içinde tensör([1.5, 2.5])])
		yazdır([x döngü x içinde tensör([0.1])][0] == 0.1)
		yazdır([tip(x) döngü x içinde maks_indeks(tensör([[1, 5], [7, 0]]), eksen: -1)])
		değişken k = maks_indeks(tensör([[[0, 9], [5, 0]]]), eksen: -1)
		döngü s içinde k { döngü x içinde s { yazdır(tip(x), x) } }
		değişken z = sıfırlar([2, 2])
		döngü satır içinde z { satır[0] = 9 }
		yazdır(z.değer)
		döngü x içinde sıfırlar([0]) { yazdır("hiç") }
		değişken w = tensör([[1.0, 2.0], [3.0, 4.0]], türev_izle: doğru)
		değişken kayip = toplam(tensör(0.0))
		döngü s içinde w { kayip = kayip + toplam(s * s) }
		geri_yayılım(kayip)
		yazdır(w.gradyan.değer)
	`)
	want := strings.Join([]string{
		"0 [2] [1.0, 2.0]",
		"1 [2] [3.0, 4.0]",
		"2 [2] [5.0, 6.0]",
		"[3.0, 5.0]",
		"doğru",
		`[Tam, Tam]`,
		"Tam 1",
		"Tam 0",
		"[[9.0, 0.0], [9.0, 0.0]]",
		"[[2.0, 4.0], [6.0, 8.0]]",
	}, "\n")
	if strings.TrimSpace(out) != want {
		t.Fatalf("tensör döngüsü:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, kod := range []string{
		"değişken a = tensör(3.0)\n\ndöngü x içinde a { }\n",
		"değişken a = tensör(3.0)\n\nyazdır([x döngü x içinde a])\n",
	} {
		_, err := calistir(t, kod)
		if err == nil || !strings.HasPrefix(err.Error(), "Satır 3:") || !strings.Contains(err.Error(), "0 boyutlu) tensör üzerinde döngü kurulamaz") {
			t.Errorf("0 boyutlu tensörde satır bilgili döngü hatası bekleniyordu: %v", err)
		}
	}
}

// Bulgu 7: indeksle atama tek kopya ve tek sürüm artışıdır (eleman eleman yazılmaz);
// bitişik olmayan hedefte de.
func TestIndekseAtamaTekKopya(t *testing.T) {
	v := deger(t, `değişken cikti = sıfırlar([300, 200])
cikti[0..300] = birler([300, 200])
cikti.devrik()[5] = 2
cikti[1][2] = 3`)
	d := v.(*Deger)
	if s := d.t.Version(); s != 3 {
		t.Fatalf("3 atama için sürüm %d; her atama bir kez artırmalı", s)
	}
	veri := d.t.Data()
	if veri[0] != 1 || veri[5] != 2 || veri[200+5] != 2 || veri[299*200+5] != 2 || veri[200+2] != 3 || veri[6] != 1 {
		t.Fatalf("atama sonucu yanlış: %v", veri[:8])
	}
}
