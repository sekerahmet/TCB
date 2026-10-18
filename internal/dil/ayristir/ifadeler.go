package ayristir

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/sozcuk"
)

// --- İFADE ÇÖZÜMLEME (PRATT PARSER) ---

func (p *Parser) parseExpression(precedence int) agac.Expression {
	if !p.derinlikGir() {
		return nil
	}
	defer p.derinlikCik()
	var leftExp agac.Expression
	desen := p.desenKipi
	p.desenKipi = false

	switch p.curToken().Type {
	case sozcuk.TOKEN_IDENT:
		// Tek parametreli ok fonksiyonu: x => x * 2 (eşle deseninde ad, değeriyle karşılaştırılır)
		if !desen && p.peekTokenIs(sozcuk.TOKEN_ARROW) {
			leftExp = p.parseSingleParamLambda()
		} else {
			leftExp = &agac.Identifier{Token: p.curToken(), Value: p.curToken().Literal}
		}
	case sozcuk.TOKEN_KENDI:
		leftExp = &agac.Identifier{Token: p.curToken(), Value: kendiAdi}
	case sozcuk.TOKEN_INT:
		leftExp = p.parseIntegerLiteral()
	case sozcuk.TOKEN_FLOAT:
		leftExp = p.parseFloatLiteral()
	case sozcuk.TOKEN_STRING:
		leftExp = p.parseStringOrInterpolation()
	case sozcuk.TOKEN_DOGRU:
		leftExp = &agac.BooleanLiteral{Token: p.curToken(), Value: true}
	case sozcuk.TOKEN_YANLIS:
		leftExp = &agac.BooleanLiteral{Token: p.curToken(), Value: false}
	case sozcuk.TOKEN_YOK:
		leftExp = &agac.NullLiteral{Token: p.curToken()}
	case sozcuk.TOKEN_BANG, sozcuk.TOKEN_MINUS, sozcuk.TOKEN_DEGIL:
		leftExp = p.parsePrefixExpression()
	case sozcuk.TOKEN_LPAREN:
		leftExp = p.parseGroupedOrLambdaExpression(desen)
	case sozcuk.TOKEN_EGER:
		leftExp = p.parseIfExpression()
	case sozcuk.TOKEN_ESLE:
		leftExp = p.parseMatchExpression()
	case sozcuk.TOKEN_FONKSIYON:
		leftExp = p.parseFunctionLiteral()
	case sozcuk.TOKEN_LBRACKET:
		leftExp = p.parseListLiteral()
	case sozcuk.TOKEN_LBRACE:
		leftExp = p.parseDictLiteral()
	default:
		p.addError("Sözdizimi hatası — burada bir değer ya da ifade bekleniyordu, ancak %s bulundu", tokenAciklamasi(p.curToken()))
		return nil
	}

	for !p.peekTokenIs(sozcuk.TOKEN_SEMICOLON) && precedence < p.peekPrecedence() {
		switch p.peekToken().Type {
		case sozcuk.TOKEN_PLUS, sozcuk.TOKEN_MINUS, sozcuk.TOKEN_SLASH, sozcuk.TOKEN_ASTERISK,
			sozcuk.TOKEN_PERCENT, sozcuk.TOKEN_POWER, sozcuk.TOKEN_MATMUL,
			sozcuk.TOKEN_EQ, sozcuk.TOKEN_NOT_EQ, sozcuk.TOKEN_LT, sozcuk.TOKEN_GT,
			sozcuk.TOKEN_LTE, sozcuk.TOKEN_GTE, sozcuk.TOKEN_AND, sozcuk.TOKEN_OR,
			sozcuk.TOKEN_VE, sozcuk.TOKEN_VEYA, sozcuk.TOKEN_PIPE, sozcuk.TOKEN_NULL_COALESC:
			p.nextToken()
			leftExp = p.parseInfixExpression(leftExp)
		case sozcuk.TOKEN_RANGE, sozcuk.TOKEN_RANGE_INCL:
			p.nextToken()
			leftExp = p.parseRangeExpression(leftExp)
		case sozcuk.TOKEN_LPAREN, sozcuk.TOKEN_LBRACKET:
			// Yeni satırda başlayan '(' veya '[' önceki ifadeyi sürdürmez; yeni bir
			// deyim başlatır. Aksi hâlde 'değişken f = 5' ardından gelen
			// '(1 + 2) |> yazdır' satırı '5(1 + 2)' çağrısı olarak okunurdu.
			if p.peekToken().Line > p.curToken().Line {
				return leftExp
			}
			p.nextToken()
			if p.curTokenIs(sozcuk.TOKEN_LPAREN) {
				leftExp = p.parseCallExpression(leftExp)
			} else {
				leftExp = p.parseIndexExpression(leftExp)
			}
		case sozcuk.TOKEN_DOT, sozcuk.TOKEN_SAFE_DOT:
			p.nextToken()
			leftExp = p.parseMemberExpression(leftExp)
		case sozcuk.TOKEN_LBRACE:
			// Sondaki blok yalnızca aynı satırda ve kontrol yapısı başlığı dışında geçerlidir
			if p.noBlock || p.peekToken().Line != p.curToken().Line || !isTrailingBlockTarget(leftExp) {
				return leftExp
			}
			p.nextToken()
			leftExp = p.parseTrailingBlock(leftExp)
		default:
			return leftExp
		}
	}

	return leftExp
}
func (p *Parser) parseIntegerLiteral() agac.Expression {
	tok := p.curToken()
	val, err := strconv.ParseInt(tok.Literal, 10, 64)
	if err != nil {
		p.addError("%s tam sayı olarak çok büyük (en fazla %d olabilir; ondalık sayı için sonuna .0 ekleyin)", tok.Literal, int64(math.MaxInt64))
		return nil
	}
	return &agac.IntegerLiteral{Token: tok, Value: val}
}
func (p *Parser) parseFloatLiteral() agac.Expression {
	tok := p.curToken()
	val, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		p.addError("%s ondalık sayı olarak çok büyük ya da geçersiz", tok.Literal)
		return nil
	}
	return &agac.FloatLiteral{Token: tok, Value: val}
}
func (p *Parser) parseStringOrInterpolation() agac.Expression {
	tok := p.curToken()
	raw := tok.Literal

	if !strings.Contains(raw, "{") {
		return &agac.StringLiteral{Token: tok, Value: unescapeBraces(raw)}
	}

	// Metin içi {ifade} ayrıştırması. Yerleştirmelerin aralıklarını sözcük çözümleyici metni
	// okurken bulur (iç içe tırnaklı metinlerdeki süslü parantezler sayılmaz).
	runes := []rune(raw)
	var parts []agac.Expression
	var textBuf strings.Builder
	onceki := 0

	for _, y := range p.l.Yerlestirmeler(tok) {
		ac, son := y[0], y[1]
		for _, r := range runes[onceki:ac] {
			kacissizYaz(&textBuf, r)
		}
		acSatir, acSutun := p.l.MetinKonumu(tok, ac) // hata konumu: '{'
		rawExpr := string(runes[ac+1 : son])
		ifade, sorun := p.yerlestirme(tok, runes, ac+1, son)
		kapali := son < len(runes)
		onceki = min(son+1, len(runes)) // '}' geç
		if p.derinlikAsildi {
			return &agac.StringLiteral{Token: tok, Value: ""}
		}
		if !kapali {
			// Kapanmamış '{' sözcük çözümleyicide hatadır (aynı satırda '}' ile bitmeli).
			return &agac.StringLiteral{Token: tok, Value: ""}
		}
		if sorun != "" {
			p.yerlestirmeHatasi(acSatir, acSutun, rawExpr, sorun)
			return &agac.StringLiteral{Token: tok, Value: ""}
		}
		if textBuf.Len() > 0 {
			parts = append(parts, &agac.StringLiteral{Token: tok, Value: textBuf.String()})
			textBuf.Reset()
		}
		parts = append(parts, ifade)
	}
	for _, r := range runes[onceki:] {
		kacissizYaz(&textBuf, r)
	}

	if textBuf.Len() > 0 {
		parts = append(parts, &agac.StringLiteral{Token: tok, Value: textBuf.String()})
	}

	if len(parts) == 1 {
		if sl, ok := parts[0].(*agac.StringLiteral); ok {
			return sl
		}
	}

	return &agac.InterpolatedString{Token: tok, Parts: parts}
}

