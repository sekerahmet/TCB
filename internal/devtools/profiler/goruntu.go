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

// BicimAdi ve BicimSurumu dosyanın kimliğidir.
const (
	BicimAdi    = "tr-profil"
	BicimSurumu = 1
)

// Goruntu bir programın profilinin anlık görüntüsüdür (canlı ya da son rapor).
type Goruntu struct {
	Bicim     string  `json:"biçim"`
	Surum     int     `json:"sürüm"`
	Tur       string  `json:"tür"` // "canlı" ya da "rapor"
	Program   string  `json:"program"`
	Baslangic string  `json:"başlangıç"`
	Zaman     string  `json:"zaman"`
	SureSn    float64 `json:"süre_sn"`
	Bitti     bool    `json:"bitti"`
	CikisKodu *int    `json:"çıkış_kodu,omitempty"`

	Adim      *AdimBilgisi      `json:"adım"` // null: program ilerleme(...) kullanmıyor
	Tensor    TensorBilgisi     `json:"tensör"`
	GPU       GPUBilgisi        `json:"gpu"`
	Ornekleme *OrneklemeBilgisi `json:"örnekleme"` // null: satır örneklemesi kapalı
	Bellek    BellekBilgisi     `json:"bellek"`
	Cop       CopBilgisi        `json:"çöp_toplama"`
	// Pencere son birkaç saniyenin ölçüleridir (yalnızca canlı görüntüde).
	Pencere *PencereBilgisi `json:"son_pencere,omitempty"`
}

// AdimBilgisi eğitim adımlarıdır (ilerleme(...).güncelle(...) çağrıları).
type AdimBilgisi struct {
	Sayi   int64   `json:"sayı"`   // tamamlanan adım
	Toplam int64   `json:"toplam"` // çubuğun toplamı (0: toplamsız)
	MsAdim float64 `json:"ms_adım"`
	AdimSn float64 `json:"adım_sn"`
}

// TensorBilgisi tensör işlemleridir (süre konakta ölçülür; bkz. tensor/sayac.go).
type TensorBilgisi struct {
	SureSn   float64        `json:"süre_sn"`
	Pay      float64        `json:"pay"` // programın süresi içindeki payı
	Cagri    int64          `json:"çağrı"`
	Bayt     int64          `json:"bayt"`
	Islemler []IslemBilgisi `json:"işlemler"`
}

// IslemBilgisi bir (işlem, yön, cihaz) üçlüsüdür; süreye göre büyükten küçüğe sıralıdır.
type IslemBilgisi struct {
	Kod    string   `json:"kod"`
	Ad     string   `json:"ad"`
	Yon    string   `json:"yön"`   // "ileri" ya da "geri"
	Cihaz  string   `json:"cihaz"` // "cpu" ya da "gpu"
	Cagri  int64    `json:"çağrı"`
	Olcum  int64    `json:"ölçüm"` // süresi ölçülen çağrı (örnekleme)
	SureSn float64  `json:"süre_sn"`
	Pay    float64  `json:"pay"` // tensör süresi içindeki payı
	Bayt   int64    `json:"bayt"`
	MsAdim *float64 `json:"ms_adım,omitempty"` // adım başına süre (adım varsa)
}

// GPUBilgisi GPU arka ucunun kendi ölçümleridir (internal/compute/gpu): çekirdeklerin cihazda
// geçen süresi (ölçüm açıkken) ve her GPU'nun bellek havuzu. GPU kullanılmadıysa Kullanildi
// yanlıştır ve listeler boştur.
type GPUBilgisi struct {
	Kullanildi  bool              `json:"kullanıldı"`
	Olcum       bool              `json:"ölçüm"` // çekirdek süreleri ölçülüyor
	Not         string            `json:"not,omitempty"`
	SureSn      float64           `json:"süre_sn"` // çekirdeklerin toplam GPU süresi
	Aileler     []CekirdekBilgisi `json:"aileler,omitempty"`
	Cekirdekler []CekirdekBilgisi `json:"çekirdekler,omitempty"`
	Bellek      []GPUBellegi      `json:"bellek,omitempty"`
	// ozet GPU arka ucunun kendi son raporudur (adım başına GPU süresi, boşluk, çekirdek
	// tabloları); yalnızca metin raporunda ve yalnızca ölçümü profil açtıysa (TR_GPU_PROFIL
	// ile açıksa arka uç onu program sonunda kendisi yazar).
	ozet string
}

// CekirdekBilgisi bir GPU çekirdeği ya da çekirdek ailesidir (süreye göre sıralı).
type CekirdekBilgisi struct {
	Ad     string  `json:"ad"`
	Cagri  int64   `json:"çağrı"`
	SureSn float64 `json:"süre_sn"`
	Pay    float64 `json:"pay"` // GPU süresi içindeki payı
}

