package lexer

import (
	"testing"

	"tr_dili/pkg/sozluk"
)

// Sözlükteki her anahtar kelime kodu (A0xx) tabloda tam bir kez bir token türüne bağlı
// olmalı; tablodaki her kodun sözlükte adı olmalı.
func TestAnahtarKelimeSozluguDuzgun(t *testing.T) {
	tablodaki := map[sozluk.Kod]bool{}
	for _, ak := range anahtarKelimeler {
		if tablodaki[ak.Kod] {
			t.Errorf("%s tabloda iki kez", ak.Kod)
		}
		tablodaki[ak.Kod] = true
		for _, y := range sozluk.Yazimlar(ak.Kod) {
			if LookupIdent(y) != ak.Tur {
				t.Errorf("%q → %s bekleniyordu, %s", y, ak.Tur, LookupIdent(y))
			}
		}
		if AnahtarKelimeAdi(ak.Tur) != sozluk.Ad(ak.Kod) {
			t.Errorf("%s adı %q, %q bekleniyordu", ak.Tur, AnahtarKelimeAdi(ak.Tur), sozluk.Ad(ak.Kod))
		}
	}
	for _, k := range sozluk.Kodlar() {
		if k.AnahtarKelime() && !tablodaki[k] {
			t.Errorf("sözlükteki %s (%s) bir token türüne bağlı değil", k, sozluk.Ad(k))
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
	anahtarKelimeHaritasi([]AnahtarKelime{{sozluk.A001, TOKEN_EGER}, {sozluk.A002, TOKEN_EGER}})
}
