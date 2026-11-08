package tensor

// Yerinde yazmaya kapalı depo (dil tanımı 15.3): kopya olan .TR_reshape'in sonucu ve onun
// görünümleri yazılamaz, yazılan değer kaynağa ulaşmazdı. Kilit deponun VersionCounter'ındadır
// (CPU ve GPU depoları gömer); kilidi ml'nin .TR_reshape'i kurar, ml'nin yazma kapısı
// (checkWrite) okur. Motorun kendi ara kopyaları kilitlenmez.

// WriteLock yerinde yazmaya kapalı bir deponun nedenidir: kopyayı kuran .TR_reshape'in
// kaynağının şekli ve adımları (görünüme izin vermeyenler).
type WriteLock struct {
	Shape, Strides []int
}

// LockWrites depoyu yerinde yazmaya kapatır; geri açılmaz.
func (c *VersionCounter) LockWrites(l *WriteLock) { c.writeLock.Store(l) }

// WriteLock deponun yazma kilidi; açıksa nil.
func (c *VersionCounter) WriteLock() *WriteLock { return c.writeLock.Load() }

// writeLocker yazma kilidi tutan depodur (VersionCounter'ı gömen CPU ve GPU depoları).
type writeLocker interface {
	LockWrites(l *WriteLock)
	WriteLock() *WriteLock
}

// LockWrites t'nin deposunu (ve onu paylaşan bütün görünümleri) yerinde yazmaya kapatır:
// kaynağın şekli ve adımları nedendir. Kilit tutamayan depoda false döner.
func (t *Tensor) LockWrites(srcShape, srcStrides []int) bool {
	c, ok := t.storage.(writeLocker)
	if ok {
		c.LockWrites(&WriteLock{Shape: copyInts(srcShape), Strides: copyInts(srcStrides)})
	}
	return ok
}

// WriteLock t'nin deposunun yazma kilidi; açıksa nil.
func (t *Tensor) WriteLock() *WriteLock {
	if c, ok := t.storage.(writeLocker); ok {
		return c.WriteLock()
	}
	return nil
}