// GPUBellegi bir GPU'nun bellek havuzudur (bayt).
type GPUBellegi struct {
	Cihaz        int    `json:"cihaz"`
	Canli        uint64 `json:"canlı_bayt"`    // tensörlere verilmiş
	Ayrilmis     uint64 `json:"ayrılmış_bayt"` // sürücüden ayrılmış (havuz dahil)
	Havuzda      uint64 `json:"havuzda_bayt"`
	TepeCanli    uint64 `json:"tepe_canlı_bayt"`
	TepeAyrilmis uint64 `json:"tepe_ayrılmış_bayt"`
	CopToplama   int64  `json:"çöp_toplama"` // bellek için başlatılan çöp toplama
}

// OrneklemeBilgisi satır ve fonksiyon örneklemesidir.
type OrneklemeBilgisi struct {
	AralikMs     float64          `json:"aralık_ms"`
	Ornek        int64            `json:"örnek"`
	SureSn       float64          `json:"süre_sn"`
	TensorPayi   float64          `json:"tensör_payı"` // örneklerin tensör işlemindeki payı
	Satirlar     []SatirBilgisi   `json:"satırlar"`
	Fonksiyonlar []FonksiyonBilgi `json:"fonksiyonlar"`
}

// SatirBilgisi bir TR satırıdır (öz süre: o satırda çalışırken geçen süre).
type SatirBilgisi struct {
	Dosya      string  `json:"dosya"` // ana programın dışındaysa boş
	Satir      int     `json:"satır"`
	Fonksiyon  string  `json:"fonksiyon"` // üst düzeyde boş
	Metin      string  `json:"metin,omitempty"`
	SureSn     float64 `json:"süre_sn"`
	Pay        float64 `json:"pay"`
	TensorPayi float64 `json:"tensör_payı"`
}

// FonksiyonBilgi bir fonksiyonun öz süresidir (kendi satırlarında geçen süre).
type FonksiyonBilgi struct {
	Ad     string  `json:"ad"` // üst düzey kod boş
	SureSn float64 `json:"süre_sn"`
	Pay    float64 `json:"pay"`
}

// BellekBilgisi konak belleğidir (GPU belleği: GPUBilgisi.Bellek).
type BellekBilgisi struct {
	GoYigini uint64 `json:"go_yığını_bayt"` // canlı Go nesneleri (CPU tensörleri dahil)
	GoSistem uint64 `json:"go_sistem_bayt"` // Go'nun işletim sisteminden aldığı
	// CPUTensorAyrilan profil başından beri CPU tensör işlemlerinin ayırdığı bayttır
	// (çıktılar ve gradyanlar; havuzdan yeniden kullanılanlar dahil). Canlı CPU tensör baytı
	// ayrıca izlenmez: CPU tensörleri Go yığınındadır.
	CPUTensorAyrilan int64 `json:"cpu_tensör_ayrılan_bayt"`
}

// CopBilgisi çöp toplayıcıdır.
type CopBilgisi struct {
	Sayi        uint32  `json:"sayı"`
	DuraklamaSn float64 `json:"duraklama_sn"`
}

// PencereBilgisi son saniyelerin ölçüleridir (canlı tabloyla aynı).
type PencereBilgisi struct {
	SureSn      float64        `json:"süre_sn"`
	Adim        int64          `json:"adım"`
	MsAdim      float64        `json:"ms_adım"`
	AdimSn      float64        `json:"adım_sn"`
	TensorPayi  float64        `json:"tensör_payı"`
	Islemler    []IslemBilgisi `json:"işlemler"`
	Satirlar    []SatirBilgisi `json:"satırlar,omitempty"`
	CopSayi     uint32         `json:"çöp_toplama_sayı"`
	DuraklamaSn float64        `json:"duraklama_sn"`
	// CPUTensorAyrilan pencerede CPU tensör işlemlerinin ayırdığı bayttır.
	CPUTensorAyrilan int64 `json:"cpu_tensör_ayrılan_bayt"`
	// GPU çekirdeklerinin pencerede cihazda geçen süresi, pencere içindeki payı (GPU'nun
	// meşgul olduğu zamanın payı; birden çok GPU'da 1'i aşabilir) ve ailelere göre dağılımı.
	GPUSureSn  float64           `json:"gpu_süre_sn"`
	GPUPayi    float64           `json:"gpu_payı"`
	GPUAileler []CekirdekBilgisi `json:"gpu_aileler,omitempty"`
}

// sn süreyi saniyeye çevirir.
func sn(d time.Duration) float64 { return d.Seconds() }

// oran a/b; b sıfırsa 0.
func oran(a, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return a / b
}
