package gpu

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"tr_dili/internal/hesap/tensor"
)

// Benzetimin hata yakalama yetenekleri ve arka ucun hata mesajları. Her sınama kendi
// benzetim arka ucunu kurar (yapışkan hata diğerlerini etkilemesin).

// Eşzamanlı kip (TR_GPU_ESZAMANLI=1) bu arka uçlarda kapalıdır: hatayı başlatmada değil
// eşzamanlamada beklerler; ortam değişkeninden bağımsız olmalıdırlar.
func yeniBenzetimArkaUcu(bellek uint64) *arkaUc {
	g := yeniArkaUc(tensor.GPU(7), yeniBenzetim(bellek))
	g.eszamanli = false
	return g
}

// calistirHata, govde'nin yazdığı tek bloklu (4 iş parçacığı) çekirdeği 16 baytlık bir
// tamponla başlatır ve eşzamanlama hatasının mesajını verir.
func calistirHata(t *testing.T, ad string, govde func(k *yazici, x yazmac)) string {
	t.Helper()
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	defer g.cik()
	d := g.ayir(4, tensor.Float32, true)
	c := g.cekirdekAl(ad, func() *yazici {
		k := yeniYazici(ad)
		govde(k, k.yazilan("x"))
		return k
	})
	g.baslat(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().yaz(d, 0))
	return panikMetni(g.esitle)
}

func TestBenzetimYarisYakalar(t *testing.T) {
	m := calistirHata(t, "yaris", func(k *yazici, x yazmac) {
		sm := k.ortak("sm", 16)
		tid := k.ozel("%tid.x")
		v := k.f()
		k.e("cvt.rn.f32.u32", v, tid)
		k.e("st.shared.f32", bel(sm, 0), v) // dört iş parçacığı aynı sözcüğe yazar
	})
	if !strings.Contains(m, "yarış") || !strings.Contains(m, "CUDA_ERROR_ILLEGAL_ADDRESS") {
		t.Fatalf("paylaşımlı bellek yarışı yakalanmadı: %q", m)
	}
	// Engelle ayrılmış yazma ve okuma yarış değildir.
	m = calistirHata(t, "engelli", func(k *yazici, x yazmac) {
		sm := k.ortak("sm", 16)
		tid := k.ozel("%tid.x")
		adr := k.r()
		k.e("mad.lo.s32", adr, tid, 4, sm)
		v := k.f()
		k.e("cvt.rn.f32.u32", v, tid)
		k.e("st.shared.f32", bel(adr, 0), v)
		k.engel()
		r := k.f()
		k.e("ld.shared.f32", r, bel(sm, 4))
		k.sakla(tensor.Float32, k.adres(x, tid, 4), r)
	})
	if m != "" {
		t.Fatalf("engelli erişim hata verdi: %s", m)
	}
}

func TestBenzetimDaginikEngel(t *testing.T) {
	m := calistirHata(t, "daginik", func(k *yazici, x yazmac) {
		tid := k.ozel("%tid.x")
		son := k.etiket()
		p := k.p()
		k.e("setp.ne.u32", p, tid, 0)
		k.gitEger(p, son)
		k.engel()
		k.koy(son)
	})
	if !strings.Contains(m, "dağınık engel") {
		t.Fatalf("dağınık engel yakalanmadı: %q", m)
	}
}

func TestBenzetimSinirDisi(t *testing.T) {
	m := calistirHata(t, "sinir_disi", func(k *yazici, x yazmac) {
		tid := k.ozel("%tid.x")
		i := k.r()
		// 1000..1003: tamponun boyut sınıfının (256 bayt) da dışı; benzetim yalnızca ayrılmış
		// bölgenin (sınıf boyu) dışını yakalar, donanım gibi.
		k.e("add.s32", i, tid, 1000)
		k.sakla(tensor.Float32, k.adres(x, i, 4), k.sabit(1))
	})
	if !strings.Contains(m, "ayrılmış bölge dışında") || !strings.Contains(m, "son başlatılan çekirdek: sinir_disi") {
		t.Fatalf("sınır dışı yazma yakalanmadı: %q", m)
	}
}

func TestPTXDerlemeHatasiGunluk(t *testing.T) {
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	defer g.cik()
	m := panikMetni(func() {
		g.cekirdekAl("bozuk", func() *yazici {
			k := yeniYazici("bozuk")
			k.e("bilinmeyen.f32", k.f(), k.f())
			return k
		})
	})
	if !strings.Contains(m, "CUDA_ERROR_INVALID_PTX") || !strings.Contains(m, "PTX derleyici günlüğü") || !strings.Contains(m, "PTX metni:") {
		t.Fatalf("derleme hatası mesajı eksik: %q", m)
	}
}

// TestParametreBildirimiDenetlenir yazan çekirdeğe "oku" ile verilen depolamanın iç hata
// olduğunu denetler (gölge silinmesinin unutulamaması bunun üzerine kuruludur).
func TestParametreBildirimiDenetlenir(t *testing.T) {
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	defer g.cik()
	d := g.ayir(4, tensor.Float32, true)
	c := g.cekirdekAl("doldur:float32x", doldurCekirdegi(tensor.Float32))
	m := panikMetni(func() { g.baslat(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().oku(d, 0).u32(4).f32(1)) })
	if !strings.Contains(m, "okuma/yazma bildirimi imzayla uyuşmuyor") {
		t.Fatalf("yanlış bildirim yakalanmadı: %q", m)
	}
}

