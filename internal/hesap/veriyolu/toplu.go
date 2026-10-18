package veriyolu

import (
	"context"
	"fmt"
)

// TopluIs, [B, T+1] boyutlu bir token toplu işidir. Satır i, ardışık T+1
// tokendır: girdi ilk T token, hedef ise bir kaydırılmış son T tokendır.
// Veri satır öncelikli ve bitişiktir.
type TopluIs struct {
	B, T  int
	Veri  []int32 // uzunluk B*(T+1)
	Durum Durum   // bu toplu iş üretildikten sonraki örnekleyici durumu
}

// hazirla, Veri'yi B*(T+1) uzunluğa getirir; yeterli kapasite varsa
// yeniden ayırmaz.
func (t *TopluIs) hazirla(b, tt int) {
	t.B, t.T = b, tt
	n := b * (tt + 1)
	if cap(t.Veri) >= n {
		t.Veri = t.Veri[:n]
	} else {
		t.Veri = make([]int32, n)
	}
}

// Satir, i'inci satırın T+1 tokenıdır (kopya değil, görünüm).
func (t *TopluIs) Satir(i int) []int32 {
	s := t.T + 1
	return t.Veri[i*s : (i+1)*s]
}

// Girdi, i'inci satırın ilk T tokenıdır (görünüm).
func (t *TopluIs) Girdi(i int) []int32 {
	s := t.T + 1
	return t.Veri[i*s : i*s+t.T]
}

// Hedef, i'inci satırın bir kaydırılmış T tokenıdır (görünüm).
func (t *TopluIs) Hedef(i int) []int32 {
	s := t.T + 1
	return t.Veri[i*s+1 : (i+1)*s]
}

// Ayir, girdi ve hedefi bitişik [B, T] dizilere kopyalar. Verilen dilimlerin
// kapasitesi yetmezse yenileri ayrılır; sonuç dilimleri döner.
func (t *TopluIs) Ayir(girdi, hedef []int32) ([]int32, []int32) {
	n := t.B * t.T
	if cap(girdi) < n {
		girdi = make([]int32, n)
	}
	if cap(hedef) < n {
		hedef = make([]int32, n)
	}
	girdi, hedef = girdi[:n], hedef[:n]
	for i := 0; i < t.B; i++ {
		copy(girdi[i*t.T:(i+1)*t.T], t.Girdi(i))
		copy(hedef[i*t.T:(i+1)*t.T], t.Hedef(i))
	}
	return girdi, hedef
}

// U16, Veri'yi uint16 olarak dst'ye kopyalar (kapasite yetmezse yeni ayırır).
// 65535'ten büyük bir token varsa hata döner.
func (t *TopluIs) U16(dst []uint16) ([]uint16, error) {
	if cap(dst) < len(t.Veri) {
		dst = make([]uint16, len(t.Veri))
	}
	dst = dst[:len(t.Veri)]
	for i, v := range t.Veri {
		if uint32(v) > 0xFFFF {
			return nil, fmt.Errorf("veri: %d konumundaki parça kimliği %d, 16 bite sığmıyor", i, v)
		}
		dst[i] = uint16(v)
	}
	return dst, nil
}

// Kopya, toplu işin bağımsız bir kopyasıdır.
func (t *TopluIs) Kopya() *TopluIs {
	k := *t
	k.Veri = append([]int32(nil), t.Veri...)
	return &k
}

// Kaynak, toplu iş üreten her şeydir (Ornekleyici, SiraliOkuyucu).
// Doldur, ti'yi sonraki toplu işle doldurur; kaynak bittiğinde io.EOF döner.
type Kaynak interface {
	Doldur(ctx context.Context, ti *TopluIs) error
}