// yerlestirme metin içindeki bir '{ifade}' bölümünün ifadesini ayrıştırır (2.4b): kaçışsız
// her '{' bir yerleştirme başlatır. Bölüm tek bir ifade olarak ayrıştırılamazsa (ya da
// boşsa) sorun bunu anlatan kısa metindir; düz metne dönülmez. Alt ayrıştırıcı üst
// ayrıştırıcının iç içelik düzeyinden başlar: metin içindeki ifade de dosyanın iç içelik
// sınırına (2.10) sayılır; sınır alt ayrıştırıcıda aşılırsa hata üst ayrıştırıcıya taşınır.
func (p *Parser) yerlestirme(metin sozcuk.Token, deger []rune, bas, son int) (agac.Expression, string) {
	exprCode, ifadeBas := sozcuk.YerlestirmeIfadesi(deger, bas, son)
	if exprCode == "" {
		return nil, "boş"
	}
	// İfadenin sözcükleri dosyadaki gerçek konumlarını taşır (sozcuk.Lexer.Yerlestirme):
	// önündeki kaçış dizileri ve boşluklar konumu kaydırmaz.
	subParser := New(p.l.Yerlestirme(metin, exprCode, ifadeBas))
	subParser.derinlik = p.derinlik
	subParser.roller = p.roller
	subProg := subParser.ParseProgram()
	if subParser.derinlikAsildi {
		if !p.derinlikAsildi {
			p.errors = append(p.errors, subParser.errors[len(subParser.errors)-1])
			p.derinlikAsildi = true
		}
		return nil, "iç içelik sınırı aşıldı"
	}
	if hatalar := subParser.Errors(); len(hatalar) != 0 {
		return nil, "bir ifade değil (" + konumOnekiniAt(hatalar[0]) + ")"
	}
	if len(subProg.Statements) != 1 {
		return nil, "tek bir ifade olmalı"
	}
	es, ok := subProg.Statements[0].(*agac.ExpressionStatement)
	if !ok || es.Expression == nil {
		return nil, "bir ifade değil, bir deyim"
	}
	return es.Expression, ""
}

