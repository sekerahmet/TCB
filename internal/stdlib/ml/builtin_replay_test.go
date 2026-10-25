package ml

import (
	"bytes"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
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

// Kural (dil tanımı 15.45): tekrar_oynat'ın sonucu (ısınma, kayıt ve oynatma çağrıları)
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
	// Bloğun adı metne gömülü değildir, ad kodundan gelir (yazım diline göre değişir).
	if e, _ := messages.As(err); e.Args[1] != codes.NameRef(codes.FnNoGrad) {
		t.Errorf("H6440 bloğun adını ad kodundan almalı: %v", e.Args)
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

// Kural: ulaşılamaz tekrar_oynat fonksiyonunun kaydı (çizge, kayda özel bellek, girdi ve çıktı
// tensörleri) bırakılır; sonraki tekrar_oynat çağrısı kuyruktakileri bırakır. Bırakılmasaydı her
// üret çağrısının kaydı süreç boyunca kalırdı.
func TestReplayReleasesUnreachableRecordings(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	// Program sürerken çöp toplayıcı koşarsa önceki f'in kaydı sonraki tekrar_oynat çağrısında
	// (programın içinde) bırakılır: sayaç programdan önce okunur, bekleme koşulu bırakılanları da
	// sayar.
	before := releasedRecordings.Load()
	mustRun(t, `varsayılan_cihaz("gpu")
türevsiz {
    döngü i içinde 0..3 {
        değişken f = tekrar_oynat((x) => x * 2)
        döngü j içinde 0..3 { f(tensör([1.0, 2.0])) }
    }
}`)
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		releaseQueue.mu.Lock()
		n := int64(len(releaseQueue.entries))
		releaseQueue.mu.Unlock()
		if releasedRecordings.Load()-before+n >= 3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	drainReleases()
	if got := releasedRecordings.Load() - before; got < 3 {
		t.Fatalf("3 kayıt bırakılmalıydı, %d bırakıldı", got)
	}
}

// Kural: cihazda kayıt sürerken çağrılan tekrar_oynat fonksiyonu doğrudan çağrılır; işi dıştaki
// kayda girer (iç içe tekrar_oynat; JAX'ta iç içe jit gibi). Gemini kayıt denetimi, bulgu 2A.
func TestReplayNestedRunsInsideOuterRecording(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
değişken iç = tekrar_oynat((x) => x + 1)
değişken dış = tekrar_oynat((x) => iç(x) * 2)
türevsiz {
    yazdır([dış(tensör([3.0])).değer, dış(tensör([4.0])).değer, dış(tensör([5.0])).değer])
}`)
	if want := "[[8.0], [10.0], [12.0]]\n"; out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
}

// Kural: kayıt sürerken bırakılmayı bekleyen kayıtlar kuyrukta kalır (bırakmak eşzamanlama
// ister); kayıt bitince bırakılır (bulgu 2B).
func TestReplayKeepsQueuedReleasesDuringCapture(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	drainReleases()
	e := recordedEntry(t)
	releaseQueue.mu.Lock()
	releaseQueue.entries = append(releaseQueue.entries, e)
	releaseQueue.mu.Unlock()
	// Kuyruk paketin tektir: önceki testlerin ulaşılamaz kalan tekrar_oynat'larının temizliği
	// (runtime.AddCleanup) çöp toplayıcı koştuğunda her an kayıt ekleyebilir. Bu yüzden kuyruğun
	// uzunluğuna ya da bırakma sayısına değil, bu kaydın kendisine bakılır.
	queued := func() bool {
		releaseQueue.mu.Lock()
		defer releaseQueue.mu.Unlock()
		return slices.Contains(releaseQueue.entries, e)
	}
	rec := tensor.BackendFor(tensor.GPU(0)).(tensor.Recorder)
	rec.BeginCapture()
	drainReleases()
	rec.AbortCapture()
	if !queued() || e.rec == nil {
		t.Fatalf("kayıt sürerken kuyruktaki kayıt bırakıldı (kuyrukta: %v)", queued())
	}
	drainReleases()
	if queued() || e.rec != nil {
		t.Fatalf("kayıt bitince kuyruktaki kayıt bırakılmadı")
	}
}

// recordedEntry kaydı yapılmış bir girdi bileşimi kurar (f'i tekrar_oynat gibi üç kez çağırır:
// ısınma, kayıt, oynatma).
func recordedEntry(t *testing.T) *replayEntry {
	t.Helper()
	m := interp.New()
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(parser.New(lexer.New(`varsayılan_cihaz("gpu")
fonksiyon f(x) { sonuç x * 2 + 1 }`)).ParseProgram()); err != nil {
		t.Fatal(err)
	}
	f, _ := m.GlobalEnv.Get("f")
	r := &replayFn{f: f, cache: map[string]*replayEntry{}}
	x := New(tensor.NewOn(tensor.GPU(0), []float32{1, 2}, 2))
	tensor.NoGrad(func() {
		for i := 0; i < 3; i++ {
			if _, err := r.call(m, []interp.Value{x}, nil); err != nil {
				t.Fatal(err)
			}
		}
	})
	for _, e := range r.cache {
		if e.rec != nil {
			return e
		}
	}
	t.Fatal("kayıt yok")
	return nil
}

// Kural: kayıt bırakılınca girdi ve çıktı başlıkları düşer (her konumun sarıcısı bir kez
// bırakılır; fazladan sarıcı belirlenimci bırakmayı önlerdi). Bulgu 3.
func TestReplayReleaseDropsHeaders(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	e := recordedEntry(t)
	in, out := e.inputs[0], e.outs[0]
	e.release()
	if !in.IsDropped() || !out.IsDropped() {
		t.Fatalf("girdi düştü: %v, çıktı düştü: %v", in.IsDropped(), out.IsDropped())
	}
}

// Kural: kayıt sırasında yorumlayıcı kilidi başka göreve geçmez: başka bir eşzamanlı görevin
// işi kayda girmez; kayıtta görevleri beklemek (bekle_hepsi) H2522'dir. Isınma bitmeden bileşim
// ısınmış sayılmaz (bulgu 4).
func TestReplayCaptureHoldsTheLock(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
değişken yan = sıfırlar([4])
değişken d = {"çağrı": 0}
değişken f = tekrar_oynat(fonksiyon(x) {
    d["çağrı"] += 1
    eğer d["çağrı"] == 2 {
        eşzamanlı { türevsiz { yerine_yaz(yan, birler([4]), eksen: 0) } }
        döngü i içinde 0..300 { d["boş"] = i } // kilit devredilseydi görev burada koşardı
    }
    sonuç x * 2
})
türevsiz {
    değişken x = tensör([3.0, 4.0])
    f(x)
    f(x)
    bekle_hepsi()
    yerine_yaz(yan, sıfırlar([4]), eksen: 0)
    f(x)
    yazdır(yan.değer)
}`)
	if want := "[0.0, 0.0, 0.0, 0.0]\n"; out != want {
		t.Fatalf("başka görevin işi kayda girdi: yan %q, beklenen %q", out, want)
	}
	_, err := run(t, `varsayılan_cihaz("gpu")
değişken f = tekrar_oynat(fonksiyon(x) {
    bekle_hepsi()
    sonuç x * 2
})
türevsiz {
    f(tensör([1.0]))
    f(tensör([1.0]))
}`)
	if !messages.Has(err, messages.H2522) {
		t.Fatalf("kayıtta bekle_hepsi H2522 vermeliydi: %v", err)
	}
}

// Kural: f'in yerinde değiştirdiği girdi her oynatmadan sonra çağıranın tensörüne geri
// yazılır; aynı tensör iki argümanda verilirse kayıtta da tek tensördür; tensörler farklı
// cihazlardaysa f doğrudan çağrılır (bulgu 6).
func TestReplayArgumentsKeepDirectCallMeaning(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
fonksiyon adım(ön, yeni, konum) {
    yerine_yaz(ön, yeni, eksen: 0, başlangıç: konum)
    sonuç ön * 1
}
değişken oynat = tekrar_oynat(adım)
değişken ö1 = sıfırlar([3])
değişken ö2 = sıfırlar([3])
değişken dokuz = tensör([9.0])
değişken g = tekrar_oynat(fonksiyon(a, b) {
    yerine_yaz(a, dokuz, eksen: 0)
    sonuç b * 2
})
değişken h = tekrar_oynat((x, c) => [x * 2, c * 3])
türevsiz {
    döngü i içinde 0..3 {
        değişken v = tensör([(i + 1) * 10.0])
        değişken k = tensör(i * 1.0)
        oynat(ö1, v, k)
        adım(ö2, v, k)
    }
    yazdır(ö1.değer == ö2.değer)
    değişken s = []
    döngü i içinde 0..3 {
        değişken x = sıfırlar([2])
        s.ekle(g(x, x).değer)
    }
    yazdır(s)
    yazdır([h(tensör([1.0]), tensör([c * 1.0]).cihaza("cpu"))[1].değer döngü c içinde [10, 20, 100]])
}`)
	want := "doğru\n[[18.0, 0.0], [18.0, 0.0], [18.0, 0.0]]\n[[30.0], [60.0], [300.0]]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural (yerine_yaz): kaynak hedefin depolamasındaysa (örtüşen dilim) kayıtta da önce
// kopyalanır (bulgu 7A). Bitişik olmayan hedef (H6443), eksen sayısı ya da öbür eksenleri
// farklı kaynak (H3111, yayınlama yok) ve hedeften farklı cihazdaki başlangıç (H6102) kayıt
// yolundan önce denetlenir: kayıt içinde ve dışında aynı hatadır (bulgu 7B–D, 8A). gömme'nin
// indeks tensörü de tabloyla aynı cihazda olmalıdır.
func TestWriteSliceSameRulesInsideAndOutsideRecording(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
fonksiyon kur() { sonuç tensör([i * 1.0 döngü i içinde 0..64]) }
değişken a = kur()
değişken b = kur()
türevsiz { yerine_yaz(a, a.dilim(0, 0, 48), eksen: 0, başlangıç: tensör(16.0)) }
değişken f = tekrar_oynat(fonksiyon(konum) {
    yerine_yaz(b, b.dilim(0, 0, 48), eksen: 0, başlangıç: konum)
    sonuç b * 1
})
türevsiz {
    f(tensör(0.0))
    yerine_yaz(b, kur(), eksen: 0)
    f(tensör(16.0))
}
yazdır(a.değer == b.değer, b.değer[48])`)
	if out != "doğru 32.0\n" {
		t.Fatalf("örtüşen dilim kayıtta bozuldu: %q", out)
	}
	for code, call := range map[messages.Code]string{
		messages.H6443: `yerine_yaz(sıfırlar([3, 4]).devrik(), birler([4, 1]), eksen: 1, başlangıç: tensör(0.0))`,
		messages.H3111: `yerine_yaz(sıfırlar([2, 4]), birler([1, 2]), eksen: 1, başlangıç: tensör(0.0))`,
		messages.H6102: `yerine_yaz(sıfırlar([4]), birler([2]), eksen: 0, başlangıç: tensör(0.0).cihaza("cpu"))`,
	} {
		if _, err := run(t, `varsayılan_cihaz("gpu")
türevsiz { `+call+` }`); !messages.Has(err, code) {
			t.Errorf("%s: %s bekleniyordu, %v", call, code, err)
		}
	}
	if _, err := run(t, `yerine_yaz(sıfırlar([4]), 5.0, eksen: 0)`); !messages.Has(err, messages.H3111) {
		t.Errorf("0 boyutlu kaynak H3111 vermeliydi: %v", err)
	}
	if _, err := run(t, `varsayılan_cihaz("gpu")
gömme(birler([4, 2]), tensör([1.0]).cihaza("cpu"))`); !messages.Has(err, messages.H6102) {
		t.Errorf("gömme farklı cihazdaki indekste H6102 vermeliydi: %v", err)
	}
}

