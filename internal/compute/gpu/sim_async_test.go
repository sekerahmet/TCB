package gpu

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Benzetimin vektörel erişimleri ve cp.async'i: doğru kullanım doğru sonuç verir; beklemeden
// okuma, beklenmeyen kopya ve hizasız erişim yakalanır.

// vecKernel 4 iş parçacıklı tek blok: her iş parçacığı girdinin kendi 4'lüsünü paylaşımlı
// belleğe taşır (vektörel ld/st ya da cp.async), engelden sonra komşusunun 4'lüsünü çıktıya
// vektörel yazar. kip: "vek", "cp", "cp_beklemesiz", "cp_engelsiz", "hizasiz".
func vecKernel(mode2 string) func() *emitter {
	return func() *emitter {
		k := newEmitter("vek_" + mode2)
		if strings.HasPrefix(mode2, "cp") {
			k.target = 80
		}
		x := k.pointer("x")
		y := k.written("y")
		sm := k.shared("sm", 64)
		tid := k.special("%tid.x")
		ofs := k.r()
		k.e("mul.lo.s32", ofs, tid, 16)
		if mode2 == "hizasiz" {
			k.e("add.s32", ofs, ofs, 4)
		}
		ga := k.rd()
		k.e("cvt.u64.u32", ga, ofs)
		k.e("add.s64", ga, ga, x)
		sa := k.r()
		k.e("add.s32", sa, sm, ofs)
		if strings.HasPrefix(mode2, "cp") {
			k.cpAsync(sa, 0, ga, k.whole(16))
			k.cpGroup()
			if mode2 != "cp_beklemesiz" {
				k.cpWait(0)
			}
		} else {
			v := k.loadV("global", "", ga, 0, 4)
			k.storeV("shared", "", sa, 0, v)
		}
		if mode2 != "cp_engelsiz" {
			k.barrier()
		}
		neighbor := k.r()
		k.e("add.s32", neighbor, tid, 1)
		k.e("and.b32", neighbor, neighbor, 3)
		k.e("mul.lo.s32", neighbor, neighbor, 16)
		k.e("add.s32", neighbor, neighbor, sm)
		v := k.loadV("shared", "", neighbor, 0, 4)
		ya := k.rd()
		k.e("mul.wide.u32", ya, tid, 16)
		k.e("add.s64", ya, ya, y)
		k.storeV("global", "", ya, 0, v)
		return k
	}
}

func vecRun(t *testing.T, mode2 string) ([]float32, string) {
	t.Helper()
	g := newSimBackend(1 << 20)
	g.enter()
	defer g.exit()
	data := make([]float32, 20)
	for i := range data {
		data[i] = float32(i + 1)
	}
	x := lockedLoad(g, data)
	y := g.allocate(16, tensor.Float32, true)
	c := g.takeKernel("vek:"+mode2, vecKernel(mode2))
	g.launch(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().read(x, 0).write(y, 0))
	m := panicText(g.equalize)
	if m != "" {
		return nil, m
	}
	return lockedDownload(g, y), ""
}

func TestSimVectorAndCpAsync(t *testing.T) {
	for _, mode2 := range []string{"vek", "cp"} {
		r, m := vecRun(t, mode2)
		if m != "" {
			t.Fatalf("%s: %s", mode2, m)
		}
		for i, v := range r {
			if expected := float32((i+4)%16 + 1); v != expected {
				t.Fatalf("%s: y[%d] = %v, %v bekleniyordu", mode2, i, v, expected)
			}
		}
	}
	errs := map[string]string{
		"cp_beklemesiz": "beklenmeden bitti",
		"cp_engelsiz":   "yarış",
		"hizasiz":       "hizasız",
	}
	for mode2, part := range errs {
		if _, m := vecRun(t, mode2); !strings.Contains(m, part) {
			t.Fatalf("%s: %q hatası bekleniyordu, %q", mode2, part, m)
		}
	}
}

// TestCpAsyncZeroFills kaynak boyu 0 olan kopyanın hedefi sıfırladığını denetler.
func TestCpAsyncZeroFills(t *testing.T) {
	g := newSimBackend(1 << 20)
	g.enter()
	defer g.exit()
	y := lockedLoad(g, []float32{9, 9, 9, 9})
	c := g.takeKernel("cp_sifir", func() *emitter {
		k := newEmitter("cp_sifir")
		k.target = 80
		yy := k.written("y")
		sm := k.shared("sm", 16)
		v0 := k.constant(5)
		k.storeV("shared", "", sm, 0, []reg{v0, v0, v0, v0})
		k.barrier()
		k.cpAsync(sm, 0, yy, k.whole(0))
		k.cpGroup()
		k.cpWait(0)
		k.barrier()
		k.storeV("global", "", yy, 0, k.loadV("shared", "", sm, 0, 4))
		return k
	})
	g.launch(c, [3]uint32{1, 1, 1}, [3]uint32{1, 1, 1}, g.param().write(y, 0))
	if m := panicText(g.equalize); m != "" {
		t.Fatal(m)
	}
	for i, v := range lockedDownload(g, y) {
		if v != 0 {
			t.Fatalf("y[%d] = %v, 0 bekleniyordu", i, v)
		}
	}
}

// lockedDownload arka ucun kilidi tutulurken depolamayı cihazdan okur.
func lockedDownload(g *backend, d *storage) []float32 {
	out := make([]float32, d.n)
	g.download(unsafe.Pointer(&out[0]), d.addr, 4*d.n)
	return out
}

// lockedLoad arka ucun kilidi tutulurken konak verisini yeni bir depolamaya yükler.
func lockedLoad(g *backend, data []float32) *storage {
	d := g.allocate(len(data), tensor.Float32, false)
	g.loadHost(d, unsafe.Pointer(&data[0]), 4*len(data))
	return d
}
