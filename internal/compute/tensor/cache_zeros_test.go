package tensor

import (
	"slices"
	"testing"
)

// cacheShape blok eşiğinin üstündedir: float32'de 2 MiB, bf16'da 1 MiB.
var cacheShape = []int{4, 128, 2, 512}

// withCacheLimits önbelleğin payını test boyunca verilen sınırlara indirir.
func withCacheLimits(t *testing.T, blocks, address int64) {
	t.Helper()
	old := hostCacheLimits
	hostCacheLimits.blocks = func() int64 { return blocks }
	hostCacheLimits.address = address
	t.Cleanup(func() { hostCacheLimits = old })
}

// inBlock tensörün deposu öbek dışı blok mu.
func inBlock(x *Tensor) bool {
	s, ok := x.storage.(*cpuStorage)
	return ok && s.block != nil
}

// needCacheBlock bu platformda önbelleğe blok verilmiyorsa testi atlar.
func needCacheBlock(t *testing.T, dt DType) *Tensor {
	t.Helper()
	x := CacheZeros(dt, cacheShape...)
	if !inBlock(x) {
		t.Skip("bu platformda öbek dışı blok yok")
	}
	return x
}

// fillCache iki katmanın görünümüne (K ve V gibi, sıfırıncı eksende dilim) parça yazar.
func fillCache(x *Tensor) {
	chunk := make([]float32, 1*3*2*512)
	for i := range chunk {
		chunk[i] = float32(i%97) - 48.5
	}
	src := New(chunk, 1, 3, 2, 512)
	if x.DType() != Float32 {
		src = src.Cast(x.DType())
	}
	x.Slice(0, 1, 2).Slice(1, 5, 8).CopyFrom(src)
	x.Slice(0, 3, 4).Slice(1, 0, 3).CopyFrom(src)
}

// Önbelleğin tamponu blokta; pay dolunca öbekte. İkisinde de sıfırlıdır ve aynı yazmalar aynı
// bitleri verir (A1).
func TestKVCacheBuffersUseHostBlocks(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		t.Run(dt.String(), func(t *testing.T) {
			blk := needCacheBlock(t, dt)
			withCacheLimits(t, hostCacheSpent.blocks.Load(), hostAddressSpace)
			heap := CacheZeros(dt, cacheShape...)
			if inBlock(heap) {
				t.Fatal("önbelleğin payı dolu, blok verildi")
			}
			if !slices.Equal(blk.Data(), heap.Data()) || slices.ContainsFunc(blk.Data(), func(v float32) bool { return v != 0 }) {
				t.Fatal("tampon sıfırlı değil")
			}
			fillCache(blk)
			fillCache(heap)
			if !slices.Equal(blk.Data(), heap.Data()) {
				t.Fatal("blokta ve öbekte aynı yazmalar farklı bitler verdi")
			}
		})
	}
}

// Katmanların K ve V'si tek bloğun sıfırıncı eksendeki dilimleridir: dilim bitişiktir, yazılan
// bloğa ve yalnız kendi yerine gider (A5).
func TestKVCacheViewsWriteThroughBlock(t *testing.T) {
	blk := needCacheBlock(t, Float32)
	for i := range 4 {
		if !blk.Slice(0, i, i+1).IsContiguous() {
			t.Fatalf("katman görünümü %d bitişik değil", i)
		}
	}
	fillCache(blk)
	data, row := blk.Data(), 128*2*512
	for layer := range 4 {
		part := data[layer*row : (layer+1)*row]
		written := slices.ContainsFunc(part, func(v float32) bool { return v != 0 })
		if written != (layer == 1 || layer == 3) {
			t.Fatalf("görünüm %d: yazıldı %v", layer, written)
		}
	}
	if got := data[1*row+5*2*512+7]; got != float32(7%97)-48.5 {
		t.Fatalf("bloğa yazılan değer %v", got)
	}
}

// Büyük sıfır tensörü (TR_zeros) öbekte kalır: blok yalnız önbelleğin tamponunadır (A2).
func TestZerosStayOnHeap(t *testing.T) {
	needCacheBlock(t, Float32)
	if inBlock(ZerosOf(Float32, cacheShape...)) {
		t.Fatal("sıfır tensörü bloğa gitti")
	}
}

// Önbellek payı dolunca önbellek öbekte sürer, ağırlık yine blok alır; ağırlık payı dolunca da
// önbellek blok alır. Önbelleğin bloğundan türeyen çevirme ağırlık payından blok almaz (A7).
func TestCacheShareLeavesWeightShare(t *testing.T) {
	blk := needCacheBlock(t, Float32)
	if inBlock(blk.Cast(BFloat16)) {
		t.Fatal("önbelleğin çevirmesi ağırlık payından blok aldı")
	}
	t.Run("önbellek payı dolu", func(t *testing.T) {
		withCacheLimits(t, hostCacheSpent.blocks.Load(), hostAddressSpace)
		weights := hostMemory.blocks.Load()
		if inBlock(CacheZeros(Float32, cacheShape...)) {
			t.Fatal("önbelleğin payı dolu, blok verildi")
		}
		if NewHostBlock(hostSize*4) == nil {
			t.Fatal("önbellek payı dolu, ağırlığa blok verilmedi")
		}
		if hostMemory.blocks.Load() != weights+1 {
			t.Fatal("ağırlığın bloğu ağırlık payına sayılmadı")
		}
	})
	t.Run("ağırlık payı dolu", func(t *testing.T) {
		withHostLimits(t, hostMemory.blocks.Load(), hostAddressSpace)
		cache := hostCacheSpent.blocks.Load()
		if NewHostBlock(hostSize*4) != nil {
			t.Fatal("ağırlık payı dolu, blok verildi")
		}
		if !inBlock(CacheZeros(Float32, cacheShape...)) {
			t.Fatal("ağırlık payı dolu, önbelleğe blok verilmedi")
		}
		if hostCacheSpent.blocks.Load() != cache+1 {
			t.Fatal("önbelleğin bloğu önbellek payına sayılmadı")
		}
	})
}

// İki pay işletim sisteminin sınırının yarısını böler: dörtte biri önbelleğin.
func TestHostSharesSplitTheLimit(t *testing.T) {
	total := hostBlockLimit()
	if got := hostCacheLimits.blocks(); got != total/4 {
		t.Fatalf("önbelleğin blok payı %d, beklenen %d", got, total/4)
	}
	if got := hostLimits.blocks() + hostCacheLimits.blocks(); got != total {
		t.Fatalf("payların toplamı %d, beklenen %d", got, total)
	}
	if got := hostLimits.address + hostCacheLimits.address; got != hostAddressSpace/2 {
		t.Fatalf("adres paylarının toplamı %d, beklenen %d", got, int64(hostAddressSpace/2))
	}
}
