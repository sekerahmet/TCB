package logging

import (
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Adım sınırı kancası: eğitim döngüsünde İlerleme.güncelle'nin her çağrısı bir adımın
// bittiği yerdir. Canlı profil (tr izle, İzleme Masası) ms/adım'ı ve adım başına işlem
// dağılımını bu kancayla ölçer: kanca iki çağrı arasında geçen süreyi ve o arada biriken
// sayaçları adıma bağlar. Kanca yoksa maliyet tek bir atomik okumadır.

// StepBoundary bir adım sınırının bilgisidir.
type StepBoundary struct {
	// VM güncelle'yi çağıran görevdir (yorumlayıcı kilidini tutar).
	VM *interp.VM
	// Step güncellemeden sonra tamamlanan adım sayısıdır; Total çubuğun toplamıdır (0:
	// toplamsız sayaç).
	Step, Total int64
	// Time çağrının anıdır (çubuğun kendi hız ölçümüyle aynı saat okuması).
	Time time.Time
}

var stepHook atomic.Pointer[func(StepBoundary)]

// clock şimdiki zamandır; testler sıklık sınırlarını sınamak için değiştirir.
var clock = time.Now

// SetStepHook her adım sınırında çağrılacak kancayı kurar (nil: kaldırır). Kanca
// yorumlayıcı kilidi altında, eğitim döngüsünün her adımında çağrılır: kısa olmalı, TR kodu
// çalıştırmamalı ve beklememelidir. Herhangi bir goroutine'den kurulabilir.
func SetStepHook(f func(StepBoundary)) {
	if f == nil {
		stepHook.Store(nil)
		return
	}
	stepHook.Store(&f)
}
