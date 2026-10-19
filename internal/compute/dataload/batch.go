package dataload

import (
	"context"
	"fmt"
)

// Batch, [B, T+1] boyutlu bir token toplu işidir. Satır i, ardışık T+1
// tokendır: girdi ilk T token, hedef ise bir kaydırılmış son T tokendır.
// Veri satır öncelikli ve bitişiktir.
type Batch struct {
	B, T  int
	Data  []int32 // uzunluk B*(T+1)
	State State   // bu toplu iş üretildikten sonraki örnekleyici durumu
}

// prepare, Veri'yi B*(T+1) uzunluğa getirir; yeterli kapasite varsa
// yeniden ayırmaz.
func (t *Batch) prepare(b, tt int) {
	t.B, t.T = b, tt
	n := b * (tt + 1)
	if cap(t.Data) >= n {
		t.Data = t.Data[:n]
	} else {
		t.Data = make([]int32, n)
	}
}

// Row, i'inci satırın T+1 tokenıdır (kopya değil, görünüm).
func (t *Batch) Row(i int) []int32 {
	s := t.T + 1
	return t.Data[i*s : (i+1)*s]
}

// Input, i'inci satırın ilk T tokenıdır (görünüm).
func (t *Batch) Input(i int) []int32 {
	s := t.T + 1
	return t.Data[i*s : i*s+t.T]
}

// Target, i'inci satırın bir kaydırılmış T tokenıdır (görünüm).
func (t *Batch) Target(i int) []int32 {
	s := t.T + 1
	return t.Data[i*s+1 : (i+1)*s]
}

// SplitInputTarget, girdi ve hedefi bitişik [B, T] dizilere kopyalar. Verilen dilimlerin
// kapasitesi yetmezse yenileri ayrılır; sonuç dilimleri döner.
func (t *Batch) SplitInputTarget(input, target []int32) ([]int32, []int32) {
	n := t.B * t.T
	if cap(input) < n {
		input = make([]int32, n)
	}
	if cap(target) < n {
		target = make([]int32, n)
	}
	input, target = input[:n], target[:n]
	for i := 0; i < t.B; i++ {
		copy(input[i*t.T:(i+1)*t.T], t.Input(i))
		copy(target[i*t.T:(i+1)*t.T], t.Target(i))
	}
	return input, target
}

// U16, Veri'yi uint16 olarak dst'ye kopyalar (kapasite yetmezse yeni ayırır).
// 65535'ten büyük bir token varsa hata döner.
func (t *Batch) U16(dst []uint16) ([]uint16, error) {
	if cap(dst) < len(t.Data) {
		dst = make([]uint16, len(t.Data))
	}
	dst = dst[:len(t.Data)]
	for i, v := range t.Data {
		if uint32(v) > 0xFFFF {
			return nil, fmt.Errorf("veri: %d konumundaki parça kimliği %d, 16 bite sığmıyor", i, v)
		}
		dst[i] = uint16(v)
	}
	return dst, nil
}

// Clone, toplu işin bağımsız bir kopyasıdır.
func (t *Batch) Clone() *Batch {
	k := *t
	k.Data = append([]int32(nil), t.Data...)
	return &k
}

// Source, toplu iş üreten her şeydir (Ornekleyici, SiraliOkuyucu).
// Doldur, ti'yi sonraki toplu işle doldurur; kaynak bittiğinde io.EOF döner.
type Source interface {
	Fill(ctx context.Context, ti *Batch) error
}
