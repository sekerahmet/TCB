package tensor

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// opKind bir ayırmanın hangi işlemden geldiğini hata mesajında adlandırır.
type opKind uint8

const (
	isCreate opKind = iota
	isAdd
	isSub
	isMul
	isDiv
	isBroadcastElem
	isCompare
	isSelect
	isMatMul
	isConcat
	isReshape
	isCopy
	isCast
	isElementwise
	isReduce
	isSoftmaxOp
	isNormalize
	isEmbedding
	isAttention
	isGrad
	isBackend
)

var opNames = [...]messages.Code{
	isCreate:        messages.H3089,
	isAdd:           messages.H6119,
	isSub:           messages.H6120,
	isMul:           messages.H6121,
	isDiv:           messages.H6122,
	isBroadcastElem: messages.H3090,
	isCompare:       messages.H3091,
	isSelect:        messages.H3092,
	isMatMul:        messages.H3093,
	isConcat:        messages.H3094,
	isReshape:       messages.H3095,
	isCopy:          messages.H3096,
	isCast:          messages.H3097,
	isElementwise:   messages.H3090,
	isReduce:        messages.H3098,
	isSoftmaxOp:     messages.H3099,
	isNormalize:     messages.H3100,
	isEmbedding:     messages.H3101,
	isAttention:     messages.H3102,
	isGrad:          messages.H3103,
	isBackend:       messages.H3104,
}

// binopOp ikili işlemlerin (yayınlamalı) hata mesajı adları.
var binopOp = [...]opKind{OpAdd: isAdd, OpSub: isSub, OpMul: isMul, OpDiv: isDiv,
	OpAtan2: isBroadcastElem, OpLt: isCompare, OpGt: isCompare, OpLe: isCompare, OpGe: isCompare}

func (i opKind) String() string { return messages.Text(opNames[i]) }

// broadcasted işlemin çıktı şeklinin yayınlamayla üretilip üretilmediği.
func (i opKind) broadcasted() bool { return i >= isAdd && i <= isSelect }

// defaultMemory fiziksel bellek okunamazsa varsayılan sınır (64 GB).
const defaultMemory = 64 << 30

// limitBytes tek ayırmanın en fazla baytı; 0 henüz okunmadığını gösterir. İlk
// kullanımda fiziksel bellekten bir kez okunur. Testler küçültüp geri yükleyebilir.
var limitBytes atomic.Int64

// allocLimit tek ayırmanın en fazla baytını verir (fiziksel bellek).
func allocLimit() int64 {
	if s := limitBytes.Load(); s > 0 {
		return s
	}
	return readLimit()
}

// readLimit sınırı fiziksel bellekten okuyup saklar (ayirmaSiniri'nin soğuk yolu; sıcak
// yol satır içine alınabilsin diye ayrıdır).
func readLimit() int64 {
	b, ok := physicalMemory()
	if !ok || b == 0 || b > math.MaxInt64/8 {
		b = defaultMemory
	}
	limitBytes.Store(int64(b))
	return int64(b)
}

