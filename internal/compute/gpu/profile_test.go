package gpu

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestProfileMeasures ölçüm açıkken çekirdeklerin ve sürücü bellek işlerinin sayılıp sürelerinin
// toplandığını, adım özetinin ve bellek durumunun yazıldığını denetler.
func TestProfileMeasures(t *testing.T) {
	EnableProfile(true)
	defer EnableProfile(false)
	g := gpuBackend()
	a := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	b := tensor.NewOn(gpuDevice, []float32{1, 0, 0, 1, 1, 1}, 3, 2)
	before := map[string]int64{}
	for _, o := range KernelSamples() {
		before[o.Name] = o.Calls
	}
	for step := 0; step < 4; step++ {
		g.enter()
		g.markStep()
		g.exit()
		y := tensor.MatMul(a, b).SumAll()
		y.Backward()
		_ = y.Item()
	}
	g.enter()
	g.markStep()
	g.exit()
	var gemm2, total int64
	for _, o := range KernelSamples() {
		fresh2 := o.Calls - before[o.Name]
		total += fresh2
		if o.Family == "carpim" {
			gemm2 += fresh2
		}
		if o.GPUTime < 0 {
			t.Fatalf("%s: negatif süre", o.Name)
		}
	}
	if gemm2 < 4 || total < 8 {
		t.Fatalf("ölçüm eksik: %d çarpım, %d toplam başlatma", gemm2, total)
	}
	var w bytes.Buffer
	WriteProfileSummary(&w)
	t.Log("\n" + w.String())
	for _, part := range []string{"GPU profili", "aile", "carpim", "bellek: ayrılmış"} {
		if !strings.Contains(w.String(), part) {
			t.Fatalf("özette %q yok:\n%s", part, w.String())
		}
	}
	// Adım satırı adımların süresine bağlı değildir (Windows'ta kısa bir adımın duvar süresi
	// kaba saatle 0 ölçülebilir): ısınma dışındaki tamamlanmış adımlar (en az 3. ve 4.) sayılır.
	// Paketteki öbür testler profili açıp adım işaretlemiş olabilir; sayı yalnızca artar.
	if e := regexp.MustCompile(`adım başına \((\d+) adım\)`).FindStringSubmatch(w.String()); e == nil {
		t.Fatalf("özette adım satırı yok:\n%s", w.String())
	} else if n, _ := strconv.Atoi(e[1]); n < 2 {
		t.Fatalf("özette %d adım, en az 2 bekleniyordu:\n%s", n, w.String())
	}
	if b := MemorySamples(); len(b) == 0 || b[0].Reserved == 0 || b[0].Live > b[0].Reserved+b[0].Pooled {
		t.Fatalf("bellek ölçümü: %+v", b)
	}
}

// TestProfileStepLine adım satırının adımların süresine değil tamamlanmasına baktığını ve
// tamamlanmış adım yoksa bunu söylediğini denetler. Profilin adım kayıtları test boyunca
// değiştirilip sonra geri konur (paylaşılan durum).
func TestProfileStepLine(t *testing.T) {
	g := gpuBackend()
	p := &g.profiler
	p.mu.Lock()
	prev := p.steps
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.steps = prev
		p.mu.Unlock()
	}()
	summary := func(steps []stepStats) string {
		p.mu.Lock()
		p.steps = steps
		p.mu.Unlock()
		var w bytes.Buffer
		WriteProfileSummary(&w)
		return w.String()
	}
	// Kaba saatle sıfır ölçülmüş iki tamamlanmış adım (ısınmadan sonra) ve yarım son adım.
	s := summary([]stepStats{{done: true}, {done: true}, {done: true}, {done: true, wall: time.Millisecond}, {}})
	if !strings.Contains(s, "adım başına (2 adım): duvar 0.50 ms") {
		t.Fatalf("sıfır süreli adımlar sayılmadı:\n%s", s)
	}
	for _, steps := range [][]stepStats{nil, {{done: true}, {done: true}, {}}} {
		if s := summary(steps); !strings.Contains(s, "adım başına: adım bilgisi yok") {
			t.Fatalf("%d adım: 'adım bilgisi yok' satırı yok:\n%s", len(steps), s)
		}
	}
}

func TestKernelFamily(t *testing.T) {
	for ad, family := range map[string]string{
		"carpim_128_128_8_8_8_false_true_false": "carpim", "tekli_turev_3_": "tekli_turev",
		"ce_ileri_256": "ce_ileri", "gomme": "gomme", "softmax_256_true": "softmax", "surucu_kopya": "surucu_kopya",
		"cublaslt_carpim_nt": "carpim",
	} {
		if a := kernelFamily(ad); a != family {
			t.Fatalf("%s: aile %q, %q bekleniyordu", ad, a, family)
		}
	}
}

// TestClassSize küçük (≤ 1 MB) boyut sınıflarının istenen boydan küçük olmadığını ve kendi
// sınıfları olduğunu denetler (büyük istekler parçalardan verilir: memory_blocks_test.go).
func TestClassSize(t *testing.T) {
	for _, b := range []uint64{1, 256, 257, 1000, 4097, 1<<20 - 1, 1 << 20} {
		s := classSize(b)
		if s < b || s > 2*b+256 {
			t.Fatalf("%d baytın sınıfı %d", b, s)
		}
		if classSize(s) != s {
			t.Fatalf("sınıf %d kendi sınıfı değil", s)
		}
	}
}

// TestPeakLiveBySite konum izlemesi açıkken tepe anındaki canlı belleğin ayıran konuma göre
// dağılımını verdiğini denetler: tepeden sonra bırakılan blok tepede sayılır, tepeden sonra
// ayrılan küçük blok sayılmaz; izleme kapanınca ayırmalar kaydedilmez.
func TestPeakLiveBySite(t *testing.T) {
	site := uint64(0)
	SetAllocSite(func() uint64 { return site })
	g := gpuBackend()
	g.enter()
	site = 1
	a := g.allocate(1<<20, tensor.Float32, false) // 4 MB
	site = 2
	b := g.allocate(1<<18, tensor.Float32, false) // 1 MB
	g.release(a)
	site = 3
	c := g.allocate(1<<16, tensor.Float32, false) // 256 KB
	g.exit()
	sites, peak := PeakLiveBySite()
	by := map[uint64]SitePeak{}
	for _, s := range sites {
		by[s.Site] = s
	}
	ca, cb := largeSize(4<<20), classSize(1<<20)
	if by[1].Bytes != ca || by[2].Bytes != cb || by[1].Blocks != 1 || by[1].DType != tensor.Float32 || by[3].Bytes != 0 || peak != ca+cb {
		t.Fatalf("tepe dağılımı %+v (tepe %d); beklenen 1:%d 2:%d (birer float32 blok), tepe %d", sites, peak, ca, cb, ca+cb)
	}
	SetAllocSite(nil)
	g.enter()
	d := g.allocate(1<<20, tensor.Float32, false)
	g.release(b)
	g.release(c)
	g.release(d)
	g.exit()
	if sites, peak := PeakLiveBySite(); len(sites) != 0 || peak != 0 {
		t.Fatalf("izleme kapalıyken dağılım kalmamalı: %v (tepe %d)", sites, peak)
	}
}
