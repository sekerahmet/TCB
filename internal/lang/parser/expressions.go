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
	case lexer.TokBang, lexer.TokMinus, lexer.TokKwNot:
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
			p.syntaxError(p.curToken(), messages.H0167, tokenDescription(p.curToken()))
			return nil
		}
		p.syntaxError(p.curToken(), messages.H0141, tokenDescription(p.curToken()))
		return nil
	}

	return p.parseInfixChain(leftExp, precedence)
}
func (p *Parser) parseIntegerLiteral() ast.Expression {
	tok := p.curToken()
	val, err := strconv.ParseInt(tok.Literal, 10, 64)
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
	op := lexer.TokenType(tok.Literal)
	if tok.Type == lexer.TokKwNot {
		op = "!"
	}
	p.nextToken()
	prec := precPrefix
	if tok.Type == lexer.TokKwNot {
		// 'değil' karşılaştırmadan zayıf bağlar: 'değil 1 == 2' -> değil (1 == 2), ancak
		// 'değil a ve b' -> (değil a) ve b
		prec = precAnd
	}
	right := p.parseExpression(prec)
	return &ast.PrefixExpression{Token: tok, Operator: op, Right: right}
}
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
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
	right := p.parseExpression(prec)
	return &ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
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
		case lexer.TokPlus, lexer.TokMinus, lexer.TokSlash, lexer.TokAsterisk,
			lexer.TokPercent, lexer.TokPower, lexer.TokMatmul,
			lexer.TokEq, lexer.TokNotEq, lexer.TokLt, lexer.TokGt,
			lexer.TokLte, lexer.TokGte, lexer.TokAnd, lexer.TokOr,
			lexer.TokKwAnd, lexer.TokKwOr, lexer.TokPipe, lexer.TokNullCoalesc:
			// Yeni satırın başındaki ikili işleç önceki ifadeyi sürdürmez (2.8): 'a = 1'
			// ardından gelen '-x' satırı 'a = 1 - x' okunmaz (Go, Python, Kotlin gibi). İfadeyi
			// bölmek için işleç satır sonuna yazılır ya da parantez kullanılır. Zincir işleci
			// '|>' tekli biçimi olmadığı için satır başında da sürer ('.' gibi; Elixir, F#).
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

	return leftExp
}

// leadingBinaryOperator şimdiki belirteç satırın ilk belirteci olan bir ikili işleç mi: önceki
// satırı sürdürmek için yazılmış olabilir (2.8), hata nedenini söyler.
func (p *Parser) leadingBinaryOperator() bool {
	switch p.curToken().Type {
	case lexer.TokPlus, lexer.TokSlash, lexer.TokAsterisk, lexer.TokPercent,
		lexer.TokPower, lexer.TokMatmul, lexer.TokEq, lexer.TokNotEq, lexer.TokLt,
		lexer.TokGt, lexer.TokLte, lexer.TokGte, lexer.TokAnd, lexer.TokOr,
		lexer.TokKwAnd, lexer.TokKwOr, lexer.TokNullCoalesc, lexer.TokRange,
		lexer.TokRangeIncl:
	default:
		return false
	}
	return p.pos == 0 || p.tokens[p.pos-1].Line < p.curToken().Line
}
