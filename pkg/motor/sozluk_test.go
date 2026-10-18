package motor

import (
	"testing"

	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/vm"
)

// Tüm motorlar yüklüyken genel kapsamdaki her yerleşiğin sözlükte bir yerleşik kodu
// (G/T/V/W) olmalı ve kayıtlı ad o kodun adı olmalı; sözlükteki her yerleşik kodu da
// bütün yazımlarıyla (eş yazımlar dahil) kayıtlı olmalı. Böylece sözlüğe kaydı olmayan bir ad eklenemez,
// kayıt da sözlükte adı olmayan bir ad kullanamaz.
func TestYerlesiklerSozlukleTutarli(t *testing.T) {
	m := vm.New()
	for _, ad := range m.GlobalEnv.Names() {
		d, _ := m.GlobalEnv.Get(ad)
		b, ok := d.(*vm.BuiltinVal)
		if !ok {
			continue
		}
		k, ok := sozluk.Bul(sozluk.AlanGenel, ad)
		if !ok || !k.Yerlesik() {
			t.Errorf("kayıtlı yerleşik %q sözlükte bir yerleşik kodu değil", ad)
			continue
		}
		if asil, _ := sozluk.AsilAdAlanda(sozluk.AlanGenel, ad); b.Name != asil {
			t.Errorf("%q (%s) yerleşiğinin adı %q, sözlükte %q", ad, k, b.Name, asil)
		}
	}
	for _, k := range sozluk.Kodlar() {
		if !k.Yerlesik() {
			continue
		}
		b := m.Yerlesik(k)
		if b == nil {
			t.Errorf("sözlükteki %s (%s) kayıtlı değil", k, sozluk.Ad(k))
			continue
		}
		for _, y := range sozluk.Yazimlar(k) {
			d, _ := m.GlobalEnv.Get(y)
			if sozluk.EsYazimi(k, y) {
				// Eş yazım aynı fonksiyonu kendi adıyla taşır.
				if e, ok := d.(*vm.BuiltinVal); !ok || e == b {
					t.Errorf("%s eş yazımı %q ayrı adlı bir yerleşiğe bağlı olmalı", k, y)
				}
				continue
			}
			if d != b {
				t.Errorf("%s yazımı %q aynı yerleşiğe bağlı değil", k, y)
			}
		}
	}
}
