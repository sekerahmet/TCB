package ogrenme

import (
	"fmt"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// ----------------------------------------------------------------------------
// Türev
// ----------------------------------------------------------------------------

// geriYayilim geri_yayılım(kayıp) yerleşiğidir: tek elemanlı kayıptan geri yayılım yapar;
// yaprak tensörlerin gradyanları birikir.
func geriYayilim(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("geri_yayılım(kayıp) 1 parametre bekler, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("geri_yayılım()", named); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("geri_yayılım() bir tensör bekler, %s verildi", tipAdi(args[0]))
	}
	t := d.t
	if t.Numel() != 1 {
		return nil, fmt.Errorf("geri_yayılım tek elemanlı (skaler) bir kayıp ister, %s şeklinde tensör verildi; önce toplam(...) ya da ortalama(...) ile indirgeyin", sekilMetni(t.Shape()))
	}
	if !t.RequiresGrad() {
		return nil, fmt.Errorf("geri_yayılım: kayıp türev izlenen hiçbir tensöre bağlı değil; parametreleri tensör(..., türev_izle: doğru) ile oluşturun (türevsiz bloğu içinde hesaplanan değerler izlenmez)")
	}
	// Dış düğümlerin (özel_türev) geri hesapları bu görevle çalışır.
	if err := geriYay(m, t, nil); err != nil {
		return nil, err
	}
	return yurut.TRUE, nil
}

// turevsiz iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func turevsiz(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %d parametre verildi", len(args))
	}
	if err := baglama.AdliDenetle("türevsiz()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *Deger:
		return Yeni(x.t.Detach()), nil
	case *yurut.FuncVal, *yurut.BuiltinVal:
		var sonuc yurut.Value
		var err error
		tensor.NoGrad(func() {
			sonuc, err = m.ApplyFunction(x, nil, nil, 0)
		})
		if err != nil {
			return nil, err
		}
		if sonuc == nil {
			sonuc = yurut.NULL
		}
		return sonuc, nil
	}
	return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %s verildi", tipAdi(args[0]))
}
