package profiler

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// gpuTraining GPU'da (benzetim) 6 adımlık küçük bir eğitimdir.
const gpuTraining = `varsayılan_cihaz("gpu")
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

// setupGPU GPU benzetimini kaydeder (gerçek GPU varsa o kullanılır); test bitince varsayılan
// cihaz CPU'ya döner.
func setupGPU(t *testing.T) {
	t.Helper()
	gpu.RegisterSimulator()
	if !tensor.HasDevice(tensor.GPU(0)) {
		t.Skip("GPU ya da benzetim kurulamadı")
	}
	t.Cleanup(func() { tensor.SetDefaultDevice(tensor.CPU) })
}

// GPU yolu: çekirdek süreleri ve GPU belleği arka ucun profilinden okunur; ölçümü profil
// açar ve bitince eski durumuna döndürür; arka ucun kendi raporu metin raporuna eklenir.
func TestProfileGPUKernelsAndMemory(t *testing.T) {
	setupGPU(t)
	if gpu.ProfileEnabled() {
		t.Skip("TR_GPU_PROFIL açık: ölçümün açılıp kapanması sınanamaz")
	}
	g, _ := runWithClose(t, gpuTraining, Options{GPUMeasure: true})
	if gpu.ProfileEnabled() {
		t.Error("profil bitince GPU ölçümü eski durumuna (kapalı) dönmedi")
	}
	gp := g.GPU
	if !gp.Used || !gp.Measured || gp.Seconds <= 0 || len(gp.Kernels) == 0 {
		t.Fatalf("GPU ölçümü eksik: %+v", gp)
	}
	family := map[string]bool{}
	for _, a := range gp.Families {
		family[a.Name] = true
	}
	if !family["carpim"] {
		t.Errorf("matris çarpımı ailesi yok: %+v", gp.Families)
	}
	if len(gp.Memory) == 0 || gp.Memory[0].PeakLive == 0 {
		t.Errorf("GPU belleği yok: %+v", gp.Memory)
	}
	if !strings.Contains(gp.summary, "GPU profili") {
		t.Errorf("arka ucun raporu alınmadı: %q", gp.summary)
	}
	// GPU işlemlerinin her çağrısı ölçülür (örnekleme yok): eşzamanlama beklemesi kaçmaz.
	b := findOp(g, "konaga_okuma", "ileri")
	if b == nil || b.Device != "gpu" || b.Measured != b.Calls {
		t.Errorf("GPU'da konağa okuma: %+v; her çağrı ölçülmeli", b)
	}
	m := findOp(g, "matris_carpimi", "ileri")
	if m == nil || m.Device != "gpu" || m.Calls != 6 {
		t.Errorf("GPU'da matris çarpımı: %+v; 6 çağrı beklenirdi", m)
	}
	text := ReportText(g)
	for _, p := range []string{"GPU çekirdekleri: cihazda", "carpim", "GPU canlı", "GPU profili"} {
		if !strings.Contains(text, p) {
			t.Errorf("raporda %q yok:\n%s", p, text)
		}
	}
}

// Arka plan işinin profili GPU ölçümünü açmaz; GPU belleği yine görünür.
func TestProfileGPUMeasureOff(t *testing.T) {
	setupGPU(t)
	if gpu.ProfileEnabled() {
		t.Skip("TR_GPU_PROFIL açık")
	}
	g, _ := runWithClose(t, gpuTraining, Options{})
	if !g.GPU.Used || g.GPU.Measured || len(g.GPU.Kernels) != 0 || g.GPU.Note == "" {
		t.Errorf("ölçüm kapalıyken: %+v", g.GPU)
	}
	if len(g.GPU.Memory) == 0 {
		t.Error("ölçüm kapalıyken de GPU belleği görünmeli")
	}
	if strings.Contains(ReportText(g), "GPU profili") {
		t.Error("ölçüm kapalıyken arka ucun raporu eklenmemeli")
	}
}

// runWithClose calistir gibidir; ek olarak program bitince (motorlar kapanmadan önce)
// BeforeClose'yi çağırır (tr izle ve tr profil'in yaptığı gibi).
func runWithClose(t *testing.T, code string, s Options) (*Snapshot, *Profile) {
	t.Helper()
	var pr *Profile
	g, _ := runThen(t, code, s, func(p *Profile) {
		pr = p
		p.BeforeClose()
		p.BeforeClose() // ikinci çağrı bir şey yapmaz
	})
	return g, pr
}

// Satır örneklemesi açıkken rapor tepe GPU belleği anında canlı tensörleri ayıran satıra göre
// verir: en büyük tensörü ayıran satır en üsttedir; örnekleme kapalıyken bölüm yoktur.
func TestProfileGPUPeakByLine(t *testing.T) {
	setupGPU(t)
	const code = `varsayılan_cihaz("gpu")
değişken büyük = rastgele_normal([512, 512], tohum: 1)
değişken küçük = rastgele_normal([16, 16], tohum: 2)
değişken t = toplam(büyük) + toplam(küçük)
varsayılan_cihaz("cpu")
`
	g, _ := runWithClose(t, code, Options{SampleLines: true})
	if len(g.gpuPeak) == 0 || g.gpuPeakTotal == 0 {
		t.Fatalf("tepe bellek satırları yok")
	}
	top := g.gpuPeak[0]
	if top.Line.Line != 2 || !strings.Contains(top.Line.Text, "büyük") || top.Bytes < 512*512*4 {
		t.Errorf("en üstte büyük tensörün satırı beklenirdi: %+v", top)
	}
	text := ReportText(g)
	if !strings.Contains(text, "ayıran satıra göre") || !strings.Contains(text, "rastgele_normal([512, 512]") {
		t.Errorf("raporda tepe bellek bölümü yok:\n%s", text)
	}
	g, _ = runWithClose(t, code, Options{})
	if len(g.gpuPeak) != 0 || strings.Contains(ReportText(g), "ayıran satıra göre") {
		t.Errorf("örnekleme kapalıyken tepe bellek bölümü olmamalı")
	}
}
