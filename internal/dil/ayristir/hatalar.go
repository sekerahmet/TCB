package ayristir

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"tr_dili/internal/dil/sozcuk"
)

// Ayrıştırıcının hata mesajları kullanıcıya iç token adlarını (DOSYA_SONU, SONUÇ,
// TANIMLAYICI) göstermez; bunların yerine "dosya sonu", "'sonuç' anahtar kelimesi",
// "'x' adı" gibi Türkçe anlatımlar kullanılır.

// turAciklamasi beklenen bir token türünü anlatır (ör. "'{'", "bir ad",
// "'içinde' anahtar kelimesi").
func turAciklamasi(t sozcuk.TokenType) string {
	switch t {
	case sozcuk.TOKEN_EOF:
		return "dosya sonu"
	case sozcuk.TOKEN_IDENT:
		return "bir ad"
	case sozcuk.TOKEN_INT:
		return "bir tam sayı"
	case sozcuk.TOKEN_FLOAT:
		return "bir ondalık sayı"
	case sozcuk.TOKEN_STRING:
		return "bir metin"
	case sozcuk.TOKEN_ILLEGAL:
		return "geçersiz bir karakter"
	}
	if yazim := anahtarKelimeYazimi(t); yazim != "" {
		return fmt.Sprintf("'%s' anahtar kelimesi", yazim)
	}
	return fmt.Sprintf("'%s'", string(t))
}

// tokenAciklamasi kaynakta bulunan bir token'ı yazıldığı biçimiyle anlatır
// (ör. "'x' adı", "42 sayısı", "'sonuç' anahtar kelimesi", "dosya sonu").
func tokenAciklamasi(tok sozcuk.Token) string {
	switch tok.Type {
	case sozcuk.TOKEN_EOF:
		return "dosya sonu"
	case sozcuk.TOKEN_IDENT:
		return fmt.Sprintf("'%s' adı", tok.Literal)
	case sozcuk.TOKEN_INT, sozcuk.TOKEN_FLOAT:
		return fmt.Sprintf("%s sayısı", tok.Literal)
	case sozcuk.TOKEN_STRING:
		return fmt.Sprintf("%s metni", strconv.Quote(kisaMetin(unescapeBraces(tok.Literal))))
	case sozcuk.TOKEN_ILLEGAL:
		return fmt.Sprintf("geçersiz '%s' karakteri", tok.Literal)
	}
	if anahtarKelimeYazimi(tok.Type) != "" {
		return fmt.Sprintf("'%s' anahtar kelimesi", tok.Literal)
	}
	return fmt.Sprintf("'%s'", tok.Literal)
}

// anahtarKelimeYazimi anahtar kelime türünün sözlükteki adını bulur; tür bir anahtar
// kelime değilse "" döner.
func anahtarKelimeYazimi(t sozcuk.TokenType) string {
	return sozcuk.AnahtarKelimeAdi(t)
}

// kisaMetin hata mesajında gösterilecek metin sabitini kısaltır.
func kisaMetin(s string) string {
	const sinir = 30
	if utf8.RuneCountInString(s) <= sinir {
		return s
	}
	return string([]rune(s)[:sinir-1]) + "…"
}

// beklenmeyenIpucu sık yapılan yanlışlar için hata mesajına eklenecek ipucunu verir.
func beklenmeyenIpucu(beklenen sozcuk.TokenType, bulunan sozcuk.Token) string {
	switch {
	case beklenen == sozcuk.TOKEN_LBRACE && bulunan.Type == sozcuk.TOKEN_ASSIGN:
		return " (karşılaştırma için '==' yazın; '=' yalnızca atamadır)"
	case beklenen == sozcuk.TOKEN_RPAREN && bulunan.Type == sozcuk.TOKEN_EOF,
		beklenen == sozcuk.TOKEN_RBRACKET && bulunan.Type == sozcuk.TOKEN_EOF,
		beklenen == sozcuk.TOKEN_RBRACE && bulunan.Type == sozcuk.TOKEN_EOF:
		return fmt.Sprintf(" (açılan bir parantez kapatılmamış olabilir; '%s' ekleyin)", string(beklenen))
	}
	return ""
}

