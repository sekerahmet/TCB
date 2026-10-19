package dataload

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// ---------------------------------------------------------------------------
// Sıralı okuma (doğrulama)
// ---------------------------------------------------------------------------

// SequentialOptions, sıralı okumanın ayarlarıdır.
type SequentialOptions struct {
	B, T int
	// Stride, ardışık pencere başlangıçları arasındaki token sayısıdır.
	// Varsayılan T: her token en çok bir kez hedef olur; parçanın ilk tokenı
	// ve son pencereye sığmayan kuyruğu (en çok T-1 token) hedef olmaz.
	Stride int
	// Start verilirse okuma bu konumdan sürer (bir toplu işin Durum'u: Sira parça
	// sırası, Pencere o parçadaki sonraki pencerenin token konumu). Sira parça sayısına
	// eşitse okuyucu baştan bitmiştir.
	Start *State
}

// SequentialReader, parçaları ad sırasıyla, karıştırmadan ve pencereleri
// sırayla okur. Pencereler parça sınırını aşmaz. Son toplu iş B'den az
// satırlı olabilir (ti.B buna göre ayarlanır); ardından io.EOF döner.
// Yöntemler kilitle korunur; Kapat süren bir Doldur'un bitmesini bekler.
type SequentialReader struct {
	mu         sync.Mutex
	k          *Dataset
	s          SequentialOptions
	row        int64
	order      int
	begin      int64
	shard      *Shard
	firstStart int64 // Baslangic'ın pencere konumu: ilk açılan parçada bas bununla başlar
	closed     bool
}

// Sequential, kümeden sıralı okuyan bir okuyucu kurar.
func (k *Dataset) Sequential(s SequentialOptions) (*SequentialReader, error) {
	if s.B <= 0 || s.T <= 0 {
		return nil, fmt.Errorf("veri: B ve T pozitif olmalı (B=%d, T=%d)", s.B, s.T)
	}
	if s.Stride == 0 {
		s.Stride = s.T
	}
	if s.Stride < 0 {
		return nil, fmt.Errorf("veri: adım negatif olamaz")
	}
	if err := k.checkCacheQuota(1, int64(s.T)+1); err != nil {
		return nil, err
	}
	so := &SequentialReader{k: k, s: s, row: int64(s.T) + 1}
	if b := s.Start; b != nil {
		if b.Pos < 0 || b.Pos > len(k.shards) || b.Window < 0 || (b.Pos == len(k.shards) && b.Window != 0) {
			return nil, fmt.Errorf("veri: geçersiz başlangıç durumu %+v (parça sayısı %d)", *b, len(k.shards))
		}
		so.order, so.firstStart = b.Pos, b.Window
	}
	return so, nil
}

// WindowCount, okuyucunun baştan sona üreteceği pencere sayısıdır.
func (so *SequentialReader) WindowCount() int64 {
	var t int64
	for _, p := range so.k.shards {
		if p.Token >= so.row {
			t += (p.Token-so.row)/int64(so.s.Stride) + 1
		}
	}
	return t
}

// Fill, ti'yi sonraki en çok B pencereyle doldurur; bitince io.EOF döner.
func (so *SequentialReader) Fill(ctx context.Context, ti *Batch) error {
	so.mu.Lock()
	defer so.mu.Unlock()
	if so.closed {
		return ErrClosed
	}
	ti.prepare(so.s.B, so.s.T)
	r := 0
	for r < so.s.B {
		if so.shard == nil {
			if so.order >= len(so.k.shards) {
				break
			}
			if so.k.shards[so.order].Token < so.row {
				so.order++
				so.firstStart = 0
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			p, err := so.k.OpenShard(ctx, so.order)
			if err != nil {
				return err
			}
			so.shard, so.begin = p, so.firstStart
			so.firstStart = 0
		}
		if so.begin+so.row > so.shard.Length() {
			err := so.shard.Close()
			so.shard = nil
			so.order++
			so.begin = 0
			if err != nil {
				return err
			}
			continue
		}
		if err := so.shard.Read(ti.Row(r), so.begin); err != nil {
			return err
		}
		so.begin += int64(so.s.Stride)
		r++
	}
	if r == 0 {
		return io.EOF
	}
	ti.B = r
	ti.Data = ti.Data[:r*int(so.row)]
	ti.State = State{Pos: so.order, Window: so.begin}
	return nil
}

// Rewind, okuyucuyu başa sarar.
func (so *SequentialReader) Rewind() error {
	so.mu.Lock()
	defer so.mu.Unlock()
	return so.toStart()
}

func (so *SequentialReader) toStart() error {
	var err error
	if so.shard != nil {
		err = so.shard.Close()
		so.shard = nil
	}
	so.order, so.begin, so.firstStart = 0, 0, 0
	return err
}

// Close, açık parçayı kapatır.
func (so *SequentialReader) Close() error {
	so.mu.Lock()
	defer so.mu.Unlock()
	so.closed = true
	return so.toStart()
}
