package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// ----------------------------------------------------------------------------
// Türev
// ----------------------------------------------------------------------------

// geriYayilim geri_yayılım(kayıp) yerleşiğidir: tek elemanlı kayıptan geri yayılım yapar;
// yaprak tensörlerin gradyanları birikir.
func geriYayilim(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T300)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(kayıp) 1 parametre bekler, %d verildi", fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("%s() bir tensör bekler, %s verildi", fn, tipAdi(args[0]))
	}
	t := d.t
	if t.Numel() != 1 {
		return nil, fmt.Errorf("%s tek elemanlı (skaler) bir kayıp ister, %s şeklinde tensör verildi; önce %s(...) ya da %s(...) ile indirgeyin",
			fn, sekilMetni(t.Shape()), codes.Name(codes.G400), codes.Name(codes.G410))
	}
	if !t.RequiresGrad() {
		return nil, fmt.Errorf("%s: kayıp türev izlenen hiçbir tensöre bağlı değil; parametreleri %s ile oluşturun (%s bloğu içinde hesaplanan değerler izlenmez)",
			fn, turevIzleyenOrnek("..."), codes.Name(codes.T301))
	}
	// Dış düğümlerin (özel_türev) geri hesapları bu görevle çalışır.
	if err := geriYay(m, t, nil); err != nil {
		return nil, err
	}
	return interp.TRUE, nil
}

// turevsiz iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func turevsiz(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T301)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s { ... } bir blok ya da %s(x) bir tensör bekler, %d parametre verildi", fn, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *Deger:
		return Yeni(x.t.Detach()), nil
	case *interp.FuncVal, *interp.BuiltinVal:
		var sonuc interp.Value
		var err error
		tensor.NoGrad(func() {
			sonuc, err = m.ApplyFunction(x, nil, nil, 0)
		})
		if err != nil {
			return nil, err
		}
		if sonuc == nil {
			sonuc = interp.NULL
		}
		return sonuc, nil
	}
	return nil, fmt.Errorf("%s { ... } bir blok ya da %s(x) bir tensör bekler, %s verildi", fn, fn, tipAdi(args[0]))
}
