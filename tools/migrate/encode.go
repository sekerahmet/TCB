package main

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// encodeView yazımı yeni kurala çevrilmiş gösterim metnini (langCode dilinde) bugünkü kodlama yoluyla
// kimliklere çevirir ve başlıksız kodlu gövdeyi verir.
func encodeView(text, langCode string) (string, error) {
	lang, err := codes.LoadLanguage(langCode)
	if err != nil {
		return "", err
	}
	coded, err := source.EncodeIn(text, lang)
	if err != nil {
		return "", err
	}
	_, body, _ := firstLine(coded)
	if !strings.HasPrefix(coded, newHeader) {
		body = coded
	}
	return body, nil
}
