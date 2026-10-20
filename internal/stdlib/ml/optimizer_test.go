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
