package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Sözlükteki her anahtar kelime kodu (A0xx) tabloda tam bir kez bir token türüne bağlı
// olmalı; tablodaki her kodun sözlükte adı olmalı.
func TestAnahtarKelimeSozluguDuzgun(t *testing.T) {
	tablodaki := map[codes.Kod]bool{}
	for _, ak := range anahtarKelimeler {
		if tablodaki[ak.Kod] {
			t.Errorf("%s tabloda iki kez", ak.Kod)
		}
		tablodaki[ak.Kod] = true
		for _, y := range codes.Yazimlar(ak.Kod) {
			if LookupIdent(y) != ak.Tur {
				t.Errorf("%q → %s bekleniyordu, %s", y, ak.Tur, LookupIdent(y))
			}
		}
		if AnahtarKelimeAdi(ak.Tur) != codes.Ad(ak.Kod) {
			t.Errorf("%s adı %q, %q bekleniyordu", ak.Tur, AnahtarKelimeAdi(ak.Tur), codes.Ad(ak.Kod))
		}
	}
	for _, k := range codes.Kodlar() {
		if k.AnahtarKelime() && !tablodaki[k] {
			t.Errorf("sözlükteki %s (%s) bir token türüne bağlı değil", k, codes.Ad(k))
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
	anahtarKelimeHaritasi([]AnahtarKelime{{codes.A001, TOKEN_EGER}, {codes.A002, TOKEN_EGER}})
}
