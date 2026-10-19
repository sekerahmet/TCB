package interp

import (
	"errors"
	"fmt"
)

// Program çıkışı: bir yerleşik programın hemen, belirli bir çıkış koduyla bitmesini
// isteyebilir (ör. komut satırı seçeneklerinde --yardım yazılınca 0, hatalı bir argümanda
// 2). İstek bir hata gibi yukarı yayılır: yığın çözülürken 'ertele' blokları çalışır, ama
// 'dene/yakala' onu yakalamaz (kesilme gibi, bkz. kesilme.go). Run isteği döndürür;
// komut satırı Mesaj'ı yazar ve Kod ile çıkar.

// CikisIstegi programın bitirilmesi isteğidir. Mesaj insanın göreceği metindir (yardım ya da
// hata; sonunda satır sonu olmadan); Kod 0 ise standart çıktıya, değilse standart hataya
// yazılır.
type CikisIstegi struct {
	Kod   int
	Mesaj string
}

func (c *CikisIstegi) Error() string {
	if c.Mesaj == "" {
		return fmt.Sprintf("program %d koduyla bitirildi", c.Kod)
	}
	return c.Mesaj
}

// CikisIstegiMi hata (ya da sardığı bir hata) bir program çıkışı isteği mi?
func CikisIstegiMi(err error) (*CikisIstegi, bool) {
	var c *CikisIstegi
	if errors.As(err, &c) {
		return c, true
	}
	return nil, false
}

// yakalanamaz dene/yakala'nın yakalamadığı hatalardır: kesilme ve program çıkışı isteği.
func yakalanamaz(err error) bool {
	if errors.Is(err, ErrKesildi) {
		return true
	}
	_, ok := CikisIstegiMi(err)
	return ok
}
