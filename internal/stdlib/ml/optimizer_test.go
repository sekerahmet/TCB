package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestGradClip(t *testing.T) {
	out := mustRun(t, `değişken w = tensör([1.0, 1.0], türev_izle: doğru)
değişken b = tensör([1.0], türev_izle: doğru)
geri_yayılım(toplam(w * tensör([3.0, 0.0])) + toplam(b * 4))
yazdır(gradyan_kırp([w, b], en_fazla: 1.0).değer)
yazdır(w.gradyan.değer, b.gradyan.değer)
yazdır(gradyan_kırp([w, b]))`)
	// Kırpma öncesi norm 0 boyutlu tensördür; sayısı .değer ile okunur.
	want := "5.0\n[0.6, 0.0] [0.8]\nTensör(1.0000)\n"
	if out != want {
		t.Fatalf("gradyan_kırp:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], en_fazla: 0)", messages.H6429},
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], max_norm: 1)", messages.H7901},
		{"gradyan_kırp([sıfırlar([1])])", messages.H6513},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// AdamW'nin epsilon'u pozitif olmalı: 0 (ya da float32'de 0'a düşen değer) sıfır gradyanlı
// ilk adımda 0/(√0+0) = NaN üretirdi.
func TestAdamWEpsilonPositive(t *testing.T) {
	for _, eps := range []string{"0", "1e-50"} {
		_, err := run(t, "değişken w = tensör([0.0], türev_izle: doğru)\nAdamW([w], epsilon: "+eps+")")
		if err == nil || !messages.Has(err, messages.H6527) {
			t.Fatalf("epsilon %s: hata beklenirdi: %v", eps, err)
		}
	}
}

// Kural (15.46): gradyan_kırp(..., sonlu_değilse_hata: doğru) toplam norm NaN ya da sonsuzsa
// H6534 verir (PyTorch error_if_nonfinite); varsayılan (yanlış) ve sonlu normda davranış
// değişmez. Mantık olmayan değer açık hatadır.
// Neden: uzun eğitim bozuk bir gradyanla sessizce sürmesin; isteyen erken durur.
func TestGradClipErrorIfNonfinite(t *testing.T) {
	out := mustRun(t, `değişken w = tensör([3.0, 4.0], türev_izle: doğru)
geri_yayılım(toplam(w * w))
yazdır(gradyan_kırp([w], sonlu_değilse_hata: doğru).değer)
değişken v = tensör([1.0, 2.0], türev_izle: doğru)
geri_yayılım(toplam(v * ondalık("nan")))
yazdır(sonlu_mu(gradyan_kırp([v]).değer))`)
	if out != "10.0\nyanlış\n" {
		t.Fatalf("çıktı: %q", out)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`değişken v = tensör([1.0], türev_izle: doğru)
geri_yayılım(toplam(v * ondalık("nan")))
gradyan_kırp([v], sonlu_değilse_hata: doğru)`, messages.H6534},
		{`değişken v = tensör([1.0], türev_izle: doğru)
geri_yayılım(toplam(v * ondalık("inf")))
gradyan_kırp([v], en_fazla: 2.0, sonlu_değilse_hata: doğru)`, messages.H6534},
		{`değişken v = tensör([1.0], türev_izle: doğru)
geri_yayılım(toplam(v))
gradyan_kırp([v], sonlu_değilse_hata: 1)`, messages.H7905},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}
