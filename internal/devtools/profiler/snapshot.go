package profiler

import (
	"time"
)

// ============================================================================
// Profil biçimi (sürüm 6): canlı durum (profile.json) ve rapor (speed.json) aynı yapıdadır
// ============================================================================
//
// Anahtarlar ve sabit değerler ("live", "forward" ...) dosya biçiminin parçasıdır: dilden
// bağımsız İngilizce tanımlayıcılardır, çevrilmez (dil tanımı 17.3; günlük biçimiyle aynı
// ilke). İşlemlerin "code" alanı değişmez anahtardır, "name" insana gösterilen addır. Süreler saniye, baytlar bayt, paylar 0..1'dir.
// Ayrıntı ve örnek: docs/architecture.md, "Canlı profil".

// FormatName ve FormatVersion dosyanın kimliğidir.
const (
	FormatName    = "tr-profile"
	FormatVersion = 6
)

// Snapshot bir programın profilinin anlık görüntüsüdür (canlı ya da son rapor).
type Snapshot struct {
	Format   string  `json:"format"`
	Version  int     `json:"version"`
	Kind     string  `json:"kind"` // "live" ya da "report"
	Program  string  `json:"program"`
	Start    string  `json:"start"`
	Time     string  `json:"time"`
	Seconds  float64 `json:"seconds"`
	Done     bool    `json:"done"`
	ExitCode *int    `json:"exit_code,omitempty"`

	Step     *StepInfo     `json:"step"` // null: program ilerleme(...) kullanmıyor
	Tensor   TensorInfo    `json:"tensor"`
	GPU      GPUInfo       `json:"gpu"`
	Sampling *SamplingInfo `json:"sampling"` // null: satır örneklemesi kapalı
	Memory   MemoryInfo    `json:"memory"`
	GC       GCInfo        `json:"gc"`
	// Window son birkaç saniyenin ölçüleridir (yalnızca canlı görüntüde).
	Window *WindowInfo `json:"window,omitempty"`
	// gpuPeak tepe GPU belleğini ayıran satırlar ve tepe (yalnızca son raporda ve satır
	// örneklemesi açıkken; metin raporu içindir, dosya biçiminde yoktur).
	gpuPeak      []PeakLine
	gpuPeakTotal uint64
}

// StepInfo eğitim adımlarıdır (ilerleme(...).güncelle(...) çağrıları).
type StepInfo struct {
	Count int64 `json:"count"` // çubuğun tamamlanan adımı
	Total int64 `json:"total"` // çubuğun toplamı (0: toplamsız)
	// All profil başından beri bütün çubuklarda tamamlanan adımlardır (her epoch'ta yeni bir
	// çubuk açılınca Count baştan başlar, All birikir); adım başına süreler buna bölünür.
	All         int64   `json:"all_steps"`
	MsPerStep   float64 `json:"ms_per_step"`
	StepsPerSec float64 `json:"steps_per_second"`
	// Source adım sınırının kaynağıdır: "progress_bar" (güncelle) ya da "optimizer" (program
	// ilerleme kullanmıyorsa ilk adım atan iyileştiricinin adımı). Sürüm 4.
	Source string `json:"source"`
	// Adım başına evreler (konak süresi, sürüm 4): ileri; geri (geri yayılımlar); sonrası
	// (son geri yayılımdan adım sınırına: iyileştirici, kırpma, günlük).
	ForwardMs  float64 `json:"forward_ms"`
	BackwardMs float64 `json:"backward_ms"`
	AfterMs    float64 `json:"after_ms"`
}

// TensorInfo tensör işlemleridir (süre konakta ölçülür; bkz. tensor/counter.go).
type TensorInfo struct {
	Seconds float64  `json:"seconds"`
	Share   float64  `json:"share"` // programın süresi içindeki payı
	Calls   int64    `json:"calls"`
	Bytes   int64    `json:"bytes"`
	Ops     []OpInfo `json:"ops"`
	// Implicit işlemlerin içinde arka uçta yapılan kopya, dönüşüm ve eklemelerdir (sürüm 2).
	Implicit []ImplicitInfo `json:"implicit"`
}

