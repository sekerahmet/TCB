package ayristir

import (
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

	// Metin içi {ifade} ayrıştırması
	runes := []rune(raw)
	var parts []agac.Expression
	var textBuf strings.Builder
	i := 0

	for i < len(runes) {
		if runes[i] == '{' {
			i++ // '{' geç
			exprStart := i
			depth := 1
			var exprBuf strings.Builder
			for i < len(runes) && depth > 0 {
				if runes[i] == '{' {
					depth++
				} else if runes[i] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				exprBuf.WriteRune(runes[i])
				i++
			}
			if i < len(runes) && runes[i] == '}' {
				i++ // '}' geç
			}
			rawExpr := exprBuf.String()
			exprCode := strings.TrimSpace(rawExpr)
			interpolated := false
			if exprCode != "" {
				// İfadenin dosyadaki konumu: metnin açılış tırnağından sonraki karakter
				// sayısı kadar ilerisi (önündeki kaçış dizileri konumu bir-iki sütun kaydırabilir)
				line, col := interpolationPos(tok, runes[:exprStart])
				col += len([]rune(rawExpr)) - len([]rune(strings.TrimLeft(rawExpr, " \t")))
				subLex := sozcuk.NewAt(exprCode, line, col)
				subParser := New(subLex)
				subProg := subParser.ParseProgram()
				if len(subParser.Errors()) == 0 && len(subProg.Statements) == 1 {
					if es, ok := subProg.Statements[0].(*agac.ExpressionStatement); ok && es.Expression != nil {
						if textBuf.Len() > 0 {
							parts = append(parts, &agac.StringLiteral{Token: tok, Value: textBuf.String()})
							textBuf.Reset()
						}
						parts = append(parts, es.Expression)
						interpolated = true
					}
				}
			}
			if !interpolated {
				textBuf.WriteString("{" + unescapeBraces(exprBuf.String()) + "}")
			}
			continue
		}
		textBuf.WriteString(unescapeBraces(string(runes[i])))
		i++
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

// interpolationPos metin sabitinde, 'onceki' karakterlerinden sonra başlayan yerleştirme
// ifadesinin dosyadaki satır ve sütununu hesaplar.
func interpolationPos(tok sozcuk.Token, onceki []rune) (int, int) {
	line, col := tok.Line, tok.Column+1 // açılış tırnağından sonraki ilk karakter
	for _, r := range onceki {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
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
