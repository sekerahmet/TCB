package lexer

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Sözlükteki her anahtar kelime kodu (A0xx) tabloda tam bir kez bir token türüne bağlı
// olmalı; tablodaki her kodun sözlükte adı olmalı.
func TestKeywordDictionaryValid(t *testing.T) {
	inTable := map[codes.Code]bool{}
	for _, kw := range keywordTable {
		if inTable[kw.Code] {
			t.Errorf("%s tabloda iki kez", kw.Code)
		}
		inTable[kw.Code] = true
		for _, y := range codes.Spellings(kw.Code) {
			if LookupIdent(y) != kw.Type {
				t.Errorf("%q → %s bekleniyordu, %s", y, kw.Type, LookupIdent(y))
			}
		}
		if KeywordName(kw.Type) != codes.Name(kw.Code) {
			t.Errorf("%s adı %q, %q bekleniyordu", kw.Type, KeywordName(kw.Type), codes.Name(kw.Code))
		}
	}
	for _, k := range codes.Codes() {
		if k.IsKeyword() && !inTable[k] {
			t.Errorf("sözlükteki %s (%s) bir token türüne bağlı değil", k, codes.Name(k))
		}
	}
}

// Aynı token türü iki koda bağlanırsa başlangıçta yakalanmalı.
func TestKeywordMapCatchesClash(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("çakışan tablo için panik bekleniyordu")
		}
	}()
	keywordMap([]Keyword{{codes.A001, TokIf}, {codes.A002, TokIf}})
}