// ImplicitInfo bir (örtük tür, üst işlem, yön, cihaz) dörtlüsüdür; elemana göre büyükten
// küçüğe sıralıdır (GPU'da süre yoktur, eleman iki cihazda da kesindir).
type ImplicitInfo struct {
	Code       string  `json:"code"`        // örtük türün değişmez anahtarı (ör. "strided_copy")
	Parent     string  `json:"parent_code"` // üst işlemin değişmez anahtarı (ör. "matmul")
	Name       string  `json:"name"`        // örtük türün insana gösterilen adı (ör. "adımlı kopya")
	ParentName string  `json:"parent_name"` // üst işlemin insana gösterilen adı (ör. "geri · @")
	Direction  string  `json:"direction"`   // üst işlemin yönü: "forward" ya da "backward"
	Device     string  `json:"device"`
	Calls      int64   `json:"calls"`
	Elements   int64   `json:"elements"`
	Measured   int64   `json:"measured"` // süresi ölçülen çağrı (yalnızca CPU'da büyük kopyalar)
	Seconds    float64 `json:"seconds"`  // ölçülen çağrıların süresi
}

// OpInfo bir (işlem, yön, cihaz) üçlüsüdür; süreye göre büyükten küçüğe sıralıdır.
type OpInfo struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Direction string   `json:"direction"` // "forward" ya da "backward"
	Device    string   `json:"device"`    // "cpu" ya da "gpu"
	Calls     int64    `json:"calls"`
	Measured  int64    `json:"measured"` // süresi ölçülen çağrı (örnekleme)
	Seconds   float64  `json:"seconds"`
	Share     float64  `json:"share"` // tensör süresi içindeki payı
	Bytes     int64    `json:"bytes"`
	MsPerStep *float64 `json:"ms_per_step,omitempty"` // adım başına süre (adım varsa)
}

// GPUInfo GPU arka ucunun kendi ölçümleridir (internal/compute/gpu): çekirdeklerin cihazda
// geçen süresi (ölçüm açıkken) ve her GPU'nun bellek havuzu. GPU kullanılmadıysa Kullanildi
// yanlıştır ve listeler boştur.
type GPUInfo struct {
	Used     bool         `json:"used"`
	Measured bool         `json:"measured"` // çekirdek süreleri ölçülüyor
	Note     string       `json:"note,omitempty"`
	Seconds  float64      `json:"seconds"` // çekirdeklerin toplam GPU süresi
	Families []KernelInfo `json:"families,omitempty"`
	Kernels  []KernelInfo `json:"kernels,omitempty"`
	// Ops ve Lines çekirdeklerin cihaz süresinin onları başlatan tensör işlemine ve TR satırına
	// göre dağılımıdır (sürüm 5; geri yöndeki çekirdekler düğümü üreten ileri satırdadır).
	Ops    []GPUOpInfo   `json:"ops,omitempty"`
	Lines  []GPULineInfo `json:"lines,omitempty"`
	Memory []GPUMemory   `json:"memory,omitempty"`
	// summary GPU arka ucunun kendi son raporudur (adım başına GPU süresi, boşluk, çekirdek
	// tabloları); yalnızca metin raporunda ve yalnızca ölçümü profil açtıysa (TR_GPU_PROFIL
	// ile açıksa arka uç onu program sonunda kendisi yazar).
	summary string
}

// GPUOpInfo bir (işlem, yön) ikilisinin başlattığı çekirdeklerin cihaz süresidir.
type GPUOpInfo struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Direction string  `json:"direction"`
	Calls     int64   `json:"calls"`
	Seconds   float64 `json:"seconds"`
	Share     float64 `json:"share"` // GPU süresi içindeki payı
}

// GPULineInfo bir TR satırının başlattığı çekirdeklerin cihaz süresidir.
type GPULineInfo struct {
	File          string  `json:"file"`
	Line          int     `json:"line"`
	Function      string  `json:"function"`
	Text          string  `json:"text,omitempty"`
	Seconds       float64 `json:"seconds"`
	Share         float64 `json:"share"`          // GPU süresi içindeki payı
	BackwardShare float64 `json:"backward_share"` // satırın GPU süresinden geri hesaba düşen
}

// KernelInfo bir GPU çekirdeği ya da çekirdek ailesidir (süreye göre sıralı).
type KernelInfo struct {
	Name    string  `json:"name"`
	Calls   int64   `json:"calls"`
	Seconds float64 `json:"seconds"`
	Share   float64 `json:"share"` // GPU süresi içindeki payı
}

