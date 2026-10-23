package tensor

import (
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Tensör ömrü: belirlenimci bırakma (dil tanımı 15.43)
// ============================================================================
//
// Bir depolamanın sahipleri onu gösteren tensör başlıkları (her görünüm ayrı bir başlıktır)
// ve geri yayılımın kaydettikleridir. Her başlık oluşurken depolamanın sahip sayacını
// artırır (newHeader); Drop başlığı bırakır, geri yayılım bir düğümün kaydettiklerini o
// düğümün geri hesabından hemen sonra bırakır. Sayaç sıfıra inince depolama arka uca iade
// edilir (Releaser). Hiç bırakılmayan (çöp toplanan) bir başlık sayacı azaltmaz; onun
// depolaması bugünkü gibi çöp toplayıcının temizliğiyle döner. Kural tek yönlüdür: yalnızca
// bütün sahipleri açıkça bırakılmış depolama erken döner, şüpheli her durumda çöp toplayıcı
// kalır (PyTorch'ta depolamanın referans sayısı; TR'de sayılmayan sahip "sonsuz" sayılır).
//
// Başlığın kendisini kimin bıraktığı üst katmanın işidir: yorumlayıcı bir tensör değerinin
// tek sahibi olduğunu bildiğinde (internal/lang/interp sahiplik kuralları) değeri bırakır;
// aynı başlığı birden çok değer sarabilir, başlık her biri bırakılınca düşer (Wrap,
// Unwrap). Başlığı saklayan Go kodu (önbellek) bırakılmış başlığı kullanmamalıdır:
// IsDropped ile denetler (castMemo), ya da başlığı hiç sarmaz.
//
// Bırakılmış başlığın verisine erişim iç hatadır (view, Data): bir sahiplik hatası sessiz
// bozulma yerine görünür bir hataya dönüşür (CUDA.jl'nin "use of freed memory"
// denetimi gibi).

// headerSerial şimdiye kadar oluşturulan başlık sayısıdır; her başlık kendi sırasını taşır.
var headerSerial atomic.Uint64

// Created şimdiye kadar oluşturulan başlık sayısıdır. Bir çağrıdan önce okunup başlığın
// Serial'ıyla karşılaştırılırsa başlığın o çağrıda oluşup oluşmadığı anlaşılır (taze sonuç).
func Created() uint64 { return headerSerial.Load() }

// Serial başlığın oluşturulma sırası (Created ile karşılaştırılır).
func (t *Tensor) Serial() uint64 { return t.serial }

// newHeader yeni başlığa sıra verir ve depolamasının sahip sayacını artırır. Başlık
// oluşturan her yol bundan geçer.
func newHeader(t *Tensor) *Tensor {
	t.serial = headerSerial.Add(1)
	if t.storage != nil {
		t.storage.Hold()
	}
	return t
}

// Wrap başlığı saran bir değer daha olduğunu bildirir (bkz. Unwrap).
func (t *Tensor) Wrap() { t.wraps.Add(1) }

// Unwrap başlığı saran bir değerin bırakıldığını bildirir; başlığı saran son değer
// bırakılınca başlık düşer (Drop). Sarılmadan bırakmak iç hatadır.
func (t *Tensor) Unwrap() {
	switch n := t.wraps.Add(-1); {
	case n == 0:
		t.Drop()
	case n < 0:
		panic(messages.Internal("tensor header unwrapped more often than wrapped"))
	}
}

// Drop başlığı bırakır: depolamanın başka sahibi yoksa depolama arka uca döner. Başlık
// bundan sonra kullanılamaz; ikinci çağrı etkisizdir.
func (t *Tensor) Drop() {
	if !t.dropped.CompareAndSwap(false, true) {
		return
	}
	unholdStorage(t.be, t.storage)
}

// dropTemp, after'dan sonra oluşmuş (after = 0: her) ve hiçbir değerin sarmadığı başlığı
// bırakır; geri hesaplar ara tensörlerini böyle iade eder.
func (t *Tensor) dropTemp(after uint64) {
	if t.serial > after && t.wraps.Load() == 0 {
		t.Drop()
	}
}

// IsDropped başlığın bırakılıp bırakılmadığı.
func (t *Tensor) IsDropped() bool { return t.dropped.Load() }

// storageReturned, sahipsiz kalan depolama arka uca iade edilirken çağrılır (yalnızca
// testlerin gözlemi içindir; havuz iade edilen depolamayı hemen yeniden verebildiğinden
// depolamanın kendisine bakmak güvenilir değildir).
var storageReturned func(Storage)

// unholdStorage depolamanın bir sahibini düşer; son sahip gidince depolamayı iade eder.
func unholdStorage(be Backend, s Storage) {
	if s == nil {
		return
	}
	switch n := s.Unhold(); {
	case n == 0:
		if storageReturned != nil {
			storageReturned(s)
		}
		giveBack(be, s)
	case n < 0:
		panic(messages.Internal("storage released more often than held"))
	}
}

// mustLive bırakılmış başlığın kullanılmasını iç hata yapar.
func (t *Tensor) mustLive() {
	if t.dropped.Load() {
		panic(messages.Internal("a released tensor was used"))
	}
}

// holdSaved kaydedilen tensörlerin depolamalarını düğüm adına tutar (geri hesap okuyacak).
func holdSaved(o *node) {
	for _, k := range o.registered {
		k.t.storage.Hold()
	}
}

// releaseSaved düğümün kaydettiklerini bırakır: geri hesabı bitmiştir.
func releaseSaved(n *node) {
	for _, k := range n.registered {
		unholdStorage(k.t.be, k.t.storage)
	}
	n.registered = nil
}
