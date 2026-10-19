package gpu

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Benzetimin vektörel erişimleri ve cp.async'i: doğru kullanım doğru sonuç verir; beklemeden
// okuma, beklenmeyen kopya ve hizasız erişim yakalanır.

// vekCekirdek 4 iş parçacıklı tek blok: her iş parçacığı girdinin kendi 4'lüsünü paylaşımlı
// belleğe taşır (vektörel ld/st ya da cp.async), engelden sonra komşusunun 4'lüsünü çıktıya
// vektörel yazar. kip: "vek", "cp", "cp_beklemesiz", "cp_engelsiz", "hizasiz".
func vekCekirdek(kip string) func() *yazici {
	return func() *yazici {
		k := yeniYazici("vek_" + kip)
		if strings.HasPrefix(kip, "cp") {
			k.hedef = 80
		}
		x := k.gosterici("x")
		y := k.yazilan("y")
		sm := k.ortak("sm", 64)
		tid := k.ozel("%tid.x")
		ofs := k.r()
		k.e("mul.lo.s32", ofs, tid, 16)
		if kip == "hizasiz" {
			k.e("add.s32", ofs, ofs, 4)
		}
		ga := k.rd()
		k.e("cvt.u64.u32", ga, ofs)
		k.e("add.s64", ga, ga, x)
		sa := k.r()
		k.e("add.s32", sa, sm, ofs)
		if strings.HasPrefix(kip, "cp") {
			k.cpAsync(sa, 0, ga, k.tam(16))
			k.cpGrup()
			if kip != "cp_beklemesiz" {
				k.cpBekle(0)
			}
		} else {
			v := k.yukleV("global", "", ga, 0, 4)
			k.saklaV("shared", "", sa, 0, v)
		}
		if kip != "cp_engelsiz" {
			k.engel()
		}
		komsu := k.r()
		k.e("add.s32", komsu, tid, 1)
		k.e("and.b32", komsu, komsu, 3)
		k.e("mul.lo.s32", komsu, komsu, 16)
		k.e("add.s32", komsu, komsu, sm)
		v := k.yukleV("shared", "", komsu, 0, 4)
		ya := k.rd()
		k.e("mul.wide.u32", ya, tid, 16)
		k.e("add.s64", ya, ya, y)
		k.saklaV("global", "", ya, 0, v)
		return k
	}
}

func vekCalistir(t *testing.T, kip string) ([]float32, string) {
	t.Helper()
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	defer g.cik()
	veri := make([]float32, 20)
	for i := range veri {
		veri[i] = float32(i + 1)
	}
	x := kilitliYukle(g, veri)
	y := g.ayir(16, tensor.Float32, true)
	c := g.cekirdekAl("vek:"+kip, vekCekirdek(kip))
	g.baslat(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().oku(x, 0).yaz(y, 0))
	m := panikMetni(g.esitle)
	if m != "" {
		return nil, m
	}
	return kilitliIndir(g, y), ""
}

func TestBenzetimVektorVeCpAsync(t *testing.T) {
	for _, kip := range []string{"vek", "cp"} {
		r, m := vekCalistir(t, kip)
		if m != "" {
			t.Fatalf("%s: %s", kip, m)
		}
		for i, v := range r {
			if beklenen := float32((i+4)%16 + 1); v != beklenen {
				t.Fatalf("%s: y[%d] = %v, %v bekleniyordu", kip, i, v, beklenen)
			}
		}
	}
	hatalar := map[string]string{
		"cp_beklemesiz": "beklenmeden bitti",
		"cp_engelsiz":   "yarış",
		"hizasiz":       "hizasız",
	}
	for kip, parca := range hatalar {
		if _, m := vekCalistir(t, kip); !strings.Contains(m, parca) {
			t.Fatalf("%s: %q hatası bekleniyordu, %q", kip, parca, m)
		}
	}
}

// TestCpAsyncSifirDoldurur kaynak boyu 0 olan kopyanın hedefi sıfırladığını denetler.
func TestCpAsyncSifirDoldurur(t *testing.T) {
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	defer g.cik()
	y := kilitliYukle(g, []float32{9, 9, 9, 9})
	c := g.cekirdekAl("cp_sifir", func() *yazici {
		k := yeniYazici("cp_sifir")
		k.hedef = 80
		yy := k.yazilan("y")
		sm := k.ortak("sm", 16)
		v0 := k.sabit(5)
		k.saklaV("shared", "", sm, 0, []yazmac{v0, v0, v0, v0})
		k.engel()
		k.cpAsync(sm, 0, yy, k.tam(0))
		k.cpGrup()
		k.cpBekle(0)
		k.engel()
		k.saklaV("global", "", yy, 0, k.yukleV("shared", "", sm, 0, 4))
		return k
	})
	g.baslat(c, [3]uint32{1, 1, 1}, [3]uint32{1, 1, 1}, g.param().yaz(y, 0))
	if m := panikMetni(g.esitle); m != "" {
		t.Fatal(m)
	}
	for i, v := range kilitliIndir(g, y) {
		if v != 0 {
			t.Fatalf("y[%d] = %v, 0 bekleniyordu", i, v)
		}
	}
}

// kilitliIndir arka ucun kilidi tutulurken depolamayı cihazdan okur.
func kilitliIndir(g *arkaUc, d *depo) []float32 {
	out := make([]float32, d.n)
	g.indir(unsafe.Pointer(&out[0]), d.adr, 4*d.n)
	return out
}

// kilitliYukle arka ucun kilidi tutulurken konak verisini yeni bir depolamaya yükler.
func kilitliYukle(g *arkaUc, veri []float32) *depo {
	d := g.ayir(len(veri), tensor.Float32, false)
	g.yukleKonak(d, unsafe.Pointer(&veri[0]), 4*len(veri))
	return d
}
