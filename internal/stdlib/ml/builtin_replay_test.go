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
TR_var w = TR_randn([6, 4], TR_seed: 1)
TR_func kur() { TR_return TR_zeros([1, 5, 4]) }
TR_func adım(ön, id, konum) {
    TR_var e = TR_embedding(w, id)
    TR_write_slice(ön, e.TR_reshape([1, 1, 4]), TR_dim: 1, TR_start: konum)
    TR_return e * 2 + 1
}
TR_func sür(f, ön) {
    TR_var sonuçlar = []
    TR_no_grad {
        TR_for i TR_in 0..5 {
            sonuçlar.TR_append(f(TR_tensor([(i * 2) % 6 * 1.0]), TR_tensor(i * 1.0)).TR_tolist)
        }
    }
    TR_return [sonuçlar, ön.TR_tolist]
}
`

// Kural (dil tanımı 15.45): tekrar_oynat'ın sonucu (ısınma, kayıt ve oynatma çağrıları)
// doğrudan çağrıyla aynıdır; kayıtta tensörle verilen konum ve indeks her oynatmada
// cihazdan okunur. CPU'da f doğrudan çağrılır.
func TestReplayMatchesDirectCalls(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, dev := range []string{"cpu", "gpu"} {
		out := mustRun(t, `TR_default_device("`+dev+`")`+replayStep+`
TR_var ö1 = kur()
TR_var ö2 = kur()
TR_var oynatılan = TR_replay((id, konum) => adım(ö1, id, konum))
TR_var a = sür(oynatılan, ö1)
TR_var b = sür((id, konum) => adım(ö2, id, konum), ö2)
TR_print(a == b)
TR_print(a[0][4])`)
		lines := strings.Split(out, "\n")
		if lines[0] != "true" {
			t.Fatalf("%s: tekrar_oynat doğrudan çağrıdan farklı:\n%s", dev, out)
		}
	}
}

// Kural: kaydın çıktısı kopyalanarak verilir; sonraki oynatma önceki çağrının sonucunu
// değiştirmez. Girdinin şekli değişince yeniden kaydedilir.
func TestReplayOutputsAreCopiesAndShapesRecapture(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_default_device("gpu")
TR_var f = TR_replay((x) => x * 3)
TR_no_grad {
    TR_var s = [f(TR_tensor([1.0])), f(TR_tensor([2.0])), f(TR_tensor([4.0])), f(TR_tensor([5.0, 6.0])), f(TR_tensor([7.0, 8.0])), f(TR_tensor([9.0, 1.0]))]
    TR_print([x.TR_tolist TR_for x TR_in s])
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
	_, err := run(t, `TR_default_device("gpu")
TR_var f = TR_replay((x) => x * 2)
f(TR_tensor([1.0]))`)
	if !messages.Has(err, messages.H6440) {
		t.Fatalf("türevsiz dışında H6440 bekleniyordu: %v", err)
	}
	// Bloğun adı metne gömülü değildir, ad kodundan gelir (yazım diline göre değişir).
	if e, _ := messages.As(err); e.Args[1] != codes.NameRef(codes.FnNoGrad) {
		t.Errorf("H6440 bloğun adını ad kodundan almalı: %v", e.Args)
	}
	out, err := run(t, `TR_default_device("gpu")
TR_func g(x) {
    TR_print(TR_sum(x).TR_tolist)
    TR_return x * 2
}
TR_var f = TR_replay(g)
TR_no_grad {
    f(TR_tensor([1.0]))
    TR_try { f(TR_tensor([2.0])) } TR_catch h { TR_print("hata:", h) }
    TR_print(f(TR_tensor([3.0])).TR_tolist)
}`)
	if err != nil || !strings.Contains(out, "while a GPU recording is in progress, data cannot be read") || !strings.HasSuffix(out, "[6.0]\n") {
		t.Fatalf("kayıtta konağa okuma hatası ve sonra çalışma bekleniyordu:\n%s\n%v", out, err)
	}
}

// Kural (yerine_yaz): kayıt dışında başlangıç konakta denetlenir: dilim sığmalı (H6442),
// başlangıç tam sayı ya da tam sayı değerli 0 boyutlu tensör olmalı (H6441); türev izlenen
// hedefe yalnızca türevsiz içinde yazılır (H6716).
func TestWriteSliceChecks(t *testing.T) {
	out := mustRun(t, `TR_var t = TR_zeros([2, 4])
