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

// Kural (23.7, 23.8, 15.46, 15.48, 15.49): ters dönüşüm 16 biti float32'ye açar; iki parçadan biri float64 ise
// float64'te hesaplar ve float64 verir. float64 hesapta türev izleyen girdi H6040'tır, sessizce düşmez;
// tam sayı tensörü H7076. Toplu ekseni 0 olan girdi boş sonuç verir. Cihazlar ayrıysa H6102. TR_istft'te
// pencere izgenin hassasiyetindedir: float32 izgede float64 pencere hatadır, float64 izge float32 pencereyi alır.
func TestInverseFourierDtypes(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_var w = TR_Tensor([1.0, 0.5, 0.25], TR_dtype: "float64")
TR_var f = TR_Tensor([1.0, 0.5, 0.25])
TR_var h = TR_Tensor([1.0, 0.5, 0.25], TR_dtype: "bf16")
TR_print(TR_irfft(w, w, TR_n: 4).TR_dtype, TR_irfft(w, f, TR_n: 4).TR_dtype, TR_irfft(f, w, TR_n: 4).TR_dtype, TR_irfft(h, h, TR_n: 4).TR_dtype, TR_irfft(h, f, TR_n: 4).TR_dtype)
TR_print(TR_irfft(h, h, TR_n: 4).TR_tolist == TR_irfft(f, f, TR_n: 4).TR_tolist)
TR_print(TR_irfft(TR_zeros([0, 3]), TR_zeros([0, 3]), TR_n: 5).TR_shape)
TR_var i = TR_Tensor([1.0, 0.5, 0.25], TR_requires_grad: TR_true)
TR_print(TR_no_grad { TR_irfft(i, w, TR_n: 4) }.TR_dtype)
TR_var g16 = TR_Tensor([1.0, 0.5, 0.25], TR_dtype: "bf16", TR_requires_grad: TR_true)
TR_backward(TR_sum(TR_irfft(g16, f, TR_n: 4)))
TR_print(g16.TR_grad.TR_shape)
TR_var p = TR_Tensor([1.0, 1.0, 1.0, 1.0])
TR_var p64 = TR_Tensor([1.0, 1.0, 1.0, 1.0], TR_dtype: "float64")
TR_func kur(x: TR_Tensor, y: TR_Tensor, w: TR_Tensor) -> TR_Tensor {
    TR_return TR_istft(x.TR_reshape([3, 1]), y.TR_reshape([3, 1]), 4, TR_hop_length: 2, TR_window: w, TR_center: TR_false)
}
TR_print(kur(f, f, p).TR_dtype, kur(h, f, p).TR_dtype, kur(w, f, p).TR_dtype, kur(f, w, p64).TR_dtype)
TR_print(TR_istft(TR_zeros([0, 3, 2]), TR_zeros([0, 3, 2]), 4, TR_hop_length: 2, TR_window: p).TR_shape)
TR_print(TR_no_grad { kur(w, w, TR_Tensor([1.0, 1.0, 1.0, 1.0], TR_requires_grad: TR_true)) }.TR_dtype)`)
	if want := "float64 float64 float64 float32 float32\ntrue\n[0, 5]\nfloat64\n[3]\n" +
		"float32 float32 float64 float64\n[0, 2]\nfloat64\n"; out != want {
		t.Fatalf("ters Fourier'in türleri: %q, beklenen %q", out, want)
	}
	expectCodes(t, []codeCase{
		{`TR_irfft(TR_Tensor([1.0, 0.5], TR_requires_grad: TR_true), TR_Tensor([1.0, 0.5], TR_dtype: "float64"), TR_n: 2)`, messages.H6040},
		{`TR_irfft(TR_Tensor([1.0, 0.5], TR_dtype: "float64"), TR_Tensor([1.0, 0.5], TR_requires_grad: TR_true), TR_n: 2)`, messages.H6040},
		{`TR_irfft(TR_Tensor([1, 2]), TR_Tensor([1, 2]), TR_n: 2)`, messages.H7076},
		{`TR_irfft(TR_ones([3]).TR_to_device("gpu"), TR_ones([3]), TR_n: 4)`, messages.H6102},
		{`TR_irfft(TR_Tensor([1.0, 2.0], TR_dtype: "float64").TR_to_device("gpu"), TR_Tensor([1.0, 2.0], TR_dtype: "float64").TR_to_device("gpu"), TR_n: 2)`, messages.H7076},
		{`TR_irfft(TR_ones([2049]).TR_to_device("gpu"), TR_ones([2049]).TR_to_device("gpu"), TR_n: 4097)`, messages.H4134},
		{`TR_istft(TR_ones([3, 2]), TR_ones([3, 2]), 4, TR_hop_length: 2, TR_window: TR_Tensor([1.0, 1.0, 1.0, 1.0], TR_dtype: "float64"))`, messages.H6960},
		{`TR_istft(TR_Tensor([[1.0], [1.0]], TR_dtype: "float64"), TR_Tensor([[1.0], [1.0]], TR_dtype: "float64"), 2, TR_hop_length: 1, TR_window: TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true), TR_center: TR_false)`, messages.H6040},
		{`TR_istft(TR_Tensor([[1]]), TR_Tensor([[1]]), 1, TR_hop_length: 1, TR_window: TR_ones([1]))`, messages.H7076},
		{`TR_istft(TR_ones([3, 2]).TR_to_device("gpu"), TR_ones([3, 2]).TR_to_device("gpu"), 4, TR_hop_length: 2, TR_window: TR_ones([4]))`, messages.H6102},
		{`TR_istft(TR_ones([2049, 2]).TR_to_device("gpu"), TR_ones([2049, 2]).TR_to_device("gpu"), 4097, TR_hop_length: 512, TR_window: TR_ones([4097]).TR_to_device("gpu"))`, messages.H4134},
	})
}

// Kural (23.8): birden çok çerçevede adım n'den büyükse kurulan sinyalde boşluk kaçınılmazdır; H7921 sinyal
// ayrılmadan, TR_length:'ten ve cihazdan önce verilir (dev adımda bellek ve süre gitmez, ham Go hatası
// sızmaz), ilk boşluğun indeksini söyler. Tek çerçevede adım sonucu etkilemez.
func TestIstftHopBeyondWindowFailsBeforeAllocating(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	var cases []codeCase
	for _, hop := range []string{"9", "10000000000000", "9000000000000000000", "9223372036854775807"} {
		for _, rest := range []string{"", ", TR_length: 12", ", TR_length: 9223372036854775807", ", TR_center: TR_false", ", TR_center: TR_false, TR_length: 8"} {
			cases = append(cases, codeCase{`TR_istft(TR_ones([5, 2]), TR_ones([5, 2]), 8, TR_hop_length: ` + hop + `, TR_window: TR_ones([8])` + rest + `)`, messages.H7921})
		}
	}
	cases = append(cases, codeCase{`TR_istft(TR_ones([5, 2]).TR_to_device("gpu"), TR_ones([5, 2]).TR_to_device("gpu"), 8, TR_hop_length: 9, TR_window: TR_ones([8]).TR_to_device("gpu"))`, messages.H7921})
	expectCodes(t, cases)
	for center, index := range map[string]string{"TR_true": " 4.", "TR_false": " 8."} {
		_, err := run(t, `TR_istft(TR_ones([5, 2]), TR_ones([5, 2]), 8, TR_hop_length: 10000000000000, TR_window: TR_ones([8]), TR_center: `+center+`)`)
		if err == nil || !strings.Contains(err.Error(), index) || strings.Contains(err.Error(), "TR_center") {
			t.Errorf("ortalama %s: %v, beklenen ilk boşluk%s örnek, Hann cümlesi yok", center, err, index)
		}
	}
	out := mustRun(t, `TR_print(TR_istft(TR_ones([5, 1]), TR_zeros([5, 1]), 8, TR_hop_length: 9223372036854775807, TR_window: TR_ones([8]), TR_length: 4).TR_shape)`)
	if out != "[4]\n" {
		t.Errorf("tek çerçevede dev adım: %q", out)
	}
}

// Kural (23.5, 23.8): TR_istft kartta TR_stft'in sonucunu kartta geri kurar; işlemciyle aynıdır.
func TestIstftRunsOnTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_func geri(cihaz: TR_String) -> TR_List[TR_Tensor] {
    TR_var x = TR_Tensor(TR_Tensor([[0.0, 1.0, 0.5, -1.0, 2.0, 0.25, -0.5, 1.5, 0.75, -2.0, 1.0, 0.0, 0.5]]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var w = TR_hann_window(8, TR_periodic: TR_true).TR_to_device(cihaz)
    TR_var g, s = TR_stft(x, 8, TR_hop_length: 2, TR_window: w, TR_pad_mode: "reflect")
    TR_var y = TR_istft(g, s, 8, TR_hop_length: 2, TR_window: w, TR_length: 13)
    TR_backward(TR_sum(y ** 3))
    TR_return [y, x.TR_grad, x]
}
TR_var c = geri("cpu")
TR_var k = geri("gpu")
TR_print(k[0].TR_device, k[1].TR_device, TR_all_close(c[2], k[0].TR_to_device("cpu"), TR_atol: 1e-4), TR_all_close(c[1], k[1].TR_to_device("cpu"), TR_atol: 1e-3))
TR_print(TR_istft(TR_zeros([0, 5, 2]).TR_to_device("gpu"), TR_zeros([0, 5, 2]).TR_to_device("gpu"), 8, TR_hop_length: 2, TR_window: TR_ones([8]).TR_to_device("gpu")).TR_shape)
TR_print(TR_irfft(TR_zeros([0, 3]).TR_to_device("gpu"), TR_zeros([0, 3]).TR_to_device("gpu"), TR_n: 5).TR_shape)
TR_var gerçek = TR_Tensor([1.0, -0.5, 0.25]).TR_to_device("gpu")
TR_var temiz = TR_irfft(gerçek, TR_Tensor([0.0, 0.75, 0.0]).TR_to_device("gpu"), TR_n: 4)
TR_var kirli = TR_irfft(gerçek, TR_Tensor([TR_Float("nan"), 0.75, TR_Float("inf")]).TR_to_device("gpu"), TR_n: 4)
TR_print(TR_equal(temiz, kirli))`)
	if want := "gpu:0 gpu:0 true true\n[0, 2]\n[0, 5]\ntrue\n"; out != want {
		t.Fatalf("kartta ters STFT: %q, beklenen %q", out, want)
	}
	expectCodes(t, []codeCase{
		{`TR_istft(TR_ones([5, 4]).TR_to_device("gpu"), TR_ones([5, 4]).TR_to_device("gpu"), 8, TR_hop_length: 4, TR_window: TR_hann_window(8, TR_periodic: TR_true).TR_to_device("gpu"), TR_center: TR_false)`, messages.H7921},
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
// (iii) Kartta `TR_irfft` (i) gibidir; `TR_istft` sıfır toplamı denetlemek için pencereyi konakta okur:
// ısınma doğrudan çağrıyla aynı, kayıt H4065. Bugünkü davranışın kilidi.
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
	out = mustRun(t, `TR_default_device("gpu")
TR_var w = TR_ones([93])
TR_func f(x: TR_Tensor) -> TR_List[TR_Tensor] { TR_return [TR_irfft(x, x * 2.0, TR_n: 93)] }
TR_func g(x: TR_Tensor) -> TR_List[TR_Tensor] { TR_return [TR_istft(x, x * 2.0, 93, TR_hop_length: 40, TR_window: w)] }
TR_var oy = TR_replay(f)
TR_var oyg = TR_replay(g)
TR_no_grad {
    TR_for i TR_in 0..3 {
        TR_var x = TR_randn([2, 47], TR_seed: i)
        TR_print(oy(x)[0].TR_tolist == f(x)[0].TR_tolist)
    }
    TR_var x = TR_randn([47, 3], TR_seed: 0)
    TR_print(oyg(x)[0].TR_tolist == g(x)[0].TR_tolist)
    TR_print(TR_error_code { oyg(TR_randn([47, 3], TR_seed: 1)) })
}`)
	if want := "true\ntrue\ntrue\ntrue\nH4065\n"; out != want {
		t.Fatalf("kayıtta kartın ters dönüşümleri: %q, beklenen %q", out, want)
	}
}