// elemCount şeklin eleman sayısını taşma denetimli verir. Negatif eksen paniktir;
// sıfır uzunluklu bir eksen varsa sonuç 0'dır (öbür eksenler ne kadar büyük olursa
// olsun); çarpım int'e sığmıyorsa tasma doğrudur.
func elemCount(shape []int) (n int, overflow bool) {
	u, zeroVal := uint64(1), false
	for _, d := range shape {
		if d <= 0 {
			if d < 0 {
				panic(messages.New(messages.H3053, shape))
			}
			zeroVal = true
			continue
		}
		if !overflow {
			hi, lo := bits.Mul64(u, uint64(d))
			if hi != 0 || lo > math.MaxInt {
				overflow = true
			} else {
				u = lo
			}
		}
	}
	if zeroVal {
		return 0, false
	}
	if overflow {
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
	return hi == 0 && lo <= uint64(allocLimit())*8
}

// allocSize, is işleminin dt türünde shape şeklinde ayıracağı tensörün eleman
// sayısını verir; şekil taşıyorsa ya da tensör fiziksel belleğe sığmıyorsa Türkçe
// bir hatayla panik eder. Tensör verisi ayıran her yol bu denetimden geçer.
func allocSize(is opKind, shape []int, dt DType) int {
	n, overflow := elemCount(shape)
	if overflow || !sigar(n, dt) {
		panic(limitError(is, shape, dt, overflow))
	}
	return n
}

// checkElem şekli bilinmeyen n elemanlık bir depolama ayırmasını denetler ve n'yi
// verir (arka uçların son savunması: gradyanlar, iç tamponlar, iyileştirici durumları).
func checkElem(n int, dt DType) int {
	if n < 0 {
		panic(messages.Internal(fmt.Sprintf("backend: negative element count %d", n)))
	}
	if !sigar(n, dt) {
		panic(SizeError(n, dt, allocLimit(), PlaceHost))
	}
	return n
}

// allocSizeOn ayirmaBoyu'na ek olarak arka ucun kendi bellek sınırını (MemoryLimiter,
// ör. GPU belleği) denetler; şekli bilinen ve bir arka uçta ayrılan her tensör buradan
// geçer.
func allocSizeOn(be Backend, is opKind, shape []int, dt DType) int {
	n := checkAllocOn(be, is, shape, dt)
	counterBytes(be, dt.StorageBytes(n))
	return n
}

// checkAllocOn allocSizeOn'un denetimidir (sayaç işlemez): ayırma başka bir şekille yapılacaksa
// (dikkatin bakış önde yolu) hata kullanıcının gördüğü şekille önceden verilir.
func checkAllocOn(be Backend, is opKind, shape []int, dt DType) int {
	n := allocSize(is, shape, dt)
	if l, ok := be.(MemoryLimiter); ok {
		if byteN := uint64(dt.StorageBytes(n)); byteN > uint64(l.MemoryLimit()) {
			panic(limitErrorAt(is, shape, dt, false, l.MemoryLimit(), l.MemoryPlace()))
		}
	}
	return n
}

// limitError sınırı aşan ayırmanın hatasını kurar (soğuk yol).
func limitError(is opKind, shape []int, dt DType, overflow bool) *messages.Error {
	return limitErrorAt(is, shape, dt, overflow, allocLimit(), PlaceHost)
}

// Bellek sınırının hangi yerde aşıldığını söyleyen anahtarlar (iletilerin {place, select, ...}
// dalları); metin değil, yalnızca anahtardır.
const (
	PlaceHost = "host"
	PlaceGPU  = "gpu"
)

// SizeError, arka uçların şekli bilinmeyen bir ayırmayı reddederken verdiği hatadır
// (n eleman, dt türü; sınır ve yer: PlaceGPU). Hata cihazdan bağımsız katmanın
// hatalarıyla aynı biçimdedir.
func SizeError(n int, dt DType, limit int64, place string) *messages.Error {
	return messages.New(messages.H3083,
		numberText(float64(n)), dt, BytesText(float64(n)*float64(max(dt.Bits(), 1))/8), place, BytesText(float64(limit)))
}

// limitErrorAt sınırı aşan ayırmanın hatasını, sınır ve yer (PlaceHost, PlaceGPU) verilerek
// kurar. Yerin sözcüğü iletinin şablonundadır ({place, select, ...}), Go'da değil.
func limitErrorAt(is opKind, shape []int, dt DType, overflow bool, limit int64, place string) *messages.Error {
	elemFn := 1.0
	for _, d := range shape {
		elemFn *= float64(d)
	}
	byteN := elemFn * float64(max(dt.Bits(), 1)) / 8
	hint := ""
	if is.broadcasted() {
		hint = "broadcast" // iletinin `hint` dalını seçer
	}
	switch {
	case overflow && is == isCreate:
		return messages.New(messages.H3086, is, ShapeText(shape), numberText(elemFn), BytesText(byteN), place, BytesText(float64(limit)), hint)
	case overflow:
		return messages.New(messages.H3087, is, ShapeText(shape), numberText(elemFn), BytesText(byteN), place, BytesText(float64(limit)), hint)
	case is == isCreate:
		return messages.New(messages.H3084, is, ShapeText(shape), BytesText(byteN), place, BytesText(float64(limit)), hint)
	}
	return messages.New(messages.H3085, is, ShapeText(shape), BytesText(byteN), place, BytesText(float64(limit)), hint)
}

// ShapeText bir şekli "[2, 3]" biçiminde yazar; ml ve kontrol noktası iletileri de bunu kullanır.
func ShapeText(shape []int) string {
	p := make([]string, len(shape))
	for i, d := range shape {
		p[i] = strconv.Itoa(d)
	}
	return "[" + strings.Join(p, ", ") + "]"
}

// BytesText bayt sayısını insan okunur birimle (1024 tabanlı) yazar; 1 KB altı bayt sayısı
// iletinin dilindedir (H4020). Ondalık nokta Go'nundur: iletinin şablonu {ad|decimal} ile onu
// iletinin dilinin ayırıcısına çevirir (tr "1,5 GB", en "1.5 GB"). GPU arka ucunun bellek iletileri de bunu kullanır
// (dil tanımı 15.41, 15.42).
func BytesText(b float64) string {
	units := [...]string{"", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	i := 0
	for b >= 1024 && i < len(units)-1 {
		b /= 1024
		i++
	}
	if i == 0 {
		return messages.Text(messages.H4020, int64(b))
	}
	if b >= 1e6 {
		return numberText(b) + " " + units[i]
	}
	return strconv.FormatFloat(b, 'f', 1, 64) + " " + units[i]
}

// numberText büyük bir sayıyı yaklaşık yazar: küçükse tam, büyükse "1.2 × 10^24" (ayırıcı
// iletide {ad|decimal}).
func numberText(x float64) string {
	if x < 1e15 {
		return strconv.FormatFloat(x, 'f', 0, 64)
	}
	us := math.Floor(math.Log10(x))
	m := x / math.Pow(10, us)
	return strconv.FormatFloat(m, 'f', 1, 64) + " × 10^" + strconv.Itoa(int(us))
}