TR_write_slice(t, TR_ones([2, 2]), TR_dim: 1, TR_start: TR_tensor(2.0))
TR_print(t.TR_tolist)`)
	if out != "[[0.0, 0.0, 1.0, 1.0], [0.0, 0.0, 1.0, 1.0]]\n" {
		t.Fatalf("yerine_yaz: %q", out)
	}
	for code, src := range map[messages.Code]string{
		messages.H6442: `TR_write_slice(TR_zeros([4]), TR_ones([2]), TR_dim: 0, TR_start: 3)`,
		messages.H6441: `TR_write_slice(TR_zeros([4]), TR_ones([2]), TR_dim: 0, TR_start: 1.5)`,
		messages.H6716: `TR_write_slice(TR_zeros([4], TR_requires_grad: TR_true), TR_ones([2]), TR_dim: 0, TR_start: 0)`,
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
	mustRun(t, `TR_default_device("gpu")
TR_no_grad {
    TR_for i TR_in 0..3 {
        TR_var f = TR_replay((x) => x * 2)
        TR_for j TR_in 0..3 { f(TR_tensor([1.0, 2.0])) }
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
	out := mustRun(t, `TR_default_device("gpu")
TR_var iç = TR_replay((x) => x + 1)
TR_var dış = TR_replay((x) => iç(x) * 2)
TR_no_grad {
    TR_print([dış(TR_tensor([3.0])).TR_tolist, dış(TR_tensor([4.0])).TR_tolist, dış(TR_tensor([5.0])).TR_tolist])
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
	if _, err := m.Run(parser.New(lexer.NewBody(`TR_default_device("gpu")
TR_func f(x) { TR_return x * 2 + 1 }`, true)).ParseProgram()); err != nil {
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
	out := mustRun(t, `TR_default_device("gpu")
TR_var yan = TR_zeros([4])
TR_var d = {"çağrı": 0}
TR_var f = TR_replay(TR_func(x) {
    d["çağrı"] += 1
    TR_if d["çağrı"] == 2 {
        TR_async { TR_no_grad { TR_write_slice(yan, TR_ones([4]), TR_dim: 0) } }
        TR_for i TR_in 0..300 { d["boş"] = i } # kilit devredilseydi görev burada koşardı
    }
    TR_return x * 2
})
TR_no_grad {
    TR_var x = TR_tensor([3.0, 4.0])
    f(x)
    f(x)
    TR_wait_all()
    TR_write_slice(yan, TR_zeros([4]), TR_dim: 0)
    f(x)
    TR_print(yan.TR_tolist)
}`)
	if want := "[0.0, 0.0, 0.0, 0.0]\n"; out != want {
		t.Fatalf("başka görevin işi kayda girdi: yan %q, beklenen %q", out, want)
	}
	_, err := run(t, `TR_default_device("gpu")
TR_var f = TR_replay(TR_func(x) {
    TR_wait_all()
    TR_return x * 2
})
TR_no_grad {
    f(TR_tensor([1.0]))
    f(TR_tensor([1.0]))
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
	out := mustRun(t, `TR_default_device("gpu")
TR_func adım(ön, yeni, konum) {
    TR_write_slice(ön, yeni, TR_dim: 0, TR_start: konum)
    TR_return ön * 1
}
TR_var oynat = TR_replay(adım)
TR_var ö1 = TR_zeros([3])
TR_var ö2 = TR_zeros([3])
TR_var dokuz = TR_tensor([9.0])
TR_var g = TR_replay(TR_func(a, b) {
    TR_write_slice(a, dokuz, TR_dim: 0)
    TR_return b * 2
})
TR_var h = TR_replay((x, c) => [x * 2, c * 3])
TR_no_grad {
    TR_for i TR_in 0..3 {
        TR_var v = TR_tensor([(i + 1) * 10.0])
        TR_var k = TR_tensor(i * 1.0)
        oynat(ö1, v, k)
        adım(ö2, v, k)
    }
    TR_print(ö1.TR_tolist == ö2.TR_tolist)
    TR_var s = []
    TR_for i TR_in 0..3 {
        TR_var x = TR_zeros([2])
        s.TR_append(g(x, x).TR_tolist)
    }
    TR_print(s)
    TR_print([h(TR_tensor([1.0]), TR_tensor([c * 1.0]).TR_to_device("cpu"))[1].TR_tolist TR_for c TR_in [10, 20, 100]])
}`)
	want := "true\n[[18.0, 0.0], [18.0, 0.0], [18.0, 0.0]]\n[[30.0], [60.0], [300.0]]\n"
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
	out := mustRun(t, `TR_default_device("gpu")
TR_func kur() { TR_return TR_tensor([i * 1.0 TR_for i TR_in 0..64]) }
TR_var a = kur()
TR_var b = kur()
TR_no_grad { TR_write_slice(a, a.TR_slice(0, 0, 48), TR_dim: 0, TR_start: TR_tensor(16.0)) }
TR_var f = TR_replay(TR_func(konum) {
    TR_write_slice(b, b.TR_slice(0, 0, 48), TR_dim: 0, TR_start: konum)
    TR_return b * 1
})
TR_no_grad {
    f(TR_tensor(0.0))
    TR_write_slice(b, kur(), TR_dim: 0)
    f(TR_tensor(16.0))
}
TR_print(a.TR_tolist == b.TR_tolist, b.TR_tolist[48])`)
	if out != "true 32.0\n" {
		t.Fatalf("örtüşen dilim kayıtta bozuldu: %q", out)
	}
	for code, call := range map[messages.Code]string{
		messages.H6443: `TR_write_slice(TR_zeros([3, 4]).TR_transpose(), TR_ones([4, 1]), TR_dim: 1, TR_start: TR_tensor(0.0))`,
		messages.H3111: `TR_write_slice(TR_zeros([2, 4]), TR_ones([1, 2]), TR_dim: 1, TR_start: TR_tensor(0.0))`,
		messages.H6102: `TR_write_slice(TR_zeros([4]), TR_ones([2]), TR_dim: 0, TR_start: TR_tensor(0.0).TR_to_device("cpu"))`,
	} {
		if _, err := run(t, `TR_default_device("gpu")
TR_no_grad { `+call+` }`); !messages.Has(err, code) {
			t.Errorf("%s: %s bekleniyordu, %v", call, code, err)
		}
	}
	if _, err := run(t, `TR_write_slice(TR_zeros([4]), 5.0, TR_dim: 0)`); !messages.Has(err, messages.H3111) {
		t.Errorf("0 boyutlu kaynak H3111 vermeliydi: %v", err)
	}
	if _, err := run(t, `TR_default_device("gpu")
TR_embedding(TR_ones([4, 2]), TR_tensor([1.0]).TR_to_device("cpu"))`); !messages.Has(err, messages.H6102) {
		t.Errorf("gömme farklı cihazdaki indekste H6102 vermeliydi: %v", err)
	}
}

// Kural: sayı işlenenli işlemler (koşullu, ortalama listesi, eksensiz min, atan2, sayıyla
// karşılaştırma ve bölme) kayıtta da çalışır: sayı cihazda doldurulur, konaktan yüklenmez
// (bulgu 8B).
func TestReplayScalarOperands(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, expr := range []string{`TR_where(x > 0, x, 0.0)`, `TR_mean([x, x * 2])`, `TR_min(x)`, `TR_atan2(x, 1.0)`, `x / 3`} {
		out, err := run(t, `TR_default_device("gpu")
TR_var f = TR_replay((x) => `+expr+`)
TR_no_grad {
    TR_var x = TR_tensor([1.0, -2.0, 3.0, 4.0]) + 1
    TR_var s = [f(x).TR_tolist TR_for i TR_in 0..3]
    TR_print(s[0] == s[2], s[2] == (`+expr+`).TR_tolist)
}`)
		if err != nil || out != "true true\n" {
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
	out := mustRun(t, `TR_default_device("gpu")
TR_var q = TR_ones([1, 1, 1, 4])
TR_var k = TR_ones([1, 4, 1, 4])
TR_var v = TR_tensor([[[[1.0, 2.0, 3.0, 4.0]], [[5.0, 6.0, 7.0, 8.0]], [[1.0, 1.0, 1.0, 1.0]], [[2.0, 2.0, 2.0, 2.0]]]])
TR_var tampon = TR_tensor([[1.0, 0.0, 0.0, 0.0], [1.0, 1.0, 0.0, 0.0]])
TR_var f = TR_replay((m) => TR_attention(q, k, v, TR_causal: TR_false, TR_mask: m))
TR_no_grad {
    TR_var s = [f(tampon[1]).TR_tolist TR_for i TR_in 0..3]
    TR_print(s[0] == s[2], s[2] == TR_attention(q, k, v, TR_causal: TR_false, TR_mask: tampon[1]).TR_tolist)
}`)
	if out != "true true\n" {
		t.Fatalf("ofsetli maske: %q", out)
	}
	out = mustRun(t, `TR_import "../../../lib/katman.tr"
TR_default_device("gpu")
TR_var m = TR_GPT(12, TR_context: 6, TR_embed_dim: 8, TR_num_layers: 2, TR_num_heads: 2, TR_ple_dim: 4, TR_seed: 5)
TR_print(m.TR_generate([1, 2], 4, TR_temperature: 0, TR_cuda_graph: TR_true) == m.TR_generate([1, 2], 4, TR_temperature: 0, TR_cuda_graph: TR_false))`)
	if out != "true\n" {
		t.Fatalf("PLE'li üretim kayıtla farklı: %q", out)
	}
}
