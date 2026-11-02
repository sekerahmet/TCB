package gpu

import (
	"math"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.1, 15.39): TR_Int ve TR_Bool tensörü GPU'ya kayıpsız taşınır; görünümler ve adımlı
// kopyalar cinsi korur (2²⁴'ün üstündeki tam sayı bozulmaz).
func TestTypedStorageOnGPU(t *testing.T) {
	vals := []int64{16777217, -9007199254740993, math.MaxInt64, math.MinInt64, 5, 6}
	c := tensor.Int64sOn(tensor.CPU, tensor.Int64, vals, 2, 3)
	g := c.To(gpuDevice)
	if g.DType() != tensor.Int64 || !slices.Equal(g.Int64s(), vals) {
		t.Fatalf("int64 GPU'da: %v", g.Int64s())
	}
	want := c.Transpose(0, 1).Contiguous().Int64s()
	if got := g.Transpose(0, 1).Contiguous().Int64s(); !slices.Equal(got, want) {
		t.Fatalf("devrik kopya: GPU %v, CPU %v", got, want)
	}
	if got := g.Slice(1, 2, 3).Squeeze(1).Int64s(); !slices.Equal(got, []int64{math.MaxInt64, 6}) {
		t.Fatalf("seçim: %v", got)
	}
	b := tensor.Int64sOn(gpuDevice, tensor.Bool, []int64{1, 0, 1}, 3)
	if got := b.To(tensor.CPU).Int64s(); !slices.Equal(got, []int64{1, 0, 1}) {
		t.Fatalf("bool: %v", got)
	}
	if got := b.Slice(0, 1, 3).Contiguous().Int64s(); !slices.Equal(got, []int64{0, 1}) {
		t.Fatalf("bool dilim: %v", got)
	}
	if z := tensor.ZerosOf(tensor.Bool, 5).To(gpuDevice); !slices.Equal(z.Int64s(), []int64{0, 0, 0, 0, 0}) {
		t.Fatalf("sıfır bool: %v", z.Int64s())
	}
}

// Kural (15.48): GPU'nun ondalık çekirdeği ham bayt türünü sessizce float32 saymaz.
func TestTypedInGPUFloatKernelIsPrecisionError(t *testing.T) {
	x := tensor.Int64sOn(gpuDevice, tensor.Int64, []int64{1, 2}, 2)
	err := testkit.PanicError(func() { x.Softmax(0) })
	if !messages.Has(err, messages.H6969) {
		t.Fatalf("hassasiyet hatası bekleniyordu: %v", err)
	}
}

// Kural: konaktan yüklenen küçük ham bayt depolaması (veri kümesinin TR_Int kimlik toplu işi)
// float32'ninki gibi konak gölgesi tutar; konağa okuma eşzamanlamasızdır, her yazma yolu gölgeyi
// siler. Neden (gerçek kart, 2026-11-02): kimlikler int64 olunca gömme ve kayıp hedefleri her
// mikro toplu işte GPU kuyruğunu bekledi, hedef model adım başına %6 yavaşladı.
func TestTypedShadowReadsWithoutSync(t *testing.T) {
	g := gpuBackend()
	n := 12
	raw := func(vals ...int64) []byte {
		b := make([]byte, 8*len(vals))
		for i, v := range vals {
			tensor.PutRawInt(tensor.Int64, b[8*i:], v)
		}
		return b
	}
	seq := make([]int64, n)
	for i := range seq {
		seq[i] = int64(i) - 3
	}
	plain := func(s tensor.Storage) tensor.View { return flatView(s, n) }
	s := g.FromRaw(raw(seq...), tensor.Int64)
	before := syncCount(g)
	if got := g.ToRaw(plain(s)); !slices.Equal(got, raw(seq...)) {
		t.Fatalf("gölgeden okuma: %v", got)
	}
	if got := g.ToRaw(tensor.View{S: s, Offset: 1, Shape: []int{3}, Strides: []int{4}}); !slices.Equal(got, raw(-2, 2, 6)) {
		t.Fatalf("adımlı görünüm gölgeden: %v", got)
	}
	if after := syncCount(g); after != before {
		t.Fatalf("gölgeli ham bayt okuması %d kez eşzamanladı", after-before)
	}
	seven := make([]int64, n)
	for i := range seven {
		seven[i] = 7
	}
	for ad, write := range map[string]func(s tensor.Storage){
		"FillRaw":     func(s tensor.Storage) { g.FillRaw(s, raw(7)) },
		"CopyStrided": func(s tensor.Storage) { g.CopyStrided(plain(s), plain(g.FromRaw(raw(seven...), tensor.Int64))) },
		"Copy":        func(s tensor.Storage) { g.Copy(s, 0, plain(g.FromRaw(raw(seven...), tensor.Int64))) },
	} {
		s := g.FromRaw(raw(seq...), tensor.Int64)
		write(s)
		if got := g.ToRaw(plain(s)); !slices.Equal(got, raw(seven...)) {
			t.Fatalf("%s sonrası okunan değer eski: %v", ad, got)
		}
	}
	// Adımlı hedef çekirdek başlatmasıyla yazılır (ham_kopya; yukarıdakiler sürücü yolundandır):
	// başlatma yazdığı depolamanın ham gölgesini de siler.
	s = g.FromRaw(raw(seq...), tensor.Int64)
	g.CopyStrided(tensor.View{S: s, Shape: []int{n / 2}, Strides: []int{2}}, flatView(g.FromRaw(raw(seven[:n/2]...), tensor.Int64), n/2))
	want := slices.Clone(seq)
	for i := 0; i < n; i += 2 {
		want[i] = 7
	}
	if got := g.ToRaw(plain(s)); !slices.Equal(got, raw(want...)) {
		t.Fatalf("adımlı CopyStrided (çekirdek) sonrası okunan değer eski: %v", got)
	}
}

// syncCount arka ucun şimdiye kadarki eşzamanlama sayısıdır.
func syncCount(g *backend) int64 {
	g.enter()
	defer g.exit()
	return g.profiler.syncs
}
