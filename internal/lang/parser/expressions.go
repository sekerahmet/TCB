package parser

import (
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- İFADE ÇÖZÜMLEME (PRATT PARSER) ---

func (p *Parser) parseExpression(precedence int) ast.Expression {
	if !p.enterDepth() {
		return nil
	}
	defer p.leaveDepth()
	var leftExp ast.Expression
	pattern := p.patternMode
	p.patternMode = false

	switch p.curToken().Type {
	case lexer.TokIdent:
		// Tek parametreli ok fonksiyonu: x => x * 2 (eşle deseninde ad, değeriyle karşılaştırılır)
		if !pattern && p.peekTokenIs(lexer.TokArrow) {
			leftExp = p.parseSingleParamLambda()
		} else {
			leftExp = &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal}
		}
	case lexer.TokSelf:
		leftExp = &ast.Identifier{Token: p.curToken(), Value: p.words.self}
	case lexer.TokInt:
		leftExp = p.parseIntegerLiteral()
	case lexer.TokFloat:
		leftExp = p.parseFloatLiteral()
	case lexer.TokString:
		leftExp = p.parseStringOrInterpolation()
	case lexer.TokTrue:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: true}
	case lexer.TokFalse:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: false}
	case lexer.TokNil:
		leftExp = &ast.NullLiteral{Token: p.curToken()}
	case lexer.TokBang, lexer.TokMinus, lexer.TokTilde, lexer.TokKwNot:
		leftExp = p.parsePrefixExpression()
	case lexer.TokLParen:
		leftExp = p.parseGroupedOrLambdaExpression(pattern)
	case lexer.TokIf:
		leftExp = p.parseIfExpression()
	case lexer.TokMatch:
		leftExp = p.parseMatchExpression()
	case lexer.TokFunction:
		leftExp = p.parseFunctionLiteral()
	case lexer.TokLBracket:
		leftExp = p.parseListLiteral()
	case lexer.TokLBrace:
		leftExp = p.parseDictLiteral()
	default:
		if p.leadingBinaryOperator() {
			p.syntaxError(p.curToken(), messages.H0167, p.tokenDescription(p.curToken()))
			return nil
		}
		p.syntaxError(p.curToken(), messages.H0141, p.tokenDescription(p.curToken()))
		return nil
	}

	return p.parseInfixChain(leftExp, precedence)
}
func (p *Parser) parseIntegerLiteral() ast.Expression {
	tok := p.curToken()
	// Ön ekli yazımın (0x1F, 0o17, 0b1010; 2.4) tabanı ön ekten okunur. Ön eksiz yazım her zaman
	// ondalıktır (017, 17'dir; sekizlik değil).
	digits, base := tok.Literal, 10
	if len(digits) > 2 && digits[0] == '0' {
		switch digits[1] {
		case 'x':
			digits, base = digits[2:], 16
		case 'o':
			digits, base = digits[2:], 8
		case 'b':
			digits, base = digits[2:], 2
		}
	}
	val, err := strconv.ParseInt(digits, base, 64)
	if err != nil {
		p.recordError(p.curToken(), messages.New(messages.H0165, tok.Literal, int64(math.MaxInt64)))
		return nil
	}
	return &ast.IntegerLiteral{Token: tok, Value: val}
}
func (p *Parser) parseFloatLiteral() ast.Expression {
	tok := p.curToken()
	val, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		p.recordError(p.curToken(), messages.New(messages.H0166, tok.Literal))
		return nil
	}
	return &ast.FloatLiteral{Token: tok, Value: val}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	tok := p.curToken()
	if tok.Type == lexer.TokBang {
		// Tekli '!' C ailesinin yazımıdır (4.9); TR'de TR_not.
		p.syntaxError(tok, messages.H0204)
	}
	op := lexer.TokenType(tok.Literal)
	if tok.Type == lexer.TokKwNot {
		op = "!"
	}
	p.nextToken()
	prec := precPrefix
	if tok.Type == lexer.TokKwNot {
		// TR_not karşılaştırmadan gevşek bağlar (4.1): 'TR_not 1 == 2' -> TR_not (1 == 2), ancak
		// 'TR_not a TR_and b' -> (TR_not a) TR_and b
		prec = precNot
	}
	right := p.parseExpression(prec)
	return &ast.PrefixExpression{Token: tok, Operator: op, Right: right}
}
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	// Başka dillerin işleçleri (4.9): ileti TR'deki yazımı söyler.
	switch tok.Type {
	case lexer.TokAnd:
		p.syntaxError(tok, messages.H0202)
	case lexer.TokOr:
		p.syntaxError(tok, messages.H0203)
	case lexer.TokPipe:
		p.syntaxError(tok, messages.H0205)
	}
	op := lexer.TokenType(tok.Literal)
	if tok.Type == lexer.TokKwAnd {
		op = "&&"
	} else if tok.Type == lexer.TokKwOr {
		op = "||"
	}
	prec := p.curPrecedence()
	if tok.Type == lexer.TokPower {
		// '**' sağdan birleşir: 2 ** 3 ** 2 = 2 ** (3 ** 2)
		prec--
	}
	p.nextToken()
	if tok.Type == lexer.TokSlashSlash && !p.startsExpression() {
		// '// not' C ailesinin yorumudur (4.9): tam bölmenin sağında ifade yoksa ileti '#'i önerir.
		p.syntaxError(p.curToken(), messages.H0025)
		return nil
	}
	if prec == precCompare {
		return p.parseComparison(&ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: p.parseExpression(prec)})
	}
	if tok.Type == lexer.TokNullCoalesc {
		return p.parseNullCoalescing(tok, left)
	}
	right := p.parseExpression(prec)
	return &ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}

