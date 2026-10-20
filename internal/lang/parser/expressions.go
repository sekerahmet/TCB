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
	case lexer.TOKEN_IDENT:
		// Tek parametreli ok fonksiyonu: x => x * 2 (eşle deseninde ad, değeriyle karşılaştırılır)
		if !pattern && p.peekTokenIs(lexer.TOKEN_ARROW) {
			leftExp = p.parseSingleParamLambda()
		} else {
			leftExp = &ast.Identifier{Token: p.curToken(), Value: p.curToken().Literal}
		}
	case lexer.TOKEN_SELF:
		leftExp = &ast.Identifier{Token: p.curToken(), Value: selfName}
	case lexer.TOKEN_INT:
		leftExp = p.parseIntegerLiteral()
	case lexer.TOKEN_FLOAT:
		leftExp = p.parseFloatLiteral()
	case lexer.TOKEN_STRING:
		leftExp = p.parseStringOrInterpolation()
	case lexer.TOKEN_TRUE:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: true}
	case lexer.TOKEN_FALSE:
		leftExp = &ast.BooleanLiteral{Token: p.curToken(), Value: false}
	case lexer.TOKEN_NIL:
		leftExp = &ast.NullLiteral{Token: p.curToken()}
	case lexer.TOKEN_BANG, lexer.TOKEN_MINUS, lexer.TOKEN_KW_NOT:
		leftExp = p.parsePrefixExpression()
	case lexer.TOKEN_LPAREN:
		leftExp = p.parseGroupedOrLambdaExpression(pattern)
	case lexer.TOKEN_IF:
		leftExp = p.parseIfExpression()
	case lexer.TOKEN_MATCH:
		leftExp = p.parseMatchExpression()
	case lexer.TOKEN_FUNCTION:
		leftExp = p.parseFunctionLiteral()
	case lexer.TOKEN_LBRACKET:
		leftExp = p.parseListLiteral()
	case lexer.TOKEN_LBRACE:
		leftExp = p.parseDictLiteral()
	default:
		p.syntaxError(p.curToken(), messages.H0141, tokenDescription(p.curToken()))
		return nil
	}

	for !p.peekTokenIs(lexer.TOKEN_SEMICOLON) && precedence < p.peekPrecedence() {
		switch p.peekToken().Type {
		case lexer.TOKEN_PLUS, lexer.TOKEN_MINUS, lexer.TOKEN_SLASH, lexer.TOKEN_ASTERISK,
			lexer.TOKEN_PERCENT, lexer.TOKEN_POWER, lexer.TOKEN_MATMUL,
			lexer.TOKEN_EQ, lexer.TOKEN_NOT_EQ, lexer.TOKEN_LT, lexer.TOKEN_GT,
			lexer.TOKEN_LTE, lexer.TOKEN_GTE, lexer.TOKEN_AND, lexer.TOKEN_OR,
			lexer.TOKEN_KW_AND, lexer.TOKEN_KW_OR, lexer.TOKEN_PIPE, lexer.TOKEN_NULL_COALESC:
			p.nextToken()
			leftExp = p.parseInfixExpression(leftExp)
		case lexer.TOKEN_RANGE, lexer.TOKEN_RANGE_INCL:
			p.nextToken()
			leftExp = p.parseRangeExpression(leftExp)
		case lexer.TOKEN_LPAREN, lexer.TOKEN_LBRACKET:
			// Yeni satırda başlayan '(' veya '[' önceki ifadeyi sürdürmez; yeni bir
			// deyim başlatır. Aksi hâlde 'değişken f = 5' ardından gelen
			// '(1 + 2) |> yazdır' satırı '5(1 + 2)' çağrısı olarak okunurdu.
			if p.peekToken().Line > p.curToken().Line {
				return leftExp
			}
			p.nextToken()
			if p.curTokenIs(lexer.TOKEN_LPAREN) {
				leftExp = p.parseCallExpression(leftExp)
			} else {
				leftExp = p.parseIndexExpression(leftExp)
			}
		case lexer.TOKEN_DOT, lexer.TOKEN_SAFE_DOT:
			p.nextToken()
			leftExp = p.parseMemberExpression(leftExp)
		case lexer.TOKEN_LBRACE:
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
	op := tok.Literal
	if tok.Type == lexer.TOKEN_KW_NOT {
		op = "!"
	}
	p.nextToken()
	prec := PREFIX_PREC
	if tok.Type == lexer.TOKEN_KW_NOT {
		// 'değil' karşılaştırmadan zayıf bağlar: 'değil 1 == 2' -> değil (1 == 2), ancak
		// 'değil a ve b' -> (değil a) ve b
		prec = AND_PREC
	}
	right := p.parseExpression(prec)
	return &ast.PrefixExpression{Token: tok, Operator: op, Right: right}
}
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	op := tok.Literal
	if tok.Type == lexer.TOKEN_KW_AND {
		op = "&&"
	} else if tok.Type == lexer.TOKEN_KW_OR {
		op = "||"
	}
	prec := p.curPrecedence()
	if tok.Type == lexer.TOKEN_POWER {
		// '**' sağdan birleşir: 2 ** 3 ** 2 = 2 ** (3 ** 2)
		prec--
	}
	p.nextToken()
	right := p.parseExpression(prec)
	return &ast.InfixExpression{Token: tok, Left: left, Operator: op, Right: right}
}
func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken()
	inclusive := tok.Type == lexer.TOKEN_RANGE_INCL
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
