package ml

import "testing"

// Denetim (2026-10-21): kaynaştırılan fonksiyonun dışarıdan yakaladığı bir sayı (argüman
// değil) programa kayıt anında sabit olarak gömülür; önbellek anahtarı yalnızca
// argümanlardır. Sayı sonradan değişince kaynaşmış fonksiyon eski değerle hesaplar ve
// kaynaşmamış hesaptan ayrılır (dil tanımı 15.45: "Sonuç kaynaşmamış hesapla aynıdır").
func TestAuditFuseCapturedNumberIsBakedIn(t *testing.T) {
	out := mustRun(t, `değişken ölçek = 2.0
fonksiyon f(x) { sonuç x * ölçek }
değişken kf = kaynaştır(f)
değişken t = tensör([1.0])
yazdır(kf(t).değer, f(t).değer)
ölçek = 3.0
yazdır(kf(t).değer, f(t).değer)`)
	want := "[2.0] [2.0]\n[3.0] [3.0]\n"
	if out != want {
		t.Fatalf("kaynaşmış sonuç kaynaşmamıştan ayrıldı:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Denetim (2026-10-21): dil tanımı 15.45 karşılaştırmaları kaynaştırılabilir işlemler
// arasında sayar (fuse_grad.go'da türevleri de vardır: 0), ama tensor.Compare iz tensörünü
// tanımaz ve verinin kendisini ister: kaynaştırılan fonksiyonda '<', '>' H3107 verir.
func TestAuditFuseComparisonsAreNotFusable(t *testing.T) {
	for _, code := range []string{
		`yazdır(kaynaştır((a) => a > 0.0)(tensör([1.0, -1.0])).değer)`,
		`yazdır(kaynaştır((a, b) => a < b)(tensör([1.0, -1.0]), tensör([0.0, 0.0])).değer)`,
	} {
		if _, err := run(t, code); err != nil {
			t.Errorf("%s\nkaynaştırılan karşılaştırma hata verdi: %v", code, err)
		}
	}
}
