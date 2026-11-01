package ml

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	randv2 "math/rand/v2"
	"strconv"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Paylaşılan rastgele üreteç (dil tanımı 20.17): tohumsuz TR_randn ve TR_sample bundan çeker.
// Her program aynı sabit durumla başlar; durum kontrol noktasına "rng" alanıyla yazılır ve
// geri yüklenir (20.5). Üreteç Go'nun math/rand/v2 PCG'sidir; kayıt Go'nun ikili biçimine değil,
// iki 64 bitlik durum sözcüğüne bağlıdır.

const (
	// sharedRandomHi ve sharedRandomLo paylaşılan üretecin başlangıç durumudur (20.17).
	sharedRandomHi uint64 = 0x5452
	sharedRandomLo uint64 = 0x2017
	// sharedRandomPrefix kayıttaki durumun önekidir: "pcg:" ve 32 küçük onaltılık rakam.
	sharedRandomPrefix = "pcg:"
)

// sharedRandomKey programın paylaşılan üretecinin anahtarıdır (interp.VM.ProgramValue).
type sharedRandomKey struct{}

// sharedRandom bir programın paylaşılan üretecidir; isRandom durumu ve çekmeleri korur
// (eşzamanlı görevler aynı üreteçten çeker).
type sharedRandom struct {
	isRandom sync.Mutex
	pcg      *randv2.PCG
	stream   *rand.Rand
}

// pcgSource PCG'yi math/rand'in (v1) Source64 arayüzüne sarar: tensor.Randn *rand.Rand alır.
type pcgSource struct{ pcg *randv2.PCG }

func (s pcgSource) Uint64() uint64 { return s.pcg.Uint64() }
func (s pcgSource) Int63() int64   { return int64(s.pcg.Uint64() >> 1) }

// Seed kullanılmaz (paylaşılan üreteç tohumlanmaz, durumu yüklenir); PCG'yi tohumun durumuna koyar.
func (s pcgSource) Seed(seed int64) { s.pcg.Seed(uint64(seed), 0) }

func newSharedRandom() *sharedRandom {
	pcg := randv2.NewPCG(sharedRandomHi, sharedRandomLo)
	return &sharedRandom{pcg: pcg, stream: rand.New(pcgSource{pcg})}
}

// sharedRandomOf m'nin programının paylaşılan üretecidir; ilk kullanımda sabit durumla kurulur.
func sharedRandomOf(m *interp.VM) *sharedRandom {
	return m.ProgramValue(sharedRandomKey{}, func() any { return newSharedRandom() }).(*sharedRandom)
}

// draw f'yi paylaşılan üretecin kilidi altında çağırır.
func (r *sharedRandom) draw(f func(stream *rand.Rand)) {
	r.isRandom.Lock()
	defer r.isRandom.Unlock()
	f(r.stream)
}

// SharedRandomState programın paylaşılan üretecinin durumunu kaydın yazımıyla verir:
// "pcg:" ve iki durum sözcüğü, önce yüksek, 16'şar küçük onaltılık rakam (20.5).
func SharedRandomState(m *interp.VM) string {
	r := sharedRandomOf(m)
	r.isRandom.Lock()
	defer r.isRandom.Unlock()
	b, _ := r.pcg.MarshalBinary() // "pcg:" + hi (8 bayt, büyük sonlu) + lo; hata vermez
	hi, lo := binary.BigEndian.Uint64(b[4:12]), binary.BigEndian.Uint64(b[12:20])
	return fmt.Sprintf("%s%016x%016x", sharedRandomPrefix, hi, lo)
}

// ParseSharedRandomState kaydın yazımını çözer; yazım tam "pcg:" ve 32 küçük onaltılık rakam
// değilse ok yanlıştır.
func ParseSharedRandomState(s string) (hi, lo uint64, ok bool) {
	digits, found := strings.CutPrefix(s, sharedRandomPrefix)
	if !found || len(digits) != 32 || strings.ToLower(digits) != digits {
		return 0, 0, false
	}
	hi, errHi := strconv.ParseUint(digits[:16], 16, 64)
	lo, errLo := strconv.ParseUint(digits[16:], 16, 64)
	return hi, lo, errHi == nil && errLo == nil
}

// SetSharedRandomState programın paylaşılan üretecini çözülmüş duruma koyar (kontrol noktasından
// yükleme, 20.17).
func SetSharedRandomState(m *interp.VM, hi, lo uint64) {
	r := sharedRandomOf(m)
	r.isRandom.Lock()
	defer r.isRandom.Unlock()
	r.pcg.Seed(hi, lo)
}
