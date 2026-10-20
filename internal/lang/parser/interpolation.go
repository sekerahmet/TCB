package parser

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func (p *Parser) parseStringOrInterpolation() ast.Expression {
	tok := p.curToken()
	raw := tok.Literal

	if !strings.Contains(raw, "{") {
		return &ast.StringLiteral{Token: tok, Value: unescapeBraces(raw)}
	}

	// Metin içi {ifade} ayrıştırması. Yerleştirmelerin aralıklarını sözcük çözümleyici metni
	// okurken bulur (iç içe tırnaklı metinlerdeki süslü parantezler sayılmaz).
	runes := []rune(raw)
	var parts []ast.Expression
	var textBuf strings.Builder
	prev := 0

	for _, y := range p.l.Interpolations(tok) {
		opener, end := y[0], y[1]
		for _, r := range runes[prev:opener] {
			writeUnescaped(&textBuf, r)
		}
		openLine, openCol := p.l.StringPos(tok, opener) // hata konumu: '{'
		rawExpr := string(runes[opener+1 : end])
		expression, problem := p.interpolation(tok, runes, opener+1, end)
		closed := end < len(runes)
		prev = min(end+1, len(runes)) // '}' geç
		if p.depthExceeded {
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		if !closed {
			// Kapanmamış '{' sözcük çözümleyicide hatadır (aynı satırda '}' ile bitmeli).
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		if problem != nil {
			p.interpolationError(openLine, openCol, rawExpr, problem)
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		if textBuf.Len() > 0 {
			parts = append(parts, &ast.StringLiteral{Token: tok, Value: textBuf.String()})
			textBuf.Reset()
		}
		parts = append(parts, expression)
	}
	for _, r := range runes[prev:] {
		writeUnescaped(&textBuf, r)
	}

	if textBuf.Len() > 0 {
		parts = append(parts, &ast.StringLiteral{Token: tok, Value: textBuf.String()})
	}

	if len(parts) == 1 {
		if sl, ok := parts[0].(*ast.StringLiteral); ok {
			return sl
		}
	}

	return &ast.InterpolatedString{Token: tok, Parts: parts}
}

// interpolation metin içindeki bir '{ifade}' bölümünün ifadesini ayrıştırır (2.4b): kaçışsız
// her '{' bir yerleştirme başlatır. Bölüm tek bir ifade olarak ayrıştırılamazsa (ya da
// boşsa) sorun bunu anlatan kısa metindir; düz metne dönülmez. Alt ayrıştırıcı üst
// ayrıştırıcının iç içelik düzeyinden başlar: metin içindeki ifade de dosyanın iç içelik
// sınırına (2.10) sayılır; sınır alt ayrıştırıcıda aşılırsa hata üst ayrıştırıcıya taşınır.
func (p *Parser) interpolation(text lexer.Token, curValue []rune, begin, end int) (ast.Expression, *interpolationProblem) {
	exprCode, exprStart := lexer.InterpolationExpr(curValue, begin, end)
	if exprCode == "" {
		return nil, &interpolationProblem{code: messages.H0158}
	}
	// İfadenin sözcükleri dosyadaki gerçek konumlarını taşır (sozcuk.Lexer.Yerlestirme):
	// önündeki kaçış dizileri ve boşluklar konumu kaydırmaz.
	subParser := New(p.l.Interpolation(text, exprCode, exprStart))
	subParser.nesting = p.nesting
	subParser.roles = p.roles
	subProg := subParser.ParseProgram()
	if subParser.depthExceeded {
		if !p.depthExceeded {
			p.errors = append(p.errors, subParser.errors[len(subParser.errors)-1])
			p.depthExceeded = true
		}
		return nil, &interpolationProblem{code: messages.H0159}
	}
	if errList := subParser.Errs(); len(errList) != 0 {
		return nil, &interpolationProblem{code: messages.H0160, detail: plainError(errList[0])}
	}
	if len(subProg.Statements) != 1 {
		return nil, &interpolationProblem{code: messages.H0161}
	}
	es, ok := subProg.Statements[0].(*ast.ExpressionStatement)
	if !ok || es.Expression == nil {
		return nil, &interpolationProblem{code: messages.H0162}
	}
	return es.Expression, nil
}

// interpolationError metin içinde satir:sutun'daki '{' ile başlayan yerleştirme
// ayrıştırılamadığında orada sözdizimi hatası kaydeder ve düz süslü parantezin iki yazımını
// önerir.
func (p *Parser) interpolationError(lineNo, colNo int, rawExpr string, problem *interpolationProblem) {
	args := []any{shortText(unescapeBraces(rawExpr))}
	if problem.code == messages.H0160 {
		args = append(args, problem.detail)
	}
	p.syntaxError(lexer.Token{Line: lineNo, Column: colNo}, problem.code, args...)
}

// interpolationProblem yerleştirmenin neden ayrıştırılamadığıdır: her neden kendi tam
// cümlesi olan bir ileti kodudur (H0158-H0162); detail yalnızca H0160'ın alt hata metnidir.
type interpolationProblem struct {
	code   messages.Code
	detail string
}

// plainError bir alt hatanın konum ve "Sözdizimi hatası — " öneki olmadan metnini verir (ileti
// yerleştirme hatasının içinde gösterilir).
func plainError(err error) string {
	if pe, ok := err.(*messages.PosError); ok {
		err = pe.Err
	}
	if m, ok := err.(*messages.Error); ok && m.Code == messages.H0100 && len(m.Args) == 1 {
		return fmt.Sprint(m.Args[0])
	}
	return err.Error()
}

// writeUnescaped bir karakteri, sözcük çözümleyicinin '\{' ve '\}' işaretçilerini düz süslü
// parantezlere çevirerek yazar.
func writeUnescaped(sb *strings.Builder, r rune) {
	switch r {
	case lexer.EscapedLBrace:
		r = '{'
	case lexer.EscapedRBrace:
		r = '}'
	}
	sb.WriteRune(r)
}

// unescapeBraces sözcük çözümleyicinin '\{' ve '\}' için ürettiği işaretçi
// karakterleri düz süslü parantezlere çevirir.
func unescapeBraces(s string) string {
	s = strings.ReplaceAll(s, string(lexer.EscapedLBrace), "{")
	return strings.ReplaceAll(s, string(lexer.EscapedRBrace), "}")
}
