package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Sözlükteki her anahtar kelime kodu (A0xx) tabloda tam bir kez bir token türüne bağlı
// olmalı; tablodaki her kodun sözlükte adı olmalı.
func TestAnahtarKelimeSozluguDuzgun(t *testing.T) {
	tablodaki := map[codes.Code]bool{}
	for _, ak := range anahtarKelimeler {
		if tablodaki[ak.Code] {
			t.Errorf("%s tabloda iki kez", ak.Code)
		}
		tablodaki[ak.Code] = true
		for _, y := range codes.Spellings(ak.Code) {
			if LookupIdent(y) != ak.Type {
				t.Errorf("%q → %s bekleniyordu, %s", y, ak.Type, LookupIdent(y))
			}
		}
		if KeywordName(ak.Type) != codes.Name(ak.Code) {
			t.Errorf("%s adı %q, %q bekleniyordu", ak.Type, KeywordName(ak.Type), codes.Name(ak.Code))
		}
	}
	for _, k := range codes.Codes() {
		if k.IsKeyword() && !tablodaki[k] {
			t.Errorf("sözlükteki %s (%s) bir token türüne bağlı değil", k, codes.Name(k))
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
	anahtarKelimeHaritasi([]Keyword{{codes.A001, TOKEN_IF}, {codes.A002, TOKEN_IF}})
}
