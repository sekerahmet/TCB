package ml

import "testing"

// Kaynaştırılan fonksiyonun dışarıdan okuduğu sayı kayıtta (ilk çağrıda) programa sabit
// olarak girer (dil tanımı 15.45; JAX jit de kapanıştaki sayıları iz anında sabitler).
// Sonradan değişen sayı kaynaşmış sonucu değiştirmez; argüman olarak verilen değiştirir.
func TestFuseCapturedNumberIsBakedIn(t *testing.T) {
	out := mustRun(t, `değişken ölçek = 2.0
fonksiyon f(x) { sonuç x * ölçek }
fonksiyon g(x, s) { sonuç x * s }
değişken kf = kaynaştır(f)
değişken kg = kaynaştır(g)
değişken t = tensör([1.0])
yazdır(kf(t).değer, kg(t, ölçek).değer)
ölçek = 3.0
yazdır(kf(t).değer, kg(t, ölçek).değer)`)
	want := "[2.0] [2.0]\n[2.0] [3.0]\n"
	if out != want {
		t.Fatalf("dış sayı kayıtta sabitlenmedi ya da argüman değişmedi:\n%q\nbeklenen:\n%q", out, want)
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
