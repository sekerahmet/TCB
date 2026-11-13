package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (23.5, 15.41): TR_rfft ve TR_stft kartta da çalışır: sonuç ve türev kartta kalır, işlemciyle
// 15.41'in 3. basamağında aynıdır (Whisper'ın log-Mel biçimi, `lib/audio.tr:304`). `TR_vendor_matmul`
// bloğu dönüşümün çarpımını kitaplığa vermez: blokta da bitler bloksuz koşununkiyle aynı. Kartta
// uzunluk en çok 4096'dır: üstü H4134, ileti sınırı ve verilen n'yi söyler, önerisi
// (`x.TR_to_device("cpu")`) kopyalanınca CPU'da çalışır. Kartta float64 H7076 verir, sinyal ile
// pencere farklı cihazdaysa H6102; ikisi de CPU'ya sessizce düşmez.
func TestFourierRunsOnTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_func spektrum(cihaz: TR_String) -> TR_List[TR_Tensor] {
    TR_var x = TR_Tensor(TR_Tensor([[0.0, 1.0, 0.5, -1.0, 2.0, 0.25, -0.5, 1.5, 0.75, -2.0, 1.0, 0.0]]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var w = TR_hann_window(8, TR_periodic: TR_true).TR_to_device(cihaz)
    TR_var g, s = TR_rfft(x, TR_n: 16)
    TR_var sg, ss = TR_stft(x, 8, TR_hop_length: 2, TR_window: w, TR_center: TR_true, TR_pad_mode: "reflect")
    TR_backward(TR_sum(g * g) + TR_sum(s) + TR_sum(sg * sg) + TR_sum(ss))
    TR_return [g, s, sg, ss, x.TR_grad]
}
TR_var c = spektrum("cpu")
TR_var k = spektrum("gpu")
TR_var hepsi = TR_true
TR_for i TR_in 0..5 {
    hepsi = hepsi TR_and TR_all_close(c[i], k[i].TR_to_device("cpu"), TR_atol: 1e-4)
}
TR_print(k[0].TR_device, k[2].TR_device, k[4].TR_device, hepsi)`)
	if want := "gpu:0 gpu:0 gpu:0 true\n"; out != want {
		t.Fatalf("kartta Fourier: %q, beklenen %q", out, want)
	}
	block := mustRun(t, `TR_var x = TR_Tensor([[0.0, 1.0, 0.5, -1.0, 2.0, 0.25, -0.5, 1.5]]).TR_to_device("gpu")
TR_var w = TR_hann_window(4, TR_periodic: TR_true).TR_to_device("gpu")
TR_var a, b = TR_stft(x, 4, TR_hop_length: 1, TR_window: w, TR_center: TR_false)
TR_var c, d = TR_vendor_matmul { TR_stft(x, 4, TR_hop_length: 1, TR_window: w, TR_center: TR_false) }
TR_var e, f = TR_rfft(x)
TR_var g, h = TR_vendor_matmul { TR_rfft(x) }
TR_print(a.TR_tolist == c.TR_tolist, b.TR_tolist == d.TR_tolist, e.TR_tolist == g.TR_tolist, f.TR_tolist == h.TR_tolist)`)
	if block != "true true true true\n" {
		t.Errorf("blokta Fourier'in bitleri bloksuzunkinden farklı: %q", block)
	}
	_, err := run(t, `TR_var x = TR_ones([4097]).TR_to_device("gpu")
TR_rfft(x)`)
	if !messages.Has(err, messages.H4134) || !strings.Contains(err.Error(), "4096") || !strings.Contains(err.Error(), "4097") ||
		!strings.Contains(err.Error(), `x.TR_to_device("cpu")`) {
		t.Fatalf("n = 4097 kartta: %v, beklenen H4134 (sınır, verilen n, öneri)", err)
	}
	if out := mustRun(t, `TR_var x = TR_ones([4097]).TR_to_device("gpu")
TR_var g, s = TR_rfft(x.TR_to_device("cpu"))
TR_print(g.TR_device, g.TR_shape)`); out != "cpu [2049]\n" {
		t.Errorf("H4134'ün önerisi: %q", out)
	}
	expectCodes(t, []codeCase{
		{`TR_rfft(TR_Tensor([1.0, 2.0], TR_dtype: "float64").TR_to_device("gpu"))`, messages.H7076},
		{`TR_stft(TR_ones([16]).TR_to_device("gpu"), 4, TR_hop_length: 1, TR_window: TR_ones([4]), TR_center: TR_false)`, messages.H6102},
	})
}

// Kural (15.45, `15_tensorler.md:559`, `:564`): `TR_replay`'in ilk çağrısı `f`'yi doğrudan
// çalıştırır, ikincisi kaydeder; kayıtta konaktan yeni veri kodlu hatadır, sessiz yükleme yoktur.
// (i) Kartta `TR_rfft`: tabanı ısınma çağrısı kurar (önbellek boşken, kayıt dışında), kayıt
// onu önbellekten alır; üç çağrıda sonuç doğrudan çağrıyla bit bit aynı. n = 91 paketin başka
// testinde geçmez: taban önbelleği süreç boyudur, başka test ısıtmış olmasın.
// (ii) Kartta `TR_stft`: çerçevelerin konum tablosu her çağrıda konaktan yüklenir
// (`fourier_card.go`'nun `stftOnCard`'ı); ısınma doğrudan çağrıyla aynı, kayıt H4066. Bugünkü
// davranışın kilidi (T1 dilim 5'in B6'sı, status 15.41): tablo kartta önbelleğe alınınca (ii)
// "doğrudan çağrıyla aynı"ya döner.
func TestReplayFourierOnTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_default_device("gpu")
TR_func f(x: TR_Tensor) -> TR_List[TR_Tensor] {
    TR_var g, s = TR_rfft(x, TR_n: 91)
    TR_return [g, s]
}
TR_var oy = TR_replay(f)
TR_no_grad {
    TR_for i TR_in 0..3 {
        TR_var x = TR_randn([2, 85], TR_seed: i)
        TR_var r = oy(x)
        TR_var d = f(x)
        TR_print(d[0].TR_tolist == r[0].TR_tolist TR_and d[1].TR_tolist == r[1].TR_tolist, r[0].TR_device)
    }
}`)
	if want := "true gpu:0\ntrue gpu:0\ntrue gpu:0\n"; out != want {
		t.Fatalf("kayıtta kartın TR_rfft'i: %q, beklenen %q", out, want)
	}
	out = mustRun(t, `TR_default_device("gpu")
TR_var w = TR_hann_window(12, TR_periodic: TR_true)
TR_func g(x: TR_Tensor) -> TR_List[TR_Tensor] {
    TR_var a, b = TR_stft(x, 12, TR_hop_length: 5, TR_window: w, TR_center: TR_true, TR_pad_mode: "reflect")
    TR_return [a, b]
}
TR_var oy = TR_replay(g)
TR_no_grad {
    TR_var x = TR_randn([2, 61], TR_seed: 0)
    TR_var r = oy(x)
    TR_var d = g(x)
    TR_print(d[0].TR_tolist == r[0].TR_tolist TR_and d[1].TR_tolist == r[1].TR_tolist, r[0].TR_device)
    TR_print(TR_error_code { oy(TR_randn([2, 61], TR_seed: 1)) })
}`)
	if want := "true gpu:0\nH4066\n"; out != want {
		t.Fatalf("kayıtta kartın TR_stft'i: %q, beklenen %q (ısınma aynı, kayıt H4066)", out, want)
	}
}
