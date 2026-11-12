package tensor

import (
	le "encoding/binary"
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Cihaz içi hata bayrağının konak yüzü (15.6, 15.45)
// ============================================================================
//
// Kartın hatası öğeye bağlı çekirdeği hatalı öğenin anahtarını (düz konum << 4 | FaultKind) ve
// o konumun girdilerinin ham bitlerini bir yuvaya yazar (gpu/fault_slot.go). FaultFromSlot
// ayrıntıyı işlemcinin aynı işlemiyle tek öğede yeniden hesaplar: ileti iki cihazda aynıdır.
// TR_replay kaydında hata beklenmez: arka uç Typed'dan Kind'ı FaultRecorded, Index'i kaydın
// kaçıncı yuvası olan bir ElemFault döner, sonuç tensörü geçerlidir; oynatmadan sonra
// FaultRecording.FirstFault yuvaları okur.

// FaultRecording (isteğe bağlı) hata yuvası taşıyan kayıt.
type FaultRecording interface {
	Recording
	// FirstFault oynatmadan sonra yuvaları okur (bekler): kayıt sırasındaki ilk dolu yuvanın
	// sırası ve hatası; hata yoksa -1, nil.
	FirstFault() (site int, fault *ElemFault)
}

// FaultFromSlot yuvanın anahtarından ve girdilerin ham bitlerinden (bits[j], küçük uçlu, öğenin
// baytları kadar) ElemFault'u kurar: çeşit ve değerler işlemcinin aynı işleminden, konum
// anahtardan. İşlemcinin çeşidi anahtardakinden başkaysa iç hatadır.
func FaultFromSlot(op TypedOp, out DType, ins []DType, key uint64, bits []uint64) *ElemFault {
	at, kind := key>>4, FaultKind(key&15)
	ts := make([]*Tensor, len(ins))
	for j, dt := range ins {
		ts[j] = elemFromBits(dt, bits[j])
	}
	r, f := typed(op, out, []int{1}, ts...)
	for _, t := range ts {
		t.Drop()
	}
	if f == nil || f.Kind != kind {
		if r != nil {
			r.Drop()
		}
		panic(messages.Internalf("fault slot: kind %d at %d, the CPU gives %v", kind, at, f))
	}
	f.Index = int(at)
	return f
}

// IndexFaultFromSlot indeks yuvasından (gömme, dilim yazma) ElemFault'u kurar: çeşit ve konum
// anahtardan, değer indeksin ham bitlerinden (dt: float32, float64 ya da int64).
func IndexFaultFromSlot(dt DType, key, bits uint64) *ElemFault {
	f := &ElemFault{Kind: FaultKind(key & 15), Index: int(key >> 4), DType: dt}
	if f.Kind != FaultIndexNotInt && f.Kind != FaultIndexRange {
		panic(messages.Internalf("index fault slot: kind %d at %d", f.Kind, f.Index))
	}
	switch dt {
	case Int64:
		f.Int = int64(bits)
	case Float64:
		f.Float = math.Float64frombits(bits)
	default:
		f.Float = float64(math.Float32frombits(uint32(bits)))
	}
	return f
}

// elemFromBits işlemcide tek öğeli dt tensörü: ham bitlerden (ondalıkta bitler, tam sayıda ve
// mantıkta küçük uçlu baytlar).
func elemFromBits(dt DType, b uint64) *Tensor {
	switch {
	case dt == Float32:
		return NewOn(CPU, []float32{math.Float32frombits(uint32(b))}, 1)
	case dt == BFloat16 || dt == Float16:
		return FromBits16On(CPU, dt, []uint16{uint16(b)}, 1)
	}
	var raw [8]byte
	le.LittleEndian.PutUint64(raw[:], b)
	return FromRawOn(CPU, dt, raw[:elemBytes(dt)], 1)
}
