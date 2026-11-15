package tensor

import "runtime"

// CacheZeros sabit önbelleğin tamponudur (lib/layers.tr, TR_cache_buffer): sıfırlı tensör, varsayılan
// cihaz işlemciyse öbek dışı blokta, önbelleğin payında. Blok verilmezse (küçük, pay dolu, platformda
// yol yok, tür float32 ya da 16 bitlik ondalık değil) ve kartta ZerosOf'tur; bitler aynı.
func CacheZeros(dt DType, shape ...int) *Tensor {
	be := defaultBackend()
	if _, cpu := be.(cpuBackend); !cpu || !dt.IsFloat() {
		return ZerosOf(dt, shape...)
	}
	n := allocSizeOn(be, isCreate, shape, dt)
	b := newHostBlock(n*dt.Bits()/8, hostCacheLimits, &hostCacheSpent)
	if b == nil {
		return ZerosOf(dt, shape...)
	}
	defer runtime.KeepAlive(b)
	return fromStorage(be, b.storage(dt, n), shape)
}

// CacheBlocksSpent önbelleğin payından ömür boyu verilen blok sayısıdır (sayaç düşmez).
func CacheBlocksSpent() int64 { return hostCacheSpent.blocks.Load() }

// LimitCacheShareForTest önbelleğin payını şimdiye dek verilenle sınırlar: sonraki önbellek öbekte
// kurulur (bitler aynı olmalı). Dönen işlev payı geri verir. Paralel testlerde güvenli değildir.
func LimitCacheShareForTest() (restore func()) {
	old := hostCacheLimits
	spent := hostCacheSpent.blocks.Load()
	hostCacheLimits.blocks = func() int64 { return spent }
	return func() { hostCacheLimits = old }
}
