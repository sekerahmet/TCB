package tensor

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"sync/atomic"
)

// ============================================================================
// Ayırma sınırı: tensör verisi ayıran her yolun geçtiği merkezi denetim
// ============================================================================
//
// Go çalışma zamanı, işletim sisteminin karşılayamadığı bir ayırmada süreci
// yakalanamaz bir ölümcül hatayla sonlandırır; ör. iki küçük tensörün yayınlanması
// ([185000, 1, 1] + [4, 46000]) yüz GB'larca bellek isteyebilir. Bu yüzden her ayırma
// önce burada denetlenir:
//
//  1. Şeklin eleman sayısı taşma denetimli hesaplanır (negatif eksen ya da tam sayı
//     taşması hatadır; taşan bir çarpım sessizce küçük bir sayıya dönmez).
//  2. Toplam bayt (eleman × tür boyu) tek ayırmada makinenin fiziksel belleğini
//     aşamaz. Sınır kasıtlı olarak geniştir: meşru büyük tensörleri kesmez, yalnızca
//     hiçbir zaman karşılanamayacak istekleri durdurur.
//
// Sınır aşılınca kurtarılabilir bir panik (Türkçe mesaj) verilir; dil bağlaması onu
// 'dene/yakala' ile yakalanabilen bir hataya çevirir. Cihazdan bağımsız katman
// (zerosOf, emptyOf, oluşturucular, işlemlerin çıktıları) şekille denetler; CPU arka
// ucu ayrıca her depolama ayırmasını eleman sayısıyla denetler (iç tamponlar,
// gradyanlar, iyileştirici durumları).

// islem bir ayırmanın hangi işlemden geldiğini hata mesajında adlandırır.
type islem uint8

const (
	isOlusturma islem = iota
	isToplama
	isCikarma
	isCarpma
	isBolme
	isYayinliEleman
	isKarsilastirma
	isSecim
	isMatrisCarpimi
	isBirlestirme
	isYenidenSekil
	isKopya
	isTurDonusumu
	isElemanBazli
	isIndirgeme
	isSoftmax
	isNormallestirme
	isGomme
	isDikkat
	isGradyan
	isArkaUc
)

var islemAdlari = [...]string{
	isOlusturma:      "tensör oluşturma",
	isToplama:        "toplama",
	isCikarma:        "çıkarma",
	isCarpma:         "çarpma",
	isBolme:          "bölme",
	isYayinliEleman:  "eleman bazlı işlem",
	isKarsilastirma:  "karşılaştırma",
	isSecim:          "koşullu seçim",
	isMatrisCarpimi:  "matris çarpımı",
	isBirlestirme:    "birleştirme",
	isYenidenSekil:   "yeniden şekillendirme",
	isKopya:          "kopya",
	isTurDonusumu:    "tür dönüşümü",
	isElemanBazli:    "eleman bazlı işlem",
	isIndirgeme:      "indirgeme",
	isSoftmax:        "softmax",
	isNormallestirme: "normalleştirme",
	isGomme:          "gömme",
	isDikkat:         "dikkat",
	isGradyan:        "gradyan",
	isArkaUc:         "arka uç",
}

// binopIslem ikili işlemlerin (yayınlamalı) hata mesajı adları.
var binopIslem = [...]islem{OpAdd: isToplama, OpSub: isCikarma, OpMul: isCarpma, OpDiv: isBolme,
	OpAtan2: isYayinliEleman, OpLt: isKarsilastirma, OpGt: isKarsilastirma, OpLe: isKarsilastirma, OpGe: isKarsilastirma}

func (i islem) String() string { return islemAdlari[i] }

// yayinli işlemin çıktı şeklinin yayınlamayla üretilip üretilmediği.
func (i islem) yayinli() bool { return i >= isToplama && i <= isSecim }

// varsayilanBellek fiziksel bellek okunamazsa varsayılan sınır (64 GB).
const varsayilanBellek = 64 << 30

