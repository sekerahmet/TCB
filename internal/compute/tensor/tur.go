package tensor

import (
	"fmt"
	"sync/atomic"
)

// Veri türleri ve karışık hassasiyet (cihazdan bağımsız katman).
//
// Her tensörün bir eleman türü vardır (varsayılan Float32). Hesap türü,
// tensörlerin (ağırlıklar, etkinleştirmeler) türüdür; ör. bfloat16 seçilirse
// bellek ve bant genişliği yarıya iner. Biriktirme ayrıdır:
//   - Arka uçlar iç toplamaları (matris çarpımı, indirgemeler, normlar,
//     softmax, çapraz entropi) en az float32'de yapar; sonuç çıktı türüne bir
//     kez yuvarlanır.
//   - 16 bitlik tensörlerin gradyanları GradDType türünde (varsayılan
//     float32) tutulur; float32 tensörlerin gradyanları her zaman float32'dir.
//   - İyileştiriciler 16 bitlik parametreler için float32 ana ağırlık kopyası
//     tutar (SetMasterWeights); güncelleme ana kopyada yapılır ve parametreye
//     yuvarlanarak yazılır, böylece küçük adımlar yuvarlamada kaybolmaz.
//   - Çapraz entropi kaybı her zaman float32'dir; MatMulOut ile 16 bitlik
//     girdilerden float32 çıktı alınabilir.

// gradTuruAyar 16 bitlik tensörlerin gradyan türü.
var gradTuruAyar atomic.Uint32 // DType; sıfır değer Float32

// SetGradDType 16 bitlik (bfloat16/float16) tensörlerin gradyan biriktirme
// türünü ayarlar (varsayılan Float32). float32 tensörlerin gradyanı her zaman
// float32'dir. Süreç geneli bir ayardır.
func SetGradDType(dt DType) {
	if !dt.IsFloat() {
		panic(fmt.Sprintf("tensor: gradyan türü kayan noktalı olmalı, %s verildi", dt))
	}
	gradTuruAyar.Store(uint32(dt))
}

// GradDType 16 bitlik tensörlerin gradyan biriktirme türü.
func GradDType() DType { return DType(gradTuruAyar.Load()) }

// gradTuru t türündeki bir tensörün gradyan depolamasının türü.
func gradTuru(t DType) DType {
	if t == Float32 {
		return Float32
	}
	return GradDType()
}

// sonucTuru iki işlenenli işlemin çıktı türü: türler aynıysa o tür, aksi halde
// float32 (daha geniş tür).
func sonucTuru(a, b DType) DType {
	if a == b {
		return a
	}
	return Float32
}

// NewOf veriyi dt türüne (en yakına-çifte yuvarlayarak) dönüştürüp varsayılan
// cihazda tensör oluşturur.
func NewOf(dt DType, data []float32, shape ...int) *Tensor {
	be := defaultBackend()
	if len(data) != ayirmaBoyuOn(be, isOlusturma, shape, dt) {
		panic(fmt.Sprintf("tensor: %d eleman %v şekline uymuyor", len(data), shape))
	}
	return fromStorage(be, be.FromHost(data, dt), shape)
}

// ZerosOf dt türünde sıfırlarla dolu tensör.
func ZerosOf(dt DType, shape ...int) *Tensor {
	return zerosOf(isOlusturma, defaultBackend(), shape, dt)
}

// Cast tensörün dt türüne dönüştürülmüş bitişik bir kopyasını verir (en
// yakına-çifte yuvarlama). Türevlenebilir: gradyan kaynak türün gradyanına
// geri aktarılır. Tür zaten dt ise tensörün kendisi döner.
func (t *Tensor) Cast(dt DType) *Tensor {
	if t.tur == dt {
		return t
	}
	if !dt.IsFloat() {
		panic(fmt.Sprintf("tensor: Cast yalnızca kayan noktalı türlere yapılabilir, %s verildi", dt))
	}
	sy := sayacBasla(t.be, IslemTurDonusumu).baslat()
	out := emptyOf(isTurDonusumu, t.be, t.shape, dt)
	t.be.Copy(out.storage, 0, t.view())
	r := track(out, "cast", []*Tensor{t}, func() { gradDevret(t, out) })
	sy.bitir()
	return r
}

// Bits16 v'yi 16 bitlik dt türüne (BFloat16 ya da Float16) en yakına-çifte
// yuvarlayarak bit deseni olarak verir. Dosyaya 16 bitlik veri yazmak içindir:
// o türden bir tensörün Data() değerleri kayıpsız geri çevrilir.
func Bits16(dt DType, v float32) uint16 {
	if dt != BFloat16 && dt != Float16 {
		panic(fmt.Sprintf("tensor: Bits16 yalnızca bfloat16/float16 içindir, %s verildi", dt))
	}
	return daraltBir(v, dt)
}

// FromBits16 16 bitlik dt türündeki (BFloat16 ya da Float16) bit desenini
// float32'ye kayıpsız açar.
func FromBits16(dt DType, h uint16) float32 {
	switch dt {
	case BFloat16:
		return bf16ToF32(h)
	case Float16:
		return f16ToF32(h)
	}
	panic(fmt.Sprintf("tensor: FromBits16 yalnızca bfloat16/float16 içindir, %s verildi", dt))
}
