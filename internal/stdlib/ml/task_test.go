package ml

import (
	"testing"
)

// watchedTR, bir tensörün hesap çizgesine kaydedilip kaydedilmediğini (geri yayılım
// yapılabiliyor mu) söyleyen TR yardımcısıdır; bekle_koşul koşul doğru olana dek (en çok
// 10 sn) kilidi bırakarak bekler. Testler süreye değil el sıkışmaya (bayraklara) dayanır.
const watchedTR = `TR_func izli(t) {
    TR_try { TR_backward(TR_sum(t)) } TR_catch h { TR_return TR_false }
    TR_return TR_true
}
TR_func bekle_koşul(k) {
    TR_for i TR_in 0..1000 {
        TR_if k() { TR_return TR_true }
        TR_sleep(0.01)
    }
    TR_return TR_false
}
TR_var d = {"başladı": TR_false, "devam": TR_false}
`

// türevsiz { } göreve bağlıdır: blok içindeki fonksiyon çağrıları ve döngü turları
// yorumlayıcı kilidini devrettiğinde, aynı anda çalışan başka görevin işlemleri çizgeye
// kaydedilmeye devam eder (hakem kanıtı: 3000 denemenin 2745'i "türev izlemiyor" hatası
// veriyordu).
func TestNoGradTaskBound(t *testing.T) {
	out := mustRun(t, `TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_func f(x) { TR_return x * 2 }
TR_var bitti = TR_false
TR_async {
    TR_no_grad {
        TR_for i TR_in 0..20000 { f(w) }
    }
    bitti = TR_true
}
TR_var izsiz = 0
TR_var deneme = 0
TR_for i TR_in 0..3000 {
    TR_var y = TR_sum(w * 3)
    deneme += 1
    TR_try { TR_backward(y) } TR_catch h { izsiz += 1 }
    TR_if bitti { TR_break }
}
TR_wait_all()
TR_print(deneme > 0, izsiz)`)
	if out != "true 0\n" {
		t.Fatalf("türevsiz başka görevi etkiledi:\n%s", out)
	}
}

// türevsiz bloğu kilidi bırakan bir çağrıdayken (bekle) başka görev türevli çalışır; blok
// beklemeden dönünce kendi türevsiz durumunu görür; yeni görev türevli başlar.
func TestNoGradAwaitTaskIsolation(t *testing.T) {
	out := mustRun(t, watchedTR+`TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var a = []
TR_var b = []
TR_async {
    TR_no_grad {
        a.TR_append(izli(w * 2))
        d["başladı"] = TR_true
        bekle_koşul(() => d["devam"])
        a.TR_append(izli(w * 2))
        TR_async { b.TR_append(izli(w * 3)) }
    }
    a.TR_append(izli(w * 2))
}
bekle_koşul(() => d["başladı"])
b.TR_append(izli(w * 2))
d["devam"] = TR_true
TR_wait_all()
TR_print(a, b)`)
	if out != "[false, false, true] [true, true]\n" {
		t.Fatalf("türevsiz görev izolasyonu:\n%s", out)
	}
}

// özel_türev'in ileri ve geri hesapları da görev başına türevsiz çalışır: ileri ya da geri
// kilidi bıraktığında başka görevin hesabı çizgeye kaydedilir.
func TestCustomGradForwardBackwardTaskBound(t *testing.T) {
	out := mustRun(t, watchedTR+`TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var v = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var adım = {"ileri": TR_false, "ileri_devam": TR_false, "geri": TR_false, "geri_devam": TR_false}
TR_var yavaş = TR_custom_grad(
    TR_forward: TR_func(x) { adım["ileri"] = TR_true; bekle_koşul(() => adım["ileri_devam"]); TR_return [x * 1, []] },
    TR_backward: TR_func(s, g) { adım["geri"] = TR_true; bekle_koşul(() => adım["geri_devam"]); TR_return [g] }
)
TR_async {
    TR_var y = yavaş(w)
    TR_backward(TR_sum(y))
}
bekle_koşul(() => adım["ileri"])
TR_var i1 = izli(v * 2)
adım["ileri_devam"] = TR_true
bekle_koşul(() => adım["geri"])
TR_var i2 = izli(v * 2)
adım["geri_devam"] = TR_true
TR_wait_all()
TR_print(i1, i2, w.TR_grad.TR_tolist)`)
	if out != "true true [1.0, 1.0]\n" {
		t.Fatalf("özel_türev görev izolasyonu:\n%s", out)
	}
}

// Başka bir görevin türevsiz bloğu sürerken, türev izlenen tensöre indeksle atama türevli
// görevde hâlâ reddedilir.
func TestNoGradIndexAssignTaskBound(t *testing.T) {
	out := mustRun(t, watchedTR+`TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_async {
    TR_no_grad { d["başladı"] = TR_true; bekle_koşul(() => d["devam"]) }
}
bekle_koşul(() => d["başladı"])
TR_var h = ""
TR_try { w[0] = 5 } TR_catch e { h = "\(e)" }
d["devam"] = TR_true
TR_wait_all()
TR_print(w.TR_tolist, h.TR_contains("no_grad"))`)
	if out != "[1.0, 2.0] true\n" {
		t.Fatalf("başka görevin türevsiz bloğu atamaya izin verdi:\n%s", out)
	}
}

// Bir görevin geri yayılımı özel_türev'in geri hesabında kilidi bırakırken, başka görev aynı
// çizgenin düğümlerinden geri yayılım başlatırsa açık hata alır (gradyanlar iki kez ya da
// eksik birikmez).
func TestConcurrentBackwardSameGraph(t *testing.T) {
	out := mustRun(t, watchedTR+`TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var yavaş = TR_custom_grad(
    TR_forward: TR_func(x) { TR_return [x * 1, []] },
    TR_backward: TR_func(s, g) { d["başladı"] = TR_true; bekle_koşul(() => d["devam"]); TR_return [g] }
)
TR_var o = yavaş(w)
TR_var k1 = TR_sum(o * 3)
TR_var k2 = TR_sum(o * 2)
TR_async { TR_backward(k1) }
bekle_koşul(() => d["başladı"])
TR_var h = ""
TR_try { TR_backward(k2) } TR_catch e { h = "\(e)" }
d["devam"] = TR_true
TR_wait_all()
TR_print(h.TR_contains("another backward pass"), w.TR_grad.TR_tolist)`)
	if out != "true [3.0, 3.0]\n" {
		t.Fatalf("aynı çizgeden eşzamanlı geri yayılım:\n%s", out)
	}
}
