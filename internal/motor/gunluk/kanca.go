package gunluk

import (
	"sync/atomic"
	"time"

	"tr_dili/internal/dil/yurut"
)

// Adım sınırı kancası: eğitim döngüsünde İlerleme.güncelle'nin her çağrısı bir adımın
// bittiği yerdir. Canlı profil (tr izle, İzleme Masası) ms/adım'ı ve adım başına işlem
// dağılımını bu kancayla ölçer: kanca iki çağrı arasında geçen süreyi ve o arada biriken
// sayaçları adıma bağlar. Kanca yoksa maliyet tek bir atomik okumadır.

// AdimSiniri bir adım sınırının bilgisidir.
type AdimSiniri struct {
	// VM güncelle'yi çağıran görevdir (yorumlayıcı kilidini tutar).
	VM *yurut.VM
	// Adim güncellemeden sonra tamamlanan adım sayısıdır; Toplam çubuğun toplamıdır (0:
	// toplamsız sayaç).
	Adim, Toplam int64
	// Zaman çağrının anıdır (çubuğun kendi hız ölçümüyle aynı saat okuması).
	Zaman time.Time
}

var adimKancasi atomic.Pointer[func(AdimSiniri)]

// saat şimdiki zamandır; testler sıklık sınırlarını sınamak için değiştirir.
var saat = time.Now

// AdimKancasiAyarla her adım sınırında çağrılacak kancayı kurar (nil: kaldırır). Kanca
// yorumlayıcı kilidi altında, eğitim döngüsünün her adımında çağrılır: kısa olmalı, TR kodu
// çalıştırmamalı ve beklememelidir. Herhangi bir goroutine'den kurulabilir.
func AdimKancasiAyarla(f func(AdimSiniri)) {
	if f == nil {
		adimKancasi.Store(nil)
		return
	}
	adimKancasi.Store(&f)
}