// parseComparison karşılaştırma zincirini okur (4.4): 'a < b <= c', 'a < b TR_and b <= c' gibi
// okunur, ortadaki ifade bir kez hesaplanır. Zincirin her halkası sola bağlı bir InfixExpression'dır
// (Chained: soldaki halkanın sağ ifadesiyle karşılaştırır). İçinde '==' ya da '!=' olan zincir
// sözdizimi hatasıdır: 'a == b == c' Python'da ve C'de ayrı anlamlara gelir.
func (p *Parser) parseComparison(first *ast.InfixExpression) ast.Expression {
	chain := first
	for precedences[p.peekToken().Type] == precCompare && !p.peekTokenIs(lexer.TokSemicolon) {
		if p.lineBreakEnds() {
			break
		}
		p.nextToken()
		tok := p.curToken()
		if equality(chain.Operator) || tok.Type == lexer.TokEq || tok.Type == lexer.TokNotEq {
			p.syntaxError(tok, messages.H0200, tok.Literal)
			return nil
		}
		p.nextToken()
		chain = &ast.InfixExpression{Token: tok, Left: chain, Operator: lexer.TokenType(tok.Literal), Right: p.parseExpression(precCompare), Chained: true}
	}
	return chain
}

// equality op '==' ya da '!=' mi; zincirin önceki halkaları da sınanır.
func equality(op lexer.TokenType) bool { return op == lexer.TokEq || op == lexer.TokNotEq }

// parseNullCoalescing 'a ?? b'yi okur (4.7). Sağ yanda parantezsiz bir ikili işleç sözdizimi
// hatasıdır: 'd.TR_get(k) ?? 0 + 1' öncelikle '?? (0 + 1)' okunurdu, Python'un
// 'd.get(k, 0) + 1' alışkanlığı sessizce yanlış sayardı. Sağ yan tekli işleçleri ve sondaki
// çağrı, indeks ve üyeyi alır; ardındaki ikili işleç hata verir.
func (p *Parser) parseNullCoalescing(tok lexer.Token, left ast.Expression) ast.Expression {
	right := p.parseExpression(precPower)
	next := p.peekToken()
	if precedences[next.Type] > precNullCoalesc && binaryOperator(next.Type) && !p.lineBreakEnds() {
		p.nextToken()
		p.syntaxError(next, messages.H0201, next.Literal)
		return nil
	}
	return &ast.InfixExpression{Token: tok, Left: left, Operator: lexer.TokenType(tok.Literal), Right: right}
}

// binaryOperator t iki işlenenli bir işleç mi (çağrı, indeks, üye ve sondaki blok değil).
func binaryOperator(t lexer.TokenType) bool {
	switch t {
	case lexer.TokLParen, lexer.TokLBracket, lexer.TokDot, lexer.TokSafeDot, lexer.TokLBrace, lexer.TokArrow:
		return false
	}
	_, ok := precedences[t]
	return ok
}

// startsExpression şimdiki belirteç aynı satırda bir ifade başlatabilir mi (ad, değer, tekli işleç,
// açan parantez ya da ifade olan anahtar kelime)?
func (p *Parser) startsExpression() bool {
	if p.pos > 0 && p.tokens[p.pos-1].Line < p.curToken().Line {
		return false
	}
	switch p.curToken().Type {
	case lexer.TokIdent, lexer.TokSelf, lexer.TokInt, lexer.TokFloat, lexer.TokString,
		lexer.TokTrue, lexer.TokFalse, lexer.TokNil, lexer.TokBang, lexer.TokMinus, lexer.TokTilde,
		lexer.TokKwNot, lexer.TokLParen, lexer.TokIf, lexer.TokMatch, lexer.TokFunction,
		lexer.TokLBracket, lexer.TokLBrace:
		return true
	}
	return false
}
func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	inclusive := tok.Type == lexer.TokRangeIncl
	prec := p.curPrecedence()
	p.nextToken()
	right := p.parseExpression(prec)
	return &ast.RangeExpression{
		Token:     tok,
		Start:     left,
		End:       right,
		Inclusive: inclusive,
	}
}

