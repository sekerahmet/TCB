package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// textsJSON komutun bütün metinleridir: çıktı satırları (söz bölümü S1), biçim yanlışlarının
// iletileri ve ölçü dosyalarının biçim sözcükleri (tablo başlıkları, "— (motordan bağımsız)" ...).
// Go kodunda Türkçe metin sabiti olmaz (TestNoTurkishLiterals); metinler bu dil dosyasındadır.
//
//go:embed texts.json
var textsJSON []byte

var texts = loadTexts(textsJSON)

func loadTexts(data []byte) map[string]string {
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		panic("measurefresh: texts.json: " + err.Error())
	}
	return m
}

// raw anahtarın metnidir; dosyada olmayan anahtar programcı hatasıdır.
func raw(key string) string {
	s, ok := texts[key]
	if !ok {
		panic("measurefresh: texts.json has no key " + key)
	}
	return s
}

// text anahtarın metnini args ile biçimler.
func text(key string, args ...any) string {
	if len(args) == 0 {
		return raw(key)
	}
	return fmt.Sprintf(raw(key), args...)
}
