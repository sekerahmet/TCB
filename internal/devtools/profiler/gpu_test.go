package profiler

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// gpuEgitimi GPU'da (benzetim) 6 adımlık küçük bir eğitimdir.
const gpuEgitimi = `varsayılan_cihaz("gpu")
değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)
değişken op = AdamW([w], öğrenme_oranı: 0.01)
değişken ç = ilerleme(6)
döngü i içinde 0..6 {
    op.sıfırla()
    değişken kayıp = toplam(relu(x @ w))
    geri_yayılım(kayıp)
    op.adım()
    ç.güncelle({kayıp: kayıp})
}
varsayılan_cihaz("cpu")
`

// gpuKur GPU benzetimini kaydeder (gerçek GPU varsa o kullanılır); test bitince varsayılan
// cihaz CPU'ya döner.
func gpuKur(t *testing.T) {
	t.Helper()
	gpu.RegisterSimulator()
	if !tensor.HasDevice(tensor.GPU(0)) {
		t.Skip("GPU ya da benzetim kurulamadı")
	}
	t.Cleanup(func() { tensor.SetDefaultDevice(tensor.CPU) })
}

// GPU yolu: çekirdek süreleri ve GPU belleği arka ucun profilinden okunur; ölçümü profil
// açar ve bitince eski durumuna döndürür; arka ucun kendi raporu metin raporuna eklenir.
func TestProfilGPUCekirdekleriVeBellegi(t *testing.T) {
	gpuKur(t)
	if gpu.ProfileEnabled() {
		t.Skip("TR_GPU_PROFIL açık: ölçümün açılıp kapanması sınanamaz")
	}
	g, _ := calistirKapanisla(t, gpuEgitimi, Secenekler{GPUOlcumu: true})
	if gpu.ProfileEnabled() {
		t.Error("profil bitince GPU ölçümü eski durumuna (kapalı) dönmedi")
	}
	gp := g.GPU
	if !gp.Kullanildi || !gp.Olcum || gp.SureSn <= 0 || len(gp.Cekirdekler) == 0 {
		t.Fatalf("GPU ölçümü eksik: %+v", gp)
	}
	aile := map[string]bool{}
	for _, a := range gp.Aileler {
		aile[a.Ad] = true
	}
	if !aile["carpim"] {
		t.Errorf("matris çarpımı ailesi yok: %+v", gp.Aileler)
	}
	if len(gp.Bellek) == 0 || gp.Bellek[0].TepeCanli == 0 {
		t.Errorf("GPU belleği yok: %+v", gp.Bellek)
	}
	if !strings.Contains(gp.ozet, "GPU profili") {
		t.Errorf("arka ucun raporu alınmadı: %q", gp.ozet)
	}
	// GPU işlemlerinin her çağrısı ölçülür (örnekleme yok): eşzamanlama beklemesi kaçmaz.
	b := islemBul(g, "konaga_okuma", "ileri")
	if b == nil || b.Cihaz != "gpu" || b.Olcum != b.Cagri {
		t.Errorf("GPU'da konağa okuma: %+v; her çağrı ölçülmeli", b)
	}
	m := islemBul(g, "matris_carpimi", "ileri")
	if m == nil || m.Cihaz != "gpu" || m.Cagri != 6 {
		t.Errorf("GPU'da matris çarpımı: %+v; 6 çağrı beklenirdi", m)
	}
	metin := RaporMetni(g)
	for _, p := range []string{"GPU çekirdekleri: cihazda", "carpim", "GPU canlı", "GPU profili"} {
		if !strings.Contains(metin, p) {
			t.Errorf("raporda %q yok:\n%s", p, metin)
		}
	}
}

// Arka plan işinin profili GPU ölçümünü açmaz; GPU belleği yine görünür.
func TestProfilGPUOlcumuKapali(t *testing.T) {
	gpuKur(t)
	if gpu.ProfileEnabled() {
		t.Skip("TR_GPU_PROFIL açık")
	}
	g, _ := calistirKapanisla(t, gpuEgitimi, Secenekler{})
	if !g.GPU.Kullanildi || g.GPU.Olcum || len(g.GPU.Cekirdekler) != 0 || g.GPU.Not == "" {
		t.Errorf("ölçüm kapalıyken: %+v", g.GPU)
	}
	if len(g.GPU.Bellek) == 0 {
		t.Error("ölçüm kapalıyken de GPU belleği görünmeli")
	}
	if strings.Contains(RaporMetni(g), "GPU profili") {
		t.Error("ölçüm kapalıyken arka ucun raporu eklenmemeli")
	}
}

// calistirKapanisla calistir gibidir; ek olarak program bitince (motorlar kapanmadan önce)
// KapanmadanOnce'yi çağırır (tr izle ve tr profil'in yaptığı gibi).
func calistirKapanisla(t *testing.T, kod string, s Secenekler) (*Goruntu, *Profil) {
	t.Helper()
	var pr *Profil
	g, _ := calistirSonra(t, kod, s, func(p *Profil) {
		pr = p
		p.KapanmadanOnce()
		p.KapanmadanOnce() // ikinci çağrı bir şey yapmaz
	})
	return g, pr
}
