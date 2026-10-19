package profiler

import (
	"time"
)

// ============================================================================
// Profil biçimi (sürüm 1): canlı durum (profil.json) ve rapor (speed.json) aynı yapıdadır
// ============================================================================
//
// Anahtarlar dosya biçiminin parçasıdır (yazım dilinden bağımsız, çevrilmez; günlük
// biçimiyle aynı ilke). İşlemlerin "kod" alanı değişmez anahtardır, "ad" insana gösterilen
// addır (dil dosyasından; çevrilebilir). Süreler saniye, baytlar bayt, paylar 0..1'dir.
// Ayrıntı ve örnek: docs/mimari.md, "Canlı profil".

// FormatName ve FormatVersion dosyanın kimliğidir.
const (
	FormatName    = "tr-profil"
	FormatVersion = 1
)

// Snapshot bir programın profilinin anlık görüntüsüdür (canlı ya da son rapor).
type Snapshot struct {
	Format   string  `json:"biçim"`
	Version  int     `json:"sürüm"`
	Kind     string  `json:"tür"` // "canlı" ya da "rapor"
	Program  string  `json:"program"`
	Start    string  `json:"başlangıç"`
	Time     string  `json:"zaman"`
	Seconds  float64 `json:"süre_sn"`
	Done     bool    `json:"bitti"`
	ExitCode *int    `json:"çıkış_kodu,omitempty"`

	Step     *StepInfo     `json:"adım"` // null: program ilerleme(...) kullanmıyor
	Tensor   TensorInfo    `json:"tensör"`
	GPU      GPUInfo       `json:"gpu"`
	Sampling *SamplingInfo `json:"örnekleme"` // null: satır örneklemesi kapalı
	Memory   MemoryInfo    `json:"bellek"`
	GC       GCInfo        `json:"çöp_toplama"`
	// Window son birkaç saniyenin ölçüleridir (yalnızca canlı görüntüde).
	Window *WindowInfo `json:"son_pencere,omitempty"`
}

// StepInfo eğitim adımlarıdır (ilerleme(...).güncelle(...) çağrıları).
type StepInfo struct {
	Count       int64   `json:"sayı"`   // tamamlanan adım
	Total       int64   `json:"toplam"` // çubuğun toplamı (0: toplamsız)
	MsPerStep   float64 `json:"ms_adım"`
	StepsPerSec float64 `json:"adım_sn"`
}

// TensorInfo tensör işlemleridir (süre konakta ölçülür; bkz. tensor/counter.go).
type TensorInfo struct {
	Seconds float64  `json:"süre_sn"`
	Share   float64  `json:"pay"` // programın süresi içindeki payı
	Calls   int64    `json:"çağrı"`
	Bytes   int64    `json:"bayt"`
	Ops     []OpInfo `json:"işlemler"`
}

// OpInfo bir (işlem, yön, cihaz) üçlüsüdür; süreye göre büyükten küçüğe sıralıdır.
type OpInfo struct {
	Code      string   `json:"kod"`
	Name      string   `json:"ad"`
	Direction string   `json:"yön"`   // "ileri" ya da "geri"
	Device    string   `json:"cihaz"` // "cpu" ya da "gpu"
	Calls     int64    `json:"çağrı"`
	Measured  int64    `json:"ölçüm"` // süresi ölçülen çağrı (örnekleme)
	Seconds   float64  `json:"süre_sn"`
	Share     float64  `json:"pay"` // tensör süresi içindeki payı
	Bytes     int64    `json:"bayt"`
	MsPerStep *float64 `json:"ms_adım,omitempty"` // adım başına süre (adım varsa)
}

// GPUInfo GPU arka ucunun kendi ölçümleridir (internal/compute/gpu): çekirdeklerin cihazda
// geçen süresi (ölçüm açıkken) ve her GPU'nun bellek havuzu. GPU kullanılmadıysa Kullanildi
// yanlıştır ve listeler boştur.
type GPUInfo struct {
	Used     bool         `json:"kullanıldı"`
	Measured bool         `json:"ölçüm"` // çekirdek süreleri ölçülüyor
	Note     string       `json:"not,omitempty"`
	Seconds  float64      `json:"süre_sn"` // çekirdeklerin toplam GPU süresi
	Families []KernelInfo `json:"aileler,omitempty"`
	Kernels  []KernelInfo `json:"çekirdekler,omitempty"`
	Memory   []GPUMemory  `json:"bellek,omitempty"`
	// summary GPU arka ucunun kendi son raporudur (adım başına GPU süresi, boşluk, çekirdek
	// tabloları); yalnızca metin raporunda ve yalnızca ölçümü profil açtıysa (TR_GPU_PROFIL
	// ile açıksa arka uç onu program sonunda kendisi yazar).
	summary string
}

