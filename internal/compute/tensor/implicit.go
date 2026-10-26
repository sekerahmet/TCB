package tensor

import "time"

// ============================================================================
// Örtük kopya sayaçları: bir işlemin içinde arka uçta yapılan kopya, dönüşüm, ekleme
// ============================================================================
//
// İşlem sayaçları (counter.go) bir işlemin toplam süresini verir; işlemin içinde arka uca
// giden kopyaları (bitişik olmayan girdinin bitişikleştirilmesi, tür dönüşümü, gradyan
// biriktirme) ayırmaz. Bu sayaçlar her arka uç kopyasını o anda çalışan işleme (üst işlem)
// bağlar: profil "kopya ← matris çarpımı" gibi satırlar gösterir (PyTorch profilinde
// aten::copy_ / aten::to çağıran işlemin altında görünür; burada tablo düzdür, üst işlem
// anahtardır). Çalışan işlem yoksa üst işlem "diğer"dir.
//
// Maliyet: çağrı ve eleman sayısı her kopyada iki toplamadır. Süre yalnızca CPU'da ve en az
// implicitMeasureMin elemanlı kopyada ölçülür (iki saat okuması kopyanın yanında
// önemsizdir); küçük kopyaların süresi sayılmaz, "ölçülen" alanı kaçının ölçüldüğünü verir.
// GPU'da konak süresi yalnızca başlatmadır: çekirdeğin cihazdaki süresini GPU profili verir.

// ImplicitKind örtük işin türüdür.
type ImplicitKind uint8

const (
	ImplicitCopy       ImplicitKind = iota // aynı tür, iki taraf bitişik
	ImplicitStrided                        // aynı tür, en az bir taraf adımlı ya da yayınlanmış
	ImplicitCast                           // tür dönüşümü
	ImplicitAccumulate                     // hedefe ekleyerek yazma (gradyan biriktirme)
	ImplicitKindCount
)

// implicitCodes dosya biçimindeki değişmez anahtarlardır (insana gösterilen ad profiler'dadır).
var implicitCodes = [ImplicitKindCount]string{"copy", "strided_copy", "cast", "accumulate"}

// Code türün dosya biçimindeki değişmez anahtarıdır.
func (k ImplicitKind) Code() string {
	if k < ImplicitKindCount {
		return implicitCodes[k]
	}
	return implicitCodes[ImplicitCopy]
}

// implicitMeasureMin süresi ölçülen en küçük kopyanın eleman sayısıdır.
const implicitMeasureMin = 4096

// implicitRecord bir (cihaz, yön, üst işlem, tür) dörtlüsünün sayaçlarıdır.
type implicitRecord struct {
	call, elems, dur, measured int64
}

// implicits [cihaz][yön][üst işlem][tür] (cihaz ve yön counters ile aynı anlamda).
var implicits [2][2][256][ImplicitKindCount]implicitRecord

// ImplicitMeasure bir örtük kopyanın ölçümüdür (NoteImplicit verir, Finish kapatır).
type ImplicitMeasure struct {
	r     *implicitRecord
	begin int64
}

// NoteImplicit CPU'da (gpuDevice yanlış) ya da GPU'da n elemanlı bir örtük işi çalışan
// işleme yazar. Dönen ölçüm Finish ile kapatılır; süre yalnızca CPU'da ve büyük işte ölçülür.
func NoteImplicit(gpuDevice bool, kind ImplicitKind, n int) ImplicitMeasure {
	c, direction, parent := 0, 0, OpKindOther
	if gpuDevice {
		c = 1
	}
	if a := read32i(&activeNo); a != 0 {
		a--
		direction, parent = int(a>>8)&1, OpKind(a)
	}
	r := &implicits[c][direction][parent][kind]
	add64(&r.call, 1)
	add64(&r.elems, int64(n))
	if c == 0 && n >= implicitMeasureMin {
		return ImplicitMeasure{r: r, begin: clock()}
	}
	return ImplicitMeasure{}
}

// Finish ölçülen süreyi ekler (ölçülmüyorsa bir şey yapmaz).
func (m ImplicitMeasure) Finish() {
	if m.r == nil {
		return
	}
	d := clock() - m.begin
	if d < 0 {
		d = 0
	}
	add64(&m.r.dur, d)
	add64(&m.r.measured, 1)
}

// ImplicitKindOf src'yi dst türünde bir hedefe yazan kopyanın türüdür (iki arka uç da kullanır).
func ImplicitKindOf(dst DType, src View, dstContiguous bool) ImplicitKind {
	switch {
	case src.DType() != dst:
		return ImplicitCast
	case !dstContiguous || !src.IsContiguous():
		return ImplicitStrided
	}
	return ImplicitCopy
}

// ImplicitRecord bir örtük iş kaydının süreç başından beri biriken sayaçlarıdır.
type ImplicitRecord struct {
	Device   DeviceKind
	Backward bool   // üst işlem geri yayılımda
	Parent   OpKind // üst işlem (çalışan işlem yoksa diğer)
	Kind     ImplicitKind
	// Calls çağrı, Elems eleman sayısıdır (kesin); Measured süresi ölçülen çağrı sayısıdır.
	Calls, Elems, Measured int64
	Duration               time.Duration // ölçülen çağrıların toplam süresi
}

// ImplicitSnapshot sıfır olmayan örtük iş kayıtlarını verir (kilitsiz okuma, counters gibi).
func ImplicitSnapshot() []ImplicitRecord {
	ratio := tickDurationRatio()
	var out []ImplicitRecord
	for c := range implicits {
		for direction := range implicits[c] {
			for p := OpKind(0); p < OpKindCount; p++ {
				for k := ImplicitKind(0); k < ImplicitKindCount; k++ {
					r := &implicits[c][direction][p][k]
					call := read64(&r.call)
					if call == 0 {
						continue
					}
					out = append(out, ImplicitRecord{
						Device: DeviceKind(c), Backward: direction == 1, Parent: p, Kind: k,
						Calls: call, Elems: read64(&r.elems), Measured: read64(&r.measured),
						Duration: time.Duration(float64(read64(&r.dur)) * ratio),
					})
				}
			}
		}
	}
	return out
}
