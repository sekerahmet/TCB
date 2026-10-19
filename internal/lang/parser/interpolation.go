package parser

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
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
		if problem != "" {
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
func (p *Parser) interpolation(text lexer.Token, curValue []rune, begin, end int) (ast.Expression, string) {
	exprCode, exprStart := lexer.InterpolationExpr(curValue, begin, end)
	if exprCode == "" {
		return nil, "boş"
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
		return nil, "iç içelik sınırı aşıldı"
	}
	if errList := subParser.Errors(); len(errList) != 0 {
		return nil, "bir ifade değil (" + stripPosPrefix(errList[0]) + ")"
	}
	if len(subProg.Statements) != 1 {
		return nil, "tek bir ifade olmalı"
	}
	es, ok := subProg.Statements[0].(*ast.ExpressionStatement)
	if !ok || es.Expression == nil {
		return nil, "bir ifade değil, bir deyim"
	}
	return es.Expression, ""
}

// interpolationError metin içinde satir:sutun'daki '{' ile başlayan yerleştirme
// ayrıştırılamadığında orada sözdizimi hatası kaydeder ve düz süslü parantezin iki yazımını
// önerir.
func (p *Parser) interpolationError(lineNo, colNo int, rawExpr, problem string) {
	p.recordError(lexer.Token{Line: lineNo, Column: colNo}, fmt.Sprintf(
		"Sözdizimi hatası — metindeki her '{' bir yerleştirme başlatır, ama '{%s' %s. Düz süslü parantez için '\\{' yazın ya da ham metin kullanın: `...{...}...`",
		shortText(unescapeBraces(rawExpr))+"}", problem))
}

// stripPosPrefix bir alt hata mesajının başındaki "Satır N, Sütun M: " ve "Sözdizimi
// hatası — " öneklerini atar (mesaj yerleştirme hatasının içinde gösterilir).
func stripPosPrefix(m string) string {
	if strings.HasPrefix(m, "Satır ") {
		if i := strings.Index(m, ": "); i >= 0 {
			m = m[i+2:]
		}
	}
	return strings.TrimPrefix(m, "Sözdizimi hatası — ")
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