// KernelInfo bir GPU çekirdeği ya da çekirdek ailesidir (süreye göre sıralı).
type KernelInfo struct {
	Name    string  `json:"ad"`
	Calls   int64   `json:"çağrı"`
	Seconds float64 `json:"süre_sn"`
	Share   float64 `json:"pay"` // GPU süresi içindeki payı
}

// GPUMemory bir GPU'nun bellek havuzudur (bayt).
type GPUMemory struct {
	Device        int    `json:"cihaz"`
	Live          uint64 `json:"canlı_bayt"`    // tensörlere verilmiş
	Allocated     uint64 `json:"ayrılmış_bayt"` // sürücüden ayrılmış (havuz dahil)
	Pooled        uint64 `json:"havuzda_bayt"`
	PeakLive      uint64 `json:"tepe_canlı_bayt"`
	PeakAllocated uint64 `json:"tepe_ayrılmış_bayt"`
	GCCollect     int64  `json:"çöp_toplama"` // bellek için başlatılan çöp toplama
}

// SamplingInfo satır ve fonksiyon örneklemesidir.
type SamplingInfo struct {
	IntervalMs  float64        `json:"aralık_ms"`
	Samples     int64          `json:"örnek"`
	Seconds     float64        `json:"süre_sn"`
	TensorShare float64        `json:"tensör_payı"` // örneklerin tensör işlemindeki payı
	Lines       []LineInfo     `json:"satırlar"`
	Functions   []FunctionInfo `json:"fonksiyonlar"`
}

// LineInfo bir TR satırıdır (öz süre: o satırda çalışırken geçen süre).
type LineInfo struct {
	File        string  `json:"dosya"` // ana programın dışındaysa boş
	Line        int     `json:"satır"`
	Function    string  `json:"fonksiyon"` // üst düzeyde boş
	Text        string  `json:"metin,omitempty"`
	Seconds     float64 `json:"süre_sn"`
	Share       float64 `json:"pay"`
	TensorShare float64 `json:"tensör_payı"`
}

// FunctionInfo bir fonksiyonun öz süresidir (kendi satırlarında geçen süre).
type FunctionInfo struct {
	Name    string  `json:"ad"` // üst düzey kod boş
	Seconds float64 `json:"süre_sn"`
	Share   float64 `json:"pay"`
}

// MemoryInfo konak belleğidir (GPU belleği: GPUInfo.Bellek).
type MemoryInfo struct {
	GoHeap uint64 `json:"go_yığını_bayt"` // canlı Go nesneleri (CPU tensörleri dahil)
	GoSys  uint64 `json:"go_sistem_bayt"` // Go'nun işletim sisteminden aldığı
	// CPUTensorAlloc profil başından beri CPU tensör işlemlerinin ayırdığı bayttır
	// (çıktılar ve gradyanlar; havuzdan yeniden kullanılanlar dahil). Canlı CPU tensör baytı
	// ayrıca izlenmez: CPU tensörleri Go yığınındadır.
	CPUTensorAlloc int64 `json:"cpu_tensör_ayrılan_bayt"`
}

// GCInfo çöp toplayıcıdır.
type GCInfo struct {
	Count    uint32  `json:"sayı"`
	PauseSec float64 `json:"duraklama_sn"`
}

// WindowInfo son saniyelerin ölçüleridir (canlı tabloyla aynı).
type WindowInfo struct {
	Seconds     float64    `json:"süre_sn"`
	Step        int64      `json:"adım"`
	MsPerStep   float64    `json:"ms_adım"`
	StepsPerSec float64    `json:"adım_sn"`
	TensorShare float64    `json:"tensör_payı"`
	Ops         []OpInfo   `json:"işlemler"`
	Lines       []LineInfo `json:"satırlar,omitempty"`
	GCCount     uint32     `json:"çöp_toplama_sayı"`
	PauseSec    float64    `json:"duraklama_sn"`
	// CPUTensorAlloc pencerede CPU tensör işlemlerinin ayırdığı bayttır.
	CPUTensorAlloc int64 `json:"cpu_tensör_ayrılan_bayt"`
	// GPU çekirdeklerinin pencerede cihazda geçen süresi, pencere içindeki payı (GPU'nun
	// meşgul olduğu zamanın payı; birden çok GPU'da 1'i aşabilir) ve ailelere göre dağılımı.
	GPUSec      float64      `json:"gpu_süre_sn"`
	GPUShare    float64      `json:"gpu_payı"`
	GPUFamilies []KernelInfo `json:"gpu_aileler,omitempty"`
}

// sn süreyi saniyeye çevirir.
func sn(d time.Duration) float64 { return d.Seconds() }

// ratio a/b; b sıfırsa 0.
func ratio(a, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return a / b
}