// TestIcIceParametreZinciri, bir başlatmanın parametreleri yazılırken ikinci bir başlatma
// hazırlanmasının (dış zinciri silerdi) iç hata verdiğini ve sonraki girişin temiz
// başladığını denetler.
func TestIcIceParametreZinciri(t *testing.T) {
	g := yeniBenzetimArkaUcu(1 << 20)
	g.gir()
	m := panikMetni(func() { g.param().u32(1); g.param() })
	g.cik()
	if !strings.Contains(m, "iç içe param()") {
		t.Fatalf("iç içe zincir yakalanmadı: %q", m)
	}
	g.gir()
	defer g.cik()
	if m := panikMetni(func() { g.param() }); m != "" {
		t.Fatalf("yeni girişte zincir kapalı olmalıydı: %q", m)
	}
}

func TestBellekYetmezse(t *testing.T) {
	g := yeniBenzetimArkaUcu(1 << 20)
	m := panikMetni(func() {
		var tut []tensor.Storage
		for i := 0; i < 8; i++ {
			tut = append(tut, g.Alloc(64<<10, tensor.Float32)) // 256 KB
		}
		_ = tut
	})
	if !strings.Contains(m, "GPU belleği yetmedi") || !strings.Contains(m, "CUDA_ERROR_OUT_OF_MEMORY") {
		t.Fatalf("bellek yetmezliği mesajı: %q", m)
	}
	// Cihaz belleğinden büyük tek ayırma, ayırmadan önce reddedilir.
	m = panikMetni(func() { g.Alloc(1<<20, tensor.Float32) })
	if !strings.Contains(m, "bu GPU'da") {
		t.Fatalf("cihaz sınırı mesajı: %q", m)
	}
	// Cihazdan bağımsız katman şekli GPU belleğiyle denetler (ayırmadan önce). Satır sayısı
	// GPU belleğini az farkla aşar; konağın belleği GPU'nunkinden küçükse önce konak sınırı
	// devreye girer (gerçek GPU'da), o da aynı biçimde Türkçe ve yakalanabilirdir.
	satir := int(gpuArkaUc().MemoryLimit()/16) + 1
	m = panikMetni(func() { tensor.ZerosOn(gpuCihaz, satir, 4) })
	if !strings.Contains(m, fmt.Sprintf("[%d, 4]", satir)) || !(strings.Contains(m, "bu GPU'da") || strings.Contains(m, "bu makinede")) {
		t.Fatalf("tensör katmanının GPU sınırı mesajı: %q", m)
	}
	if benzetimde() && !strings.Contains(m, "bu GPU'da") {
		t.Fatalf("benzetimde GPU sınırı devreye girmeliydi: %q", m)
	}
}

func TestDesteklenmeyenler(t *testing.T) {
	g := gpuArkaUc()
	if m := panikMetni(func() { g.Alloc(4, tensor.Int8) }); !strings.Contains(m, "bu işlem GPU'da henüz yok") {
		t.Fatalf("int8 mesajı: %q", m)
	}
	a := tensor.NewOn(tensor.CPU, []float32{1, 2}, 2)
	b := tensor.NewOn(gpuCihaz, []float32{1, 2}, 2)
	if m := panikMetni(func() { tensor.Add(a, b) }); !strings.Contains(m, "farklı cihazlarda") {
		t.Fatalf("farklı cihaz mesajı: %q", m)
	}
}

// TestSurucuYoksaZarif NVIDIA sürücüsü olmayan makinede (Linux'ta dlopen, Windows'ta
// DLL yükleme hatası) paketin paniklemeden "GPU yok" dediğini ve tensörlerin CPU'da
// kaldığını denetler. Gerçek GPU'lu makinede atlanır.
func TestSurucuYoksaZarif(t *testing.T) {
	if !benzetimde() {
		t.Skip("gerçek GPU var")
	}
	if n := cihazSayisi(); n != 0 {
		t.Fatalf("sürücü yokken cihaz sayısı %d", n)
	}
	if !strings.HasPrefix(Durum(), "GPU yok: ") {
		t.Fatalf("durum: %q", Durum())
	}
	if tensor.HasDevice(tensor.GPU(1)) {
		t.Fatal("sürücü yokken gpu:1 görünüyor")
	}
	if d := tensor.NewOn(tensor.CPU, []float32{1}, 1).Device(); d != tensor.CPU {
		t.Fatalf("tensör CPU'da değil: %s", d)
	}
}

// cihazdan depolamanın içeriğini gölgeye bakmadan cihazdan okur.
func cihazdan(g *arkaUc, s tensor.Storage) []float32 {
	g.gir()
	defer g.cik()
	d := g.depoOf(s)
	out := make([]float32, d.n)
	g.indir(unsafe.Pointer(&out[0]), d.adr, 4*d.n)
	return out
}