// Kural: sayı işlenenli işlemler (koşullu, ortalama listesi, eksensiz min, arktanjant2, sayıyla
// karşılaştırma ve bölme) kayıtta da çalışır: sayı cihazda doldurulur, konaktan yüklenmez
// (bulgu 8B).
func TestReplayScalarOperands(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, expr := range []string{`koşullu(x > 0, x, 0.0)`, `ortalama([x, x * 2])`, `min(x)`, `arktanjant2(x, 1.0)`, `x / 3`} {
		out, err := run(t, `varsayılan_cihaz("gpu")
değişken f = tekrar_oynat((x) => `+expr+`)
türevsiz {
    değişken x = tensör([1.0, -2.0, 3.0, 4.0]) + 1
    değişken s = [f(x).değer döngü i içinde 0..3]
    yazdır(s[0] == s[2], s[2] == (`+expr+`).değer)
}`)
		if err != nil || out != "doğru doğru\n" {
			t.Errorf("%s: %q %v", expr, out, err)
		}
	}
}

// Kural: görünümün depolamadaki başlangıcı maske ofset tablosuna girmez: ofsetli bir maske
// görünümüyle (ısınmada argümanın kendisi, kayıtta kopyası) tablo aynıdır, kayıt yükleme
// yapmaz (Gemini sayısal denetimi, bulgu 3). PLE'li GPT'nin üretimi kayıtla da kayıtsızla aynı
// parçaları verir (bulgu 2).
func TestReplayAttentionMaskViewsAndPLEGeneration(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `varsayılan_cihaz("gpu")
değişken q = birler([1, 1, 1, 4])
değişken k = birler([1, 4, 1, 4])
değişken v = tensör([[[[1.0, 2.0, 3.0, 4.0]], [[5.0, 6.0, 7.0, 8.0]], [[1.0, 1.0, 1.0, 1.0]], [[2.0, 2.0, 2.0, 2.0]]]])
değişken tampon = tensör([[1.0, 0.0, 0.0, 0.0], [1.0, 1.0, 0.0, 0.0]])
değişken f = tekrar_oynat((m) => dikkat(q, k, v, nedensel: yanlış, maske: m))
türevsiz {
    değişken s = [f(tampon[1]).değer döngü i içinde 0..3]
    yazdır(s[0] == s[2], s[2] == dikkat(q, k, v, nedensel: yanlış, maske: tampon[1]).değer)
}`)
	if out != "doğru doğru\n" {
		t.Fatalf("ofsetli maske: %q", out)
	}
	out = mustRun(t, `aktar "../../../lib/katman.tr"
varsayılan_cihaz("gpu")
değişken m = GPT(12, bağlam: 6, boyut: 8, katman: 2, bakış_sayısı: 2, ple_boyutu: 4, tohum: 5)
yazdır(m.üret([1, 2], 4, sıcaklık: 0, çizge: doğru) == m.üret([1, 2], 4, sıcaklık: 0, çizge: yanlış))`)
	if out != "doğru\n" {
		t.Fatalf("PLE'li üretim kayıtla farklı: %q", out)
	}
}
