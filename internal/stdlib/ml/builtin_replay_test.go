package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// replayStep KV önbelleği gibi kullanılan bir adım: indeksle gömme seçer, konuma yazar,
// sonucu döndürür. Aynı adım hem tekrar_oynat ile hem doğrudan çağrılır.
const replayStep = `
değişken w = rastgele_normal([6, 4], tohum: 1)
fonksiyon kur() { sonuç sıfırlar([1, 5, 4]) }
fonksiyon adım(ön, id, konum) {
    değişken e = gömme(w, id)
    yerine_yaz(ön, e.şekillendir([1, 1, 4]), eksen: 1, başlangıç: konum)
    sonuç e * 2 + 1
}
fonksiyon sür(f, ön) {
    değişken sonuçlar = []
    türevsiz {
        döngü i içinde 0..5 {
            sonuçlar.ekle(f(tensör([(i * 2) % 6 * 1.0]), tensör(i * 1.0)).değer)
        }
    }
    sonuç [sonuçlar, ön.değer]
}
`

// Kural (dil tanımı 15.48): tekrar_oynat'ın sonucu (ısınma, kayıt ve oynatma çağrıları)
// doğrudan çağrıyla aynıdır; kayıtta tensörle verilen konum ve indeks her oynatmada
// cihazdan okunur. CPU'da f doğrudan çağrılır.
func TestReplayMatchesDirectCalls(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, dev := range []string{"cpu", "gpu"} {
		out := mustRun(t, `varsayılan_cihaz("`+dev+`")`+replayStep+`
değişken ö1 = kur()
değişken ö2 = kur()
değişken oynatılan = tekrar_oynat((id, konum) => adım(ö1, id, konum))
değişken a = sür(oynatılan, ö1)
değişken b = sür((id, konum) => adım(ö2, id, konum), ö2)
yazdır(a == b)
yazdır(a[0][4])`)
		lines := strings.Split(out, "\n")
		if lines[0] != "doğru" {
			t.Fatalf("%s: tekrar_oynat doğrudan çağrıdan farklı:\n%s", dev, out)
		}
	}
}

// Kural: kaydın çıktısı kopyalanarak verilir; sonraki oynatma önceki çağrının sonucunu
// değiştirmez. Girdinin şekli değişince yeniden kaydedilir.
func TestReplayOutputsAreCopiesAndShapesRecapture(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
değişken f = tekrar_oynat((x) => x * 3)
türevsiz {
    değişken s = [f(tensör([1.0])), f(tensör([2.0])), f(tensör([4.0])), f(tensör([5.0, 6.0])), f(tensör([7.0, 8.0])), f(tensör([9.0, 1.0]))]
    yazdır([x.değer döngü x içinde s])
}`)
	want := "[[3.0], [6.0], [12.0], [15.0, 18.0], [21.0, 24.0], [27.0, 3.0]]\n"
	if out != want {
		t.Fatalf("çıktı:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural: tekrar_oynat yalnızca türevsiz içinde çağrılır (H6440); kayıt sırasında konağa
// okuma kodlu hatadır (H4065) ve kayıt bırakılır, sonraki çağrılar çalışır.
func TestReplayErrors(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	_, err := run(t, `varsayılan_cihaz("gpu")
değişken f = tekrar_oynat((x) => x * 2)
f(tensör([1.0]))`)
	if !messages.Has(err, messages.H6440) {
		t.Fatalf("türevsiz dışında H6440 bekleniyordu: %v", err)
	}
	out, err := run(t, `varsayılan_cihaz("gpu")
fonksiyon g(x) {
    yazdır(toplam(x).değer)
    sonuç x * 2
}
değişken f = tekrar_oynat(g)
türevsiz {
    f(tensör([1.0]))
    dene { f(tensör([2.0])) } yakala h { yazdır("hata:", h) }
    yazdır(f(tensör([3.0])).değer)
}`)
	if err != nil || !strings.Contains(out, "GPU kaydı sürerken cihazdan konağa okunamaz") || !strings.HasSuffix(out, "[6.0]\n") {
		t.Fatalf("kayıtta konağa okuma hatası ve sonra çalışma bekleniyordu:\n%s\n%v", out, err)
	}
}

// Kural (yerine_yaz): kayıt dışında başlangıç konakta denetlenir: dilim sığmalı (H6442),
// başlangıç tam sayı ya da tam sayı değerli 0 boyutlu tensör olmalı (H6441); türev izlenen
// hedefe yalnızca türevsiz içinde yazılır (H6716).
func TestWriteSliceChecks(t *testing.T) {
	out := mustRun(t, `değişken t = sıfırlar([2, 4])
yerine_yaz(t, birler([2, 2]), eksen: 1, başlangıç: tensör(2.0))
yazdır(t.değer)`)
	if out != "[[0.0, 0.0, 1.0, 1.0], [0.0, 0.0, 1.0, 1.0]]\n" {
		t.Fatalf("yerine_yaz: %q", out)
	}
	for code, src := range map[messages.Code]string{
		messages.H6442: `yerine_yaz(sıfırlar([4]), birler([2]), eksen: 0, başlangıç: 3)`,
		messages.H6441: `yerine_yaz(sıfırlar([4]), birler([2]), eksen: 0, başlangıç: 1.5)`,
		messages.H6716: `yerine_yaz(sıfırlar([4], türev_izle: doğru), birler([2]), eksen: 0, başlangıç: 0)`,
	} {
		if _, err := run(t, src); !messages.Has(err, code) {
			t.Errorf("%s: %s bekleniyordu, %v", src, code, err)
		}
	}
}
