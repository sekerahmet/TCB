package checkpoint

import (
	"time"

	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
)

// Testlerin dosya sistemini ve saati taklit etmesi için (yalnızca testlerde derlenir).

// FakeRename yeniden adlandırmayı f'ye yaptırır (ağ sürücüsü benzetimi); geri
// alan fonksiyonu döndürür.
func FakeRename(f func(old, fresh string) error) func() {
	prev := rename
	rename = f
	return func() { rename = prev }
}

// FakeClock şimdiki zamanı f'den okutur; geri alan fonksiyonu döndürür.
func FakeClock(f func() time.Time) func() {
	prev := clock
	clock = f
	return func() { clock = prev }
}

// İç yardımcılar (bulanıklaştırma ve biçim testleri).
var (
	DecodeDigest = decodeDigest
	DecodeState  = decodeState
	ReadPlain    = readPlain
	WritePlain   = writePlain
)

// Veri okuyucusu kontrol noktasına durumuyla kaydedilir; arayüz yapısal olduğundan bir ad
// değişikliği bu bağı sessizce koparırdı.
var _ Stateful = (*dataset.Reader)(nil)