// yerlestirmeHatasi metin içinde satir:sutun'daki '{' ile başlayan yerleştirme
// ayrıştırılamadığında orada sözdizimi hatası kaydeder ve düz süslü parantezin iki yazımını
// önerir.
func (p *Parser) yerlestirmeHatasi(satir, sutun int, rawExpr, sorun string) {
	p.hataKaydet(sozcuk.Token{Line: satir, Column: sutun}, fmt.Sprintf(
		"Sözdizimi hatası — metindeki her '{' bir yerleştirme başlatır, ama '{%s' %s. Düz süslü parantez için '\\{' yazın ya da ham metin kullanın: `...{...}...`",
		kisaMetin(unescapeBraces(rawExpr))+"}", sorun))
}

// konumOnekiniAt bir alt hata mesajının başındaki "Satır N, Sütun M: " ve "Sözdizimi
// hatası — " öneklerini atar (mesaj yerleştirme hatasının içinde gösterilir).
func konumOnekiniAt(m string) string {
	if strings.HasPrefix(m, "Satır ") {
		if i := strings.Index(m, ": "); i >= 0 {
			m = m[i+2:]
		}
	}
	return strings.TrimPrefix(m, "Sözdizimi hatası — ")
}

// kacissizYaz bir karakteri, sözcük çözümleyicinin '\{' ve '\}' işaretçilerini düz süslü
// parantezlere çevirerek yazar.
func kacissizYaz(sb *strings.Builder, r rune) {
	switch r {
	case sozcuk.EscapedLBrace:
		r = '{'
	case sozcuk.EscapedRBrace:
		r = '}'
	}
	sb.WriteRune(r)
}

// unescapeBraces sözcük çözümleyicinin '\{' ve '\}' için ürettiği işaretçi
// karakterleri düz süslü parantezlere çevirir.
func unescapeBraces(s string) string {
	s = strings.ReplaceAll(s, string(sozcuk.EscapedLBrace), "{")
	return strings.ReplaceAll(s, string(sozcuk.EscapedRBrace), "}")
}
func (p *Parser) parsePrefixExpression() agac.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == sozcuk.TOKEN_DEGIL {
		op = "!"
	}
	p.nextToken()
	prec := PREFIX_PREC
	if tok.Type == sozcuk.TOKEN_DEGIL {
		// 'değil' karşılaştırmadan zayıf bağlar: 'değil 1 == 2' -> değil (1 == 2), ancak
		// 'değil a ve b' -> (değil a) ve b
		prec = AND_PREC
	}
	right := p.parseExpression(prec)
	return &agac.PrefixExpression{Token: tok, Operator: op, Right: right}
}
func (p *Parser) parseInfixExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == sozcuk.TOKEN_VE {
		op = "&&"
	} else if tok.Type == sozcuk.TOKEN_VEYA {
		op = "||"
	}
	prec := p.curPrecedence()
	if tok.Type == sozcuk.TOKEN_POWER {
		// '**' sağdan birleşir: 2 ** 3 ** 2 = 2 ** (3 ** 2)
		prec--
	}
	p.nextToken()
	right := p.parseExpression(prec)
	return &agac.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}
func (p *Parser) parseRangeExpression(left agac.Expression) agac.Expression {
	tok := p.curToken()
	inclusive := tok.Type == sozcuk.TOKEN_RANGE_INCL
	prec := p.curPrecedence()
	p.nextToken()
	right := p.parseExpression(prec)
	return &agac.RangeExpression{
		Token:     tok,
		Start:     left,
		End:       right,
		Inclusive: inclusive,
	}
}
