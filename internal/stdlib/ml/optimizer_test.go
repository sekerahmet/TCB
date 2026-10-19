package ml

import (
	"strings"
	"testing"
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
	for _, d := range []struct{ code, expected string }{
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], en_fazla: 0)", "en_fazla pozitif bir sayı olmalıdır"},
		{"gradyan_kırp([tensör([1.0], türev_izle: doğru)], max_norm: 1)", "'max_norm' adında bir parametre almaz"},
		{"gradyan_kırp([sıfırlar([1])])", "türev izlemiyor"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
}