// parseInfixChain sol ifadeye ikili işleçleri, çağrıları, indeksleri ve üyeleri öncelik
// sırasıyla bağlar.
func (p *Parser) parseInfixChain(leftExp ast.Expression, precedence int) ast.Expression {
	for !p.peekTokenIs(lexer.TokSemicolon) && precedence < p.peekPrecedence() {
		switch p.peekToken().Type {
		case lexer.TokPlus, lexer.TokMinus, lexer.TokSlash, lexer.TokSlashSlash, lexer.TokAsterisk,
			lexer.TokPercent, lexer.TokPower, lexer.TokMatmul,
			lexer.TokEq, lexer.TokNotEq, lexer.TokLt, lexer.TokGt,
			lexer.TokLte, lexer.TokGte, lexer.TokAnd, lexer.TokOr,
			lexer.TokKwAnd, lexer.TokKwOr, lexer.TokPipe, lexer.TokNullCoalesc,
			lexer.TokBitAnd, lexer.TokBitOr, lexer.TokBitXor, lexer.TokShl, lexer.TokShr:
			// Yeni satırın başındaki ikili işleç önceki ifadeyi sürdürmez (2.8): 'a = 1'
			// ardından gelen '-x' satırı 'a = 1 - x' okunmaz (Go, Python, Kotlin gibi). İfadeyi
			// bölmek için işleç satır sonuna yazılır ya da parantez kullanılır. Başka dillerin boru
			// işleci '|>' satır başında da okunur ki hatası (4.9, H0205) yerinde çıksın.
			if p.peekToken().Type != lexer.TokPipe && p.lineBreakEnds() {
				return leftExp
			}
			p.nextToken()
			leftExp = p.parseInfixExpression(leftExp)
		case lexer.TokRange, lexer.TokRangeIncl:
			if p.lineBreakEnds() {
				return leftExp
			}
			p.nextToken()
			leftExp = p.parseRangeExpression(leftExp)
		case lexer.TokLParen, lexer.TokLBracket:
			// Yeni satırda başlayan '(' veya '[' önceki ifadeyi sürdürmez; yeni bir
			// deyim başlatır. Aksi hâlde 'değişken f = 5' ardından gelen
			// '(1 + 2) |> yazdır' satırı '5(1 + 2)' çağrısı olarak okunurdu.
			if p.peekToken().Line > p.curToken().Line {
				return leftExp
			}
			p.nextToken()
			if p.curTokenIs(lexer.TokLParen) {
				leftExp = p.parseCallExpression(leftExp)
			} else {
				leftExp = p.parseIndexExpression(leftExp)
			}
		case lexer.TokDot, lexer.TokSafeDot:
			p.nextToken()
			leftExp = p.parseMemberExpression(leftExp)
		case lexer.TokLBrace:
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
	if p.peekTokenIs(lexer.TokIn) && !p.lineBreakEnds() {
		// Python'un üyelik sınaması 'x TR_in l' yoktur (4.9); TR_in döngü ve liste üreteci
		// başlığında kendi yerinde okunur, bir ifadenin ardında hiç geçmez.
		p.syntaxError(p.peekToken(), messages.H0206)
	}

	return leftExp
}

// leadingBinaryOperator şimdiki belirteç satırın ilk belirteci olan bir ikili işleç mi: önceki
// satırı sürdürmek için yazılmış olabilir (2.8), hata nedenini söyler.
func (p *Parser) leadingBinaryOperator() bool {
	switch p.curToken().Type {
	case lexer.TokPlus, lexer.TokSlash, lexer.TokSlashSlash, lexer.TokAsterisk, lexer.TokPercent,
		lexer.TokPower, lexer.TokMatmul, lexer.TokEq, lexer.TokNotEq, lexer.TokLt,
		lexer.TokGt, lexer.TokLte, lexer.TokGte, lexer.TokAnd, lexer.TokOr,
		lexer.TokKwAnd, lexer.TokKwOr, lexer.TokNullCoalesc, lexer.TokRange,
		lexer.TokRangeIncl, lexer.TokBitAnd, lexer.TokBitOr, lexer.TokBitXor, lexer.TokShl,
		lexer.TokShr:
	default:
		return false
	}
	return p.pos == 0 || p.tokens[p.pos-1].Line < p.curToken().Line
}
