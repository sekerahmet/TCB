package tensor

import "github.com/sekerahmet/TCB/internal/lang/messages"

// TakeRows t[idx]'in gövdesidir (15.6d): ilk eksenin idx satırlarını sırayla kopyalar, sonucun şekli
// idxShape artı kalan eksenlerdir. Ondalıkta türev izlenir (Embedding); ham bayt türlerinde ve
// nicemlenmiş türde satırlar bayt bayt kopyalanır, bloklar bölünmez (15.53). Sınır dışı indeks
// çalışma hatasıdır, ileti ilk böyle indeksi söyler.
func TakeRows(t *Tensor, idx, idxShape []int) *Tensor {
	n := t.shape[0]
	for _, i := range idx {
		if i < 0 || i >= n {
			panic(messages.New(messages.H6694, i, n))
		}
	}
	shape := append(copyInts(idxShape), t.shape[1:]...)
	switch {
	case t.dtyp.IsQuantized():
		return t.quantRows(idx, shape)
	case t.dtyp.IsTyped():
		return takeRawRows(t, idx, shape)
	}
	rest := 1
	for _, d := range t.shape[1:] {
		rest *= d
	}
	flat := t.Reshape(n, rest)
	return Embedding(flat, idx).Reshape(shape...)
}

// takeRawRows ham bayt türünün satırlarını konakta kopyalar.
func takeRawRows(t *Tensor, idx, shape []int) *Tensor {
	raw := t.Raw()
	width := len(raw) / max(t.shape[0], 1)
	out := make([]byte, len(idx)*width)
	for k, i := range idx {
		copy(out[k*width:(k+1)*width], raw[i*width:(i+1)*width])
	}
	return FromRawOn(t.Device(), t.dtyp, out, shape...)
}
