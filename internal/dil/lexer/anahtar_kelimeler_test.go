package lexer

import (
	"testing"

	"tr_dili/internal/dil/kod"
)

// Sözlükteki her anahtar kelime kodu (A0xx) tabloda tam bir kez bir token türüne bağlı
// olmalı; tablodaki her kodun sözlükte adı olmalı.
func TestAnahtarKelimeSozluguDuzgun(t *testing.T) {
	tablodaki := map[kod.Kod]bool{}
	for _, ak := range anahtarKelimeler {
		if tablodaki[ak.Kod] {
			t.Errorf("%s tabloda iki kez", ak.Kod)
		}
		tablodaki[ak.Kod] = true
		for _, y := range kod.Yazimlar(ak.Kod) {
			if LookupIdent(y) != ak.Tur {
				t.Errorf("%q → %s bekleniyordu, %s", y, ak.Tur, LookupIdent(y))
			}
		}
		if AnahtarKelimeAdi(ak.Tur) != kod.Ad(ak.Kod) {
			t.Errorf("%s adı %q, %q bekleniyordu", ak.Tur, AnahtarKelimeAdi(ak.Tur), kod.Ad(ak.Kod))
		}
	}
	for _, k := range kod.Kodlar() {
		if k.AnahtarKelime() && !tablodaki[k] {
			t.Errorf("sözlükteki %s (%s) bir token türüne bağlı değil", k, kod.Ad(k))
		}
	}
}

// Aynı token türü iki koda bağlanırsa başlangıçta yakalanmalı.
func TestAnahtarKelimeHaritasiCakismayiYakalar(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("çakışan tablo için panik bekleniyordu")
		}
	}()
	anahtarKelimeHaritasi([]AnahtarKelime{{kod.A001, TOKEN_EGER}, {kod.A002, TOKEN_EGER}})
}
