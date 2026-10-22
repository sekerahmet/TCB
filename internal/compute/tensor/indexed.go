package tensor

import "github.com/sekerahmet/TCB/internal/lang/messages"

// İndeksi cihazdaki bir tensörden okuyan işlemler. Bir cihaz kaydı (CUDA Graph, gpu
// graph.go) sürerken indeks konağa okunamaz: kaydedilen iş her oynatmada indeksi cihazdan
// yeniden okumalıdır. Cihaz hata veremediği için sınır dışı indeks kırpılır (JAX gather ve
// lax.dynamic_update_slice gibi); kayıt dışındaki çağrılar indeksi konakta denetler (ml
// katmanı), bu biçimler yalnızca kayıt içindir ve türev izlemez.

// Capturer (isteğe bağlı) bir kayıt sürüp sürmediğini bildiren arka uç.
type Capturer interface{ Capturing() bool }

// Recorder (isteğe bağlı) iş dizisini kaydedip oynatabilen arka uç (gpu graph.go).
// CaptureSupported yanlışsa diğer yöntemler çağrılmaz. EndCapture kaydı kurar; kayıt bir
// sürücü kuralıyla bozulduysa panikler. AbortCapture süren kaydı bırakır (hata yolunda).
type Recorder interface {
	CaptureSupported() bool
	BeginCapture()
	EndCapture() Recording
	AbortCapture()
}

// Recording kaydedilmiş, oynatılabilir bir iş dizisi.
type Recording interface {
	Replay()
	Release()
}

// IndexReader (isteğe bağlı) indeksi cihazdaki depolamadan okuyan işlemleri yapan arka uç.
type IndexReader interface {
	// EmbeddingAt out[r, :] = w[kırp(idx[r]), :] (idx float32 tam sayı değerli, n eleman).
	EmbeddingAt(out Storage, w View, idx View, D int)
	// UpdateSliceAt dst (bitişik, [dış, Td, iç]) içine src'yi ([dış, Ts, iç]) eksende s'den
	// başlayarak yazar; s = kırp(start[0], 0, Td−Ts).
	UpdateSliceAt(dst View, src View, start View, outer, Td, Ts, inner int)
}

// Capturing cihazın arka ucunda bir kayıt sürüyor mu.
func Capturing(dev Device) bool {
	c, ok := BackendFor(dev).(Capturer)
	return ok && c.Capturing()
}

func indexReader(fn string, t *Tensor) IndexReader {
	r, ok := t.be.(IndexReader)
	if !ok {
		panic(messages.New(messages.H3088, fn, t.Device()))
	}
	return r
}

// EmbeddingAt Embedding'in indeksleri cihazdaki idx tensöründen okuyan biçimidir; çıktı
// şekli idx şekli + [D].
func EmbeddingAt(w, idx *Tensor) *Tensor {
	if len(w.shape) != 2 {
		panic(messages.New(messages.H3041, w.shape))
	}
	sameDevice("EmbeddingAt", w, idx)
	D := w.shape[1]
	out := emptyOf(isEmbedding, w.be, append(copyInts(idx.shape), D), w.dtyp)
	if idx.Numel() > 0 && D > 0 {
		indexReader("EmbeddingAt", w).EmbeddingAt(out.storage, w.view(), idx.view(), D)
	}
	return out
}

// UpdateSliceAt dst'nin axis eksenindeki [s, s+src.şekil[axis]) dilimine src'yi yazar
// (yerinde; dst'nin sürümü artar). s 0 boyutlu start tensöründen okunur. dst bitişik olmalı,
// src'nin öbür eksenleri dst'ninkilerle aynı olmalıdır. src dst'nin depolamasındaysa (örtüşen
// dilim) önce kopyalanır: çekirdek okuduğu yere yazardı (CopyFrom gibi).
func UpdateSliceAt(dst, src, start *Tensor, axis int) {
	sameDevice("UpdateSliceAt", dst, src)
	sameDevice("UpdateSliceAt", dst, start)
	axis = normAxis(axis, len(dst.shape))
	if len(src.shape) != len(dst.shape) || src.shape[axis] > dst.shape[axis] {
		panic(messages.New(messages.H3111, "UpdateSliceAt", src.shape, dst.shape, axis))
	}
	outer, inner := 1, 1
	for i, n := range dst.shape {
		if i != axis && src.shape[i] != n {
			panic(messages.New(messages.H3111, "UpdateSliceAt", src.shape, dst.shape, axis))
		}
		if i < axis {
			outer *= n
		} else if i > axis {
			inner *= n
		}
	}
	if !dst.IsContiguous() {
		panic(messages.Internal("UpdateSliceAt needs a contiguous destination"))
	}
	if src.Numel() > 0 {
		if src.storage == dst.storage {
			c := src.Clone()
			defer c.Drop()
			src = c
		}
		indexReader("UpdateSliceAt", dst).UpdateSliceAt(dst.view(), src.view(), start.view(), outer, dst.shape[axis], src.shape[axis], inner)
	}
	dst.storage.BumpVersion()
}
