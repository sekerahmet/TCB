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
	b := findOp(g, "to_host", "ileri")
	if b == nil || b.Device != "gpu" || b.Measured != b.Calls {
		t.Errorf("GPU'da konağa okuma: %+v; her çağrı ölçülmeli", b)
	}
	m := findOp(g, "matmul", "ileri")
	if m == nil || m.Device != "gpu" || m.Calls != 6 {
		t.Errorf("GPU'da matris çarpımı: %+v; 6 çağrı beklenirdi", m)
	}
	text := ReportText(g)
	for _, p := range []string{"GPU kernelleri: cihazda", "carpim", "GPU canlı", "GPU profili"} {
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

// Bir fonksiyondan dönüldükten sonra aynı satırda ayrılan bellek çağıran satırda sayılır
// (çağrılan fonksiyonun son satırında değil).
func TestProfileGPUPeakAfterCallReturn(t *testing.T) {
	if !hasDepth {
		t.Skip("yarış denetimli derlemede çağrı derinliği okunmaz")
	}
	setupGPU(t)
	const code = `varsayılan_cihaz("gpu")
fonksiyon küçük() { sonuç rastgele_normal([4, 4], tohum: 1) }
değişken büyük = toplam(küçük()) + rastgele_normal([512, 512], tohum: 2)
varsayılan_cihaz("cpu")
`
	g, _ := runWithClose(t, code, Options{SampleLines: true})
	if len(g.gpuPeak) == 0 {
		t.Fatal("tepe bellek satırları yok")
	}
	if top := g.gpuPeak[0]; top.Line.Line != 3 {
		t.Errorf("büyük tensör çağıran satırda (3) sayılmalı: %+v", top)
	}
}

// gpuTwoOptimizers ilerleme kullanmayan, adımda iki iyileştiricisi olan 6 adımlık eğitimdir.
const gpuTwoOptimizers = `varsayılan_cihaz("gpu")
değişken w = tensör(rastgele_normal([16, 16], tohum: 1), türev_izle: doğru)
değişken v = tensör(rastgele_normal([16, 16], tohum: 3), türev_izle: doğru)
değişken x = rastgele_normal([8, 16], tohum: 2)
değişken a = AdamW([w], öğrenme_oranı: 0.01)
değişken b = AdamW([v], öğrenme_oranı: 0.01)
döngü i içinde 0..6 {
    a.sıfırla()
    b.sıfırla()
    değişken kayıp = toplam(relu(x @ w @ v))
    geri_yayılım(kayıp)
    a.adım()
    b.adım()
}
varsayılan_cihaz("cpu")
`

// Çekirdeklerin cihaz süresi onları başlatan işleme ve satıra yazılır (geri yöndekiler ileri
// satıra); iki iyileştiricili adım bir adımdır (GPU arka ucunun işareti: gpu paketinin
// TestOwnerSamplesAndExternalSteps).
func TestProfileGPUTimeByOperationAndLine(t *testing.T) {
	setupGPU(t)
	g, _ := runWithClose(t, gpuTwoOptimizers, Options{GPUMeasure: true, SampleLines: true})
	gp := g.GPU
	var fwd, back bool
	for _, o := range gp.Ops {
		if o.Code == "matmul" {
			fwd = fwd || o.Direction == "ileri"
			back = back || o.Direction == "geri"
		}
	}
	if !fwd || !back {
		t.Errorf("GPU süresi işleme göre: %+v; matris çarpımının ileri ve geri çekirdekleri olmalı", gp.Ops)
	}
	var line *GPULineInfo
	for i := range gp.Lines {
		if gp.Lines[i].Line == 10 {
			line = &gp.Lines[i]
		}
	}
	if line == nil || line.BackwardShare <= 0 {
		t.Errorf("GPU süresi satıra göre: %+v; 10. satır (x @ w @ v) geri payıyla olmalı", gp.Lines)
	}
	if g.Step == nil || g.Step.All != 6 {
		t.Errorf("adım %+v; iki iyileştiricili adım bir kez sayılmalı (6)", g.Step)
	}
}
