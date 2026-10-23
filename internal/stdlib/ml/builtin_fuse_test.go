package ml

import "testing"

// Kaynaştırılan fonksiyonun dışarıdan okuduğu sayı kayıtta (ilk çağrıda) programa sabit
// olarak girer (dil tanımı 15.44; JAX jit de kapanıştaki sayıları iz anında sabitler).
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

// Dil tanımı 15.44 karşılaştırmaları kaynaştırılabilir işlemler arasında sayar (türevleri 0):
// kaynaştırılan fonksiyonda '<', '>' hata vermez.
func TestFuseComparisons(t *testing.T) {
	for _, code := range []string{
		`yazdır(kaynaştır((a) => a > 0.0)(tensör([1.0, -1.0])).değer)`,
		`yazdır(kaynaştır((a, b) => a < b)(tensör([1.0, -1.0]), tensör([0.0, 0.0])).değer)`,
	} {
		if _, err := run(t, code); err != nil {
			t.Errorf("%s\nkaynaştırılan karşılaştırma hata verdi: %v", code, err)
		}
	}
}

// Kural: argümansız çağrılan kaynaştırılmış fonksiyon kullanıcı hatası verir (argüman sayısı).
// Neden: boş args'a erişmek iç hataya (H9001) dönüşür.
func TestFuseCallWithoutArgumentsIsUserError(t *testing.T) {
	_, err := run(t, `değişken kf = kaynaştır(() => tensör([1.0]))
kf()`)
	if err == nil {
		t.Fatal("argümansız kaynaştır çağrısı hata vermeli")
	}
	msg := err.Error()
	for i := 0; i+len("index out of range") <= len(msg); i++ {
		if msg[i:i+len("index out of range")] == "index out of range" {
			t.Fatalf("kullanıcı hatası yerine dilim sınırı paniği (H9001): %v", err)
		}
	}
}

// Kural: kaynaştırılan fonksiyon en az bir tensör döndürür (15.44); boş liste hatadır (H6436),
// türevli ve türevsiz kipte aynı. Neden: türevli kipte çekirdek iç hata (H3028) verirdi.
func TestFuseRejectsFunctionReturningEmptyList(t *testing.T) {
	if out, err := run(t, `değişken kf = kaynaştır((x) => [])
yazdır(kf(tensör([1.0])))`); err == nil {
		t.Fatalf("boş liste döndüren kaynaştır fonksiyonu hata vermeliydi, çıktı: %q", out)
	}
}

// Kural: kaynaştır önbellek anahtarı etkin hesap türünü (hesap_türü()) içerir (15.44). Neden:
// iz hesap türünü sabitler; anahtarda yoksa blok içindeki çağrı blok dışındaki programı çalıştırır.
func TestFuseCacheKeyIncludesComputeDType(t *testing.T) {
	out := mustRun(t, `değişken kf = kaynaştır((x) => x.türe(hesap_türü()))
değişken t = tensör([1.0])
yazdır(kf(t).tür)
karışık_hassasiyet("bf16") {
    yazdır(kf(t).tür)
}`)
	want := "float32\nbf16\n"
	if out != want {
		t.Fatalf("kaynaştır önbelleği karışık_hassasiyet türünü yok saydı:\n%q\nbeklenen:\n%q", out, want)
	}
}