// GPUMemory bir GPU'nun bellek havuzudur (bayt).
type GPUMemory struct {
	Device        int    `json:"device"`
	Live          uint64 `json:"live_bytes"`      // tensörlere verilmiş
	Allocated     uint64 `json:"allocated_bytes"` // sürücüden ayrılmış (havuz dahil)
	Pooled        uint64 `json:"pooled_bytes"`
	PeakLive      uint64 `json:"peak_live_bytes"`
	PeakAllocated uint64 `json:"peak_allocated_bytes"`
	GCCollect     int64  `json:"gc_collections"` // bellek için başlatılan çöp toplama
}

// SamplingInfo satır ve fonksiyon örneklemesidir.
type SamplingInfo struct {
	IntervalMs  float64        `json:"interval_ms"`
	Samples     int64          `json:"samples"`
	Seconds     float64        `json:"seconds"`
	TensorShare float64        `json:"tensor_share"` // örneklerin tensör işlemindeki payı
	Lines       []LineInfo     `json:"lines"`
	Functions   []FunctionInfo `json:"functions"`
}

// LineInfo bir TR satırıdır (öz süre: o satırda çalışırken geçen süre).
type LineInfo struct {
	File        string  `json:"file"` // ana program ya da 'aktar' ile yüklenen modül
	Line        int     `json:"line"`
	Function    string  `json:"function"` // üst düzeyde boş
	Text        string  `json:"text,omitempty"`
	Seconds     float64 `json:"seconds"`
	Share       float64 `json:"share"`
	TensorShare float64 `json:"tensor_share"`
	// BackwardShare satırın süresinden geri hesaba düşen paydır: satırın ürettiği düğümlerin
	// geri yayılımı (sürüm 3; geri_yayılım(...) satırına değil, düğümü üreten satıra yazılır).
	BackwardShare float64 `json:"backward_share"`
}

// PeakLine tepe GPU belleği anında bir satırın ayırdığı canlı bellektir. Yalnızca metin
// raporundadır (dosya biçimine sürüm 2 ile girer).
type PeakLine struct {
	Line   LineInfo
	DType  string // dildeki tür adı
	Bytes  uint64
	Blocks uint64  // canlı tensör deposu sayısı
	Share  float64 // tepenin payı
}

// FunctionInfo bir fonksiyonun öz süresidir (kendi satırlarında geçen süre).
type FunctionInfo struct {
	Name    string  `json:"name"` // üst düzey kod boş
	Seconds float64 `json:"seconds"`
	Share   float64 `json:"share"`
}

// MemoryInfo konak belleğidir (GPU belleği: GPUInfo.Bellek).
type MemoryInfo struct {
	GoHeap uint64 `json:"go_heap_bytes"` // canlı Go nesneleri (CPU tensörleri dahil)
	GoSys  uint64 `json:"go_sys_bytes"`  // Go'nun işletim sisteminden aldığı
	// CPUTensorAlloc profil başından beri CPU tensör işlemlerinin ayırdığı bayttır
	// (çıktılar ve gradyanlar; havuzdan yeniden kullanılanlar dahil). Canlı CPU tensör baytı
	// ayrıca izlenmez: CPU tensörleri Go yığınındadır.
	CPUTensorAlloc int64 `json:"cpu_tensor_alloc_bytes"`
}

// GCInfo çöp toplayıcıdır.
type GCInfo struct {
	Count    uint32  `json:"count"`
	PauseSec float64 `json:"pause_seconds"`
}

// WindowInfo son saniyelerin ölçüleridir (canlı tabloyla aynı).
type WindowInfo struct {
	Seconds     float64    `json:"seconds"`
	Step        int64      `json:"step"`
	MsPerStep   float64    `json:"ms_per_step"`
	StepsPerSec float64    `json:"steps_per_second"`
	TensorShare float64    `json:"tensor_share"`
	Ops         []OpInfo   `json:"ops"`
	Lines       []LineInfo `json:"lines,omitempty"`
	GCCount     uint32     `json:"gc_count"`
	PauseSec    float64    `json:"pause_seconds"`
	// CPUTensorAlloc pencerede CPU tensör işlemlerinin ayırdığı bayttır.
	CPUTensorAlloc int64 `json:"cpu_tensor_alloc_bytes"`
	// GPU çekirdeklerinin pencerede cihazda geçen süresi, pencere içindeki payı (GPU'nun
	// meşgul olduğu zamanın payı; birden çok GPU'da 1'i aşabilir) ve ailelere göre dağılımı.
	GPUSec      float64      `json:"gpu_seconds"`
	GPUShare    float64      `json:"gpu_share"`
	GPUFamilies []KernelInfo `json:"gpu_families,omitempty"`
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
