package profiler

import (
	"time"
)

// ============================================================================
// Profil biçimi (sürüm 4): canlı durum (profil.json) ve rapor (speed.json) aynı yapıdadır
// ============================================================================
//
// Anahtarlar dosya biçiminin parçasıdır (yazım dilinden bağımsız, çevrilmez; günlük
// biçimiyle aynı ilke). İşlemlerin "kod" alanı değişmez anahtardır, "ad" insana gösterilen
// addır (dil dosyasından; çevrilebilir). Süreler saniye, baytlar bayt, paylar 0..1'dir.
// Ayrıntı ve örnek: docs/architecture.md, "Canlı profil".

// FormatName ve FormatVersion dosyanın kimliğidir.
const (
	FormatName    = "tr-profil"
	FormatVersion = 4
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
	// gpuPeak tepe GPU belleğini ayıran satırlar ve tepe (yalnızca son raporda ve satır
	// örneklemesi açıkken; metin raporu içindir, dosya biçiminde yoktur).
	gpuPeak      []PeakLine
	gpuPeakTotal uint64
}

// StepInfo eğitim adımlarıdır (ilerleme(...).güncelle(...) çağrıları).
type StepInfo struct {
	Count int64 `json:"sayı"`   // çubuğun tamamlanan adımı
	Total int64 `json:"toplam"` // çubuğun toplamı (0: toplamsız)
	// All profil başından beri bütün çubuklarda tamamlanan adımlardır (her epoch'ta yeni bir
	// çubuk açılınca Count baştan başlar, All birikir); adım başına süreler buna bölünür.
	All         int64   `json:"tüm_adım"`
	MsPerStep   float64 `json:"ms_adım"`
	StepsPerSec float64 `json:"adım_sn"`
	// Source adım sınırının kaynağıdır: "ilerleme_cubugu" (güncelle) ya da "iyilestirici" (program
	// ilerleme kullanmıyorsa ilk adım atan iyileştiricinin adımı). Sürüm 4.
	Source string `json:"kaynak"`
	// Adım başına evreler (konak süresi, sürüm 4): ileri; geri (geri yayılımlar); sonrası
	// (son geri yayılımdan adım sınırına: iyileştirici, kırpma, günlük).
	ForwardMs  float64 `json:"ileri_ms"`
	BackwardMs float64 `json:"geri_ms"`
	AfterMs    float64 `json:"sonra_ms"`
}

// TensorInfo tensör işlemleridir (süre konakta ölçülür; bkz. tensor/counter.go).
type TensorInfo struct {
	Seconds float64  `json:"süre_sn"`
	Share   float64  `json:"pay"` // programın süresi içindeki payı
	Calls   int64    `json:"çağrı"`
	Bytes   int64    `json:"bayt"`
	Ops     []OpInfo `json:"işlemler"`
	// Implicit işlemlerin içinde arka uçta yapılan kopya, dönüşüm ve eklemelerdir (sürüm 2).
	Implicit []ImplicitInfo `json:"örtük"`
}

// ImplicitInfo bir (örtük tür, üst işlem, yön, cihaz) dörtlüsüdür; elemana göre büyükten
// küçüğe sıralıdır (GPU'da süre yoktur, eleman iki cihazda da kesindir).
type ImplicitInfo struct {
	Code       string  `json:"kod"`     // örtük türün değişmez anahtarı (ör. "adimli_kopya")
	Parent     string  `json:"üst_kod"` // üst işlemin değişmez anahtarı (ör. "matris_carpimi")
	Name       string  `json:"ad"`      // örtük türün insana gösterilen adı (ör. "adımlı kopya")
	ParentName string  `json:"üst_ad"`  // üst işlemin insana gösterilen adı (ör. "geri · @")
	Direction  string  `json:"yön"`     // üst işlemin yönü: "ileri" ya da "geri"
	Device     string  `json:"cihaz"`
	Calls      int64   `json:"çağrı"`
	Elements   int64   `json:"eleman"`
	Measured   int64   `json:"ölçüm"`   // süresi ölçülen çağrı (yalnızca CPU'da büyük kopyalar)
	Seconds    float64 `json:"süre_sn"` // ölçülen çağrıların süresi
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
	File        string  `json:"dosya"` // ana program ya da 'aktar' ile yüklenen modül
	Line        int     `json:"satır"`
	Function    string  `json:"fonksiyon"` // üst düzeyde boş
	Text        string  `json:"metin,omitempty"`
	Seconds     float64 `json:"süre_sn"`
	Share       float64 `json:"pay"`
	TensorShare float64 `json:"tensör_payı"`
	// BackwardShare satırın süresinden geri hesaba düşen paydır: satırın ürettiği düğümlerin
	// geri yayılımı (sürüm 3; geri_yayılım(...) satırına değil, düğümü üreten satıra yazılır).
	BackwardShare float64 `json:"geri_payı"`
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