// hataKaydet bir sözdizimi hatası kaydeder. Bir deyimde yalnızca ilk hata kaydedilir:
// ardından gelenler çoğunlukla ilkinin sonucudur ve kullanıcıyı yanıltır. Deyim
// döngüleri senkronizeOl ile hatalı deyimin geri kalanını atlayıp kaydı yeniden açar.
func (p *Parser) hataKaydet(tok sozcuk.Token, mesaj string) {
	if p.hataModu || p.derinlikAsildi {
		return
	}
	p.hataModu = true
	p.hataSatiri = tok.Line
	p.errors = append(p.errors, fmt.Sprintf("Satır %d, Sütun %d: %s", tok.Line, tok.Column, mesaj))
}

// blokSonunaAtla, içinde bulunulan '{ ... }' bloğunu kapatan '}' işaretine kadar
// (iç içe blokları sayarak) ilerler; dosya biterse dosya sonunda durur.
func (p *Parser) blokSonunaAtla() {
	derinlik := 0
	for !p.curTokenIs(sozcuk.TOKEN_EOF) {
		if p.curTokenIs(sozcuk.TOKEN_LBRACE) {
			derinlik++
		} else if p.curTokenIs(sozcuk.TOKEN_RBRACE) {
			if derinlik == 0 {
				return
			}
			derinlik--
		}
		p.nextToken()
	}
}

// acanMi ve kapayanMi parantez, köşeli parantez ve süslü parantezleri tanır.
func acanMi(t sozcuk.TokenType) bool {
	return t == sozcuk.TOKEN_LBRACE || t == sozcuk.TOKEN_LPAREN || t == sozcuk.TOKEN_LBRACKET
}

func kapayanMi(t sozcuk.TokenType) bool {
	return t == sozcuk.TOKEN_RBRACE || t == sozcuk.TOKEN_RPAREN || t == sozcuk.TOKEN_RBRACKET
}

// senkronizeOl, hatalı bir deyimden sonra (deyim deyimBas konumundaki token'da
// başlamıştı) deyimin geri kalanını atlar: açılıp kapanan parantezleri ve blokları
// sayarak, yeni bir satırda başlayan ilk deyime, ';' işaretine ya da içinde bulunulan
// bloğu kapatan '}' işaretine kadar ilerler. 'değilse' ve 'yakala' ile başlayan
// satırlar hatalı deyimin devamı sayılır. ustDuzey dosyanın en üst düzeyidir: orada
// kapatılacak blok olmadığından sahipsiz '}' de atlanır.
func (p *Parser) senkronizeOl(deyimBas int, ustDuzey bool) {
	if !p.hataModu {
		return
	}
	defer func() { p.hataModu = false }()

	// Hatalı deyim, içinde bulunduğu bloğu kapatan '}' işaretine kadar okuduysa
	// (ör. 'eğer x { yazdır( }'), o '}' bloğa geri bırakılır.
	if !ustDuzey && p.curTokenIs(sozcuk.TOKEN_RBRACE) && p.pos > deyimBas {
		suslu := 0
		for i := deyimBas; i <= p.pos && i < len(p.tokens); i++ {
			switch p.tokens[i].Type {
			case sozcuk.TOKEN_LBRACE:
				suslu++
			case sozcuk.TOKEN_RBRACE:
				suslu--
			}
		}
		if suslu < 0 {
			p.pos--
			return
		}
	}
	// Sayım hatanın ardından başlar: kapatılmamış bir '(' dosyanın geri kalanını yutmasın.
	derinlik := 0
	for !p.peekTokenIs(sozcuk.TOKEN_EOF) {
		pk := p.peekToken()
		if derinlik == 0 {
			if pk.Type == sozcuk.TOKEN_SEMICOLON || (pk.Type == sozcuk.TOKEN_RBRACE && !ustDuzey) {
				return
			}
			yeniSatir := pk.Line > p.hataSatiri && pk.Line > p.curToken().Line
			if yeniSatir && pk.Type != sozcuk.TOKEN_DEGILSE && pk.Type != sozcuk.TOKEN_YAKALA {
				return
			}
		}
		if acanMi(pk.Type) {
			derinlik++
		} else if kapayanMi(pk.Type) && derinlik > 0 {
			derinlik--
		}
		p.nextToken()
	}
}
