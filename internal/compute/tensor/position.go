package tensor

import "sync/atomic"

// ============================================================================
// TR konumu: geri hesabın süresi, düğümü üreten ileri satıra yazılsın diye
// ============================================================================
//
// Satır örnekleyicisi (devtools/profiler) açıkken her düğüm, oluştuğu anın TR konumunu taşır
// (örnekleyicinin kodladığı opak 64 bitlik değer; tensör paketi onu yorumlamaz). Geri hesap
// çalışırken o düğümün konumu "etkin geri konum"dur: örnekleyici o ana düşen süreyi
// geri_yayılım(...) satırına değil, düğümü üreten ileri satıra yazar. PyTorch'ta aynı eşleme
// emit_nvtx'in sıra numarasıyla yapılır (ileri aralık seq=N, geri aralık stashed seq=N);
// burada eşleme düğümün kendisindedir, numara gerekmez.
//
// Maliyet: örnekleme kapalıyken track'te bir atomik gösterici okuması, geri yayılımda düğüm
// başına bir yazma.

// positionSource o anki TR konumunu veren işlevdir (nil: örnekleme kapalı).
var positionSource atomic.Pointer[func() uint64]

// SetPositionSource konum kaynağını kurar; nil kapatır. Profil başında ve sonunda çağrılır.
func SetPositionSource(f func() uint64) {
	if f == nil {
		positionSource.Store(nil)
		return
	}
	positionSource.Store(&f)
}

// currentPosition o anki TR konumudur (kaynak yoksa 0).
func currentPosition() uint64 {
	if f := positionSource.Load(); f != nil {
		return (*f)()
	}
	return 0
}

// backwardPos geri hesabı çalışan düğümün konumudur (0: geri hesap yok ya da konum yok).
var backwardPos uint64

// BackwardPosition geri hesabı çalışan düğümün TR konumunu verir (0: yok). Örnekleyici
// goroutine'i okur; değer tek kelimedir.
func BackwardPosition() uint64 { return load64u(&backwardPos) }

// Owner bir arka uç işini (ör. bir GPU çekirdeğini) başlatan tensör işlemidir: türü, yönü ve
// TR konumu (geri yönde düğümü üreten ileri satır; konum kaynağı yoksa 0). GPU profili her
// çekirdeğin cihaz süresini buna yazar.
type Owner struct {
	Kind     OpKind
	Backward bool
	Pos      uint64
}

// CurrentOwner o anda çalışan işlemi ve konumunu verir (işlem yoksa diğer).
func CurrentOwner() Owner {
	is, back, _ := ActiveOp()
	o := Owner{Kind: is, Backward: back}
	if back {
		o.Pos = BackwardPosition()
	} else {
		o.Pos = currentPosition()
	}
	return o
}
