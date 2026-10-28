package parser

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// parseStringOrInterpolation bir metin sabitini okur. Yerleştirme içermeyen metin olduğu gibi bir
// metin değeridir; '\(ifade)' yerleştirmeleri (2.7) sözcük çözümleyicinin kaydettiği aralıklardan
// bulunur ve her biri alt ayrıştırıcıyla okunur. Süslü parantez sıradan bir karakterdir.
func (p *Parser) parseStringOrInterpolation() ast.Expression {
	tok := p.curToken()
	spans := p.l.Interpolations(tok)
	if len(spans) == 0 {
		return &ast.StringLiteral{Token: tok, Value: tok.Literal}
	}
	runes := []rune(tok.Literal)
	var parts []ast.Expression
	prev := 0
	for _, y := range spans {
		opener, end := y[0], y[1]
		if opener > prev {
			parts = append(parts, &ast.StringLiteral{Token: tok, Value: string(runes[prev:opener])})
		}
		if end >= len(runes) {
			// Kapanmamış '\(' sözcük çözümleyicide hatadır (aynı satırda ')' ile bitmeli).
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		openLine, openCol := p.l.StringPos(tok, opener) // hata konumu: '('
		rawExpr := string(runes[opener+1 : end])
		expression, problem := p.interpolation(tok, runes, opener+1, end)
		prev = end + 1 // ')' geç
		if p.depthExceeded {
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		if problem != nil {
			p.interpolationError(openLine, openCol-1, rawExpr, problem)
			return &ast.StringLiteral{Token: tok, Value: ""}
		}
		parts = append(parts, expression)
	}
	if prev < len(runes) {
		parts = append(parts, &ast.StringLiteral{Token: tok, Value: string(runes[prev:])})
	}
	return &ast.InterpolatedString{Token: tok, Parts: parts}
}

// interpolation metin içindeki bir '\(ifade)' bölümünün ifadesini ayrıştırır (2.7). Bölüm tek bir
// ifade olarak ayrıştırılamazsa (ya da boşsa) sorun bunu anlatan kısa metindir. Alt ayrıştırıcı üst
// ayrıştırıcının iç içelik düzeyinden başlar: metin içindeki ifade de dosyanın iç içelik sınırına
// (2.10) sayılır; sınır alt ayrıştırıcıda aşılırsa hata üst ayrıştırıcıya taşınır.
func (p *Parser) interpolation(text lexer.Token, curValue []rune, begin, end int) (ast.Expression, *interpolationProblem) {
	exprCode, exprStart := lexer.InterpolationExpr(curValue, begin, end)
	if exprCode == "" {
		return nil, &interpolationProblem{code: messages.H0158}
	}
	// İfadenin sözcükleri dosyadaki gerçek konumlarını taşır (lexer.Interpolation): önündeki kaçış
	// dizileri ve boşluklar konumu kaydırmaz.
	subParser := New(p.l.Interpolation(text, exprCode, exprStart))
	subParser.nesting = p.nesting
	subParser.roles = p.roles
	subParser.interpolated = true
	subProg := subParser.ParseProgram()
	p.underscore = p.underscore || subParser.underscore
	p.ifWithoutElse = p.ifWithoutElse || subParser.ifWithoutElse
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

// interpolationError metin içinde satır:sütun'daki '\(' ile başlayan yerleştirme ayrıştırılamadığında
// orada sözdizimi hatası kaydeder.
func (p *Parser) interpolationError(lineNo, colNo int, rawExpr string, problem *interpolationProblem) {
	args := []any{shortText(rawExpr)}
	if problem.code == messages.H0160 {
		args = append(args, problem.detail)
	}
	// The error is about the whole part, but its end in the file is not known here: the range covers
	// the two characters of the '\(' that opens it.
	p.syntaxError(lexer.Token{Line: lineNo, Column: colNo, EndLine: lineNo, EndColumn: colNo + 2}, problem.code, args...)
}

// interpolationProblem yerleştirmenin neden ayrıştırılamadığıdır: her neden kendi tam
// cümlesi olan bir ileti kodudur (H0158-H0162); detail yalnızca H0160'ın alt hatasıdır.
type interpolationProblem struct {
	code   messages.Code
	detail any
}

// plainError bir alt hatayı konum ve "Sözdizimi hatası — " öneki olmadan verir (ileti yerleştirme
// hatasının içinde gösterilir). Hata metne çevrilmeden geçer: dış iletinin dilinde yazılır.
func plainError(err error) any {
	if pe, ok := err.(*messages.PosError); ok {
		err = pe.Err
	}
	if m, ok := err.(*messages.Error); ok && m.Code == messages.H0100 && len(m.Args) == 1 {
		return m.Args[0]
	}
	return err
}