// sinirBayt tek ayırmanın en fazla baytı; 0 henüz okunmadığını gösterir. İlk
// kullanımda fiziksel bellekten bir kez okunur. Testler küçültüp geri yükleyebilir.
var sinirBayt atomic.Int64

// ayirmaSiniri tek ayırmanın en fazla baytını verir (fiziksel bellek).
func ayirmaSiniri() int64 {
	if s := sinirBayt.Load(); s > 0 {
		return s
	}
	return sinirOku()
}

// sinirOku sınırı fiziksel bellekten okuyup saklar (ayirmaSiniri'nin soğuk yolu; sıcak
// yol satır içine alınabilsin diye ayrıdır).
func sinirOku() int64 {
	b, ok := fizikselBellek()
	if !ok || b == 0 || b > math.MaxInt64/8 {
		b = varsayilanBellek
	}
	sinirBayt.Store(int64(b))
	return int64(b)
}

// elemanSayisi şeklin eleman sayısını taşma denetimli verir. Negatif eksen paniktir;
// sıfır uzunluklu bir eksen varsa sonuç 0'dır (öbür eksenler ne kadar büyük olursa
// olsun); çarpım int'e sığmıyorsa tasma doğrudur.
func elemanSayisi(shape []int) (n int, tasma bool) {
	u, sifir := uint64(1), false
	for _, d := range shape {
		if d <= 0 {
			if d < 0 {
				panic(fmt.Sprintf("tensor: negatif boyut %v", shape))
			}
			sifir = true
			continue
		}
		if !tasma {
			hi, lo := bits.Mul64(u, uint64(d))
			if hi != 0 || lo > math.MaxInt {
				tasma = true
			} else {
				u = lo
			}
		}
	}
	if sifir {
		return 0, false
	}
	if tasma {
		return 0, true
	}
	return int(u), false
}

// sigar n elemanlık dt türü ayırmanın sınır içinde olup olmadığı (n ≥ 0).
func sigar(n int, dt DType) bool {
	eb := uint64(dt.Bits())
	if eb == 0 {
		eb = 32
	}
	hi, lo := bits.Mul64(uint64(n), eb)
	return hi == 0 && lo <= uint64(ayirmaSiniri())*8
}

// ayirmaBoyu, is işleminin dt türünde shape şeklinde ayıracağı tensörün eleman
// sayısını verir; şekil taşıyorsa ya da tensör fiziksel belleğe sığmıyorsa Türkçe
// bir hatayla panik eder. Tensör verisi ayıran her yol bu denetimden geçer.
func ayirmaBoyu(is islem, shape []int, dt DType) int {
	n, tasma := elemanSayisi(shape)
	if tasma || !sigar(n, dt) {
		panic(sinirHatasi(is, shape, dt, tasma))
	}
	return n
}

// elemanDenetle şekli bilinmeyen n elemanlık bir depolama ayırmasını denetler ve n'yi
// verir (arka uçların son savunması: gradyanlar, iç tamponlar, iyileştirici durumları).
func elemanDenetle(n int, dt DType) int {
	if n < 0 {
		panic(fmt.Sprintf("tensor: arka uç: negatif eleman sayısı %d", n))
	}
	if !sigar(n, dt) {
		panic(SizeError(n, dt, ayirmaSiniri(), "bu makinede"))
	}
	return n
}

// ayirmaBoyuOn ayirmaBoyu'na ek olarak arka ucun kendi bellek sınırını (MemoryLimiter,
// ör. GPU belleği) denetler; şekli bilinen ve bir arka uçta ayrılan her tensör buradan
// geçer.
func ayirmaBoyuOn(be Backend, is islem, shape []int, dt DType) int {
	n := ayirmaBoyu(is, shape, dt)
	if l, ok := be.(MemoryLimiter); ok {
		if bayt := uint64(dt.StorageBytes(n)); bayt > uint64(l.MemoryLimit()) {
			panic(sinirHatasiYer(is, shape, dt, false, l.MemoryLimit(), l.MemoryPlace()))
		}
	}
	sayacBayt(be, dt.StorageBytes(n))
	return n
}

