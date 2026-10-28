package ml

import "testing"

// Kaynaştırılan fonksiyonun dışarıdan okuduğu sayı kayıtta (ilk çağrıda) programa sabit
// olarak girer (dil tanımı 15.44; JAX jit de kapanıştaki sayıları iz anında sabitler).
// Sonradan değişen sayı kaynaşmış sonucu değiştirmez; argüman olarak verilen değiştirir.
func TestFuseCapturedNumberIsBakedIn(t *testing.T) {
	out := mustRun(t, `TR_var ölçek = 2.0
TR_func f(x) { TR_return x * ölçek }
TR_func g(x, s) { TR_return x * s }
TR_var kf = TR_fuse(f)
TR_var kg = TR_fuse(g)
TR_var t = TR_Tensor([1.0])
TR_print(kf(t).TR_tolist, kg(t, ölçek).TR_tolist)
ölçek = 3.0
TR_print(kf(t).TR_tolist, kg(t, ölçek).TR_tolist)`)
	want := "[2.0] [2.0]\n[2.0] [3.0]\n"
	if out != want {
		t.Fatalf("dış sayı kayıtta sabitlenmedi ya da argüman değişmedi:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Dil tanımı 15.44 karşılaştırmaları kaynaştırılabilir işlemler arasında sayar (türevleri 0):
// kaynaştırılan fonksiyonda '<', '>' hata vermez.
func TestFuseComparisons(t *testing.T) {
	for _, code := range []string{
		`TR_print(TR_fuse((a) => a > 0.0)(TR_Tensor([1.0, -1.0])).TR_tolist)`,
		`TR_print(TR_fuse((a, b) => a < b)(TR_Tensor([1.0, -1.0]), TR_Tensor([0.0, 0.0])).TR_tolist)`,
	} {
		if _, err := run(t, code); err != nil {
			t.Errorf("%s\nkaynaştırılan karşılaştırma hata verdi: %v", code, err)
		}
	}
}

// Kural: argümansız çağrılan kaynaştırılmış fonksiyon kullanıcı hatası verir (argüman sayısı).
// Neden: boş args'a erişmek iç hataya (H9001) dönüşür.
func TestFuseCallWithoutArgumentsIsUserError(t *testing.T) {
	_, err := run(t, `TR_var kf = TR_fuse(() => TR_Tensor([1.0]))
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
	if out, err := run(t, `TR_var kf = TR_fuse((x) => [])
TR_print(kf(TR_Tensor([1.0])))`); err == nil {
		t.Fatalf("boş liste döndüren kaynaştır fonksiyonu hata vermeliydi, çıktı: %q", out)
	}
}

// Kural: kaynaştır önbellek anahtarı etkin hesap türünü (hesap_türü()) içerir (15.44). Neden:
// iz hesap türünü sabitler; anahtarda yoksa blok içindeki çağrı blok dışındaki programı çalıştırır.
func TestFuseCacheKeyIncludesComputeDType(t *testing.T) {
	out := mustRun(t, `TR_var kf = TR_fuse((x) => x.TR_to_dtype(TR_compute_dtype()))
TR_var t = TR_Tensor([1.0])
TR_print(kf(t).TR_dtype)
TR_mixed_precision("bf16") {
    TR_print(kf(t).TR_dtype)
}`)
	want := "float32\nbf16\n"
	if out != want {
		t.Fatalf("kaynaştır önbelleği karışık_hassasiyet türünü yok saydı:\n%q\nbeklenen:\n%q", out, want)
	}
}