// sinirHatasi sınırı aşan ayırmanın mesajını kurar (soğuk yol).
func sinirHatasi(is islem, shape []int, dt DType, tasma bool) string {
	return sinirHatasiYer(is, shape, dt, tasma, ayirmaSiniri(), "bu makinede")
}

// SizeError, arka uçların şekli bilinmeyen bir ayırmayı reddederken verdiği Türkçe
// mesajdır (n eleman, dt türü; sınır ve yer: "bu GPU'da"). Mesaj cihazdan bağımsız
// katmanın mesajlarıyla aynı biçimdedir.
func SizeError(n int, dt DType, sinir int64, yer string) string {
	return fmt.Sprintf("tensor: arka uç: %s elemanlık %s tampon %s bellek ister; %s %s var",
		sayiMetni(float64(n)), dt, baytMetni(float64(n)*float64(max(dt.Bits(), 1))/8), yer, baytMetni(float64(sinir)))
}

// sinirHatasiYer sınırı aşan ayırmanın mesajını, sınır ve yer ("bu makinede", "bu
// GPU'da") verilerek kurar.
func sinirHatasiYer(is islem, shape []int, dt DType, tasma bool, sinir int64, yer string) string {
	eleman := 1.0
	for _, d := range shape {
		eleman *= float64(d)
	}
	bayt := eleman * float64(max(dt.Bits(), 1)) / 8
	var sb strings.Builder
	sb.WriteString("tensor: ")
	sb.WriteString(is.String())
	if is == isOlusturma {
		sb.WriteString(": istenen tensör ")
	} else {
		sb.WriteString(": sonuç tensörü ")
	}
	sb.WriteString(sekilYazi(shape))
	sb.WriteString(" şeklinde")
	if tasma {
		fmt.Fprintf(&sb, "; eleman sayısı (yaklaşık %s) tam sayı sınırını aşıyor", sayiMetni(eleman))
	}
	fmt.Fprintf(&sb, ", %s bellek ister; %s %s var. Şekilleri denetleyin", baytMetni(bayt), yer, baytMetni(float64(sinir)))
	if is.yayinli() {
		sb.WriteString(" (yayınlama beklenmedik bir boyut üretmiş olabilir)")
	}
	sb.WriteString(".")
	return sb.String()
}

// sekilYazi bir şekli "[2, 3]" biçiminde yazar.
func sekilYazi(shape []int) string {
	p := make([]string, len(shape))
	for i, d := range shape {
		p[i] = strconv.Itoa(d)
	}
	return "[" + strings.Join(p, ", ") + "]"
}

// baytMetni bayt sayısını insan okunur birimle (1024 tabanlı, ondalık virgülle) yazar.
func baytMetni(b float64) string {
	birimler := [...]string{"bayt", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	i := 0
	for b >= 1024 && i < len(birimler)-1 {
		b /= 1024
		i++
	}
	if i == 0 {
		return strconv.FormatFloat(b, 'f', 0, 64) + " bayt"
	}
	if b >= 1e6 {
		return sayiMetni(b) + " " + birimler[i]
	}
	return strings.Replace(strconv.FormatFloat(b, 'f', 1, 64), ".", ",", 1) + " " + birimler[i]
}

// sayiMetni büyük bir sayıyı yaklaşık yazar: küçükse tam, büyükse "1,2 × 10^24".
func sayiMetni(x float64) string {
	if x < 1e15 {
		return strconv.FormatFloat(x, 'f', 0, 64)
	}
	us := math.Floor(math.Log10(x))
	m := x / math.Pow(10, us)
	return strings.Replace(strconv.FormatFloat(m, 'f', 1, 64), ".", ",", 1) + " × 10^" + strconv.Itoa(int(us))
}
