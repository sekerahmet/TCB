// Package formatter TR kaynak kodunu standart biçime getirir ('tr biçimle').
//
// Biçimlendirici sözcük birimi (token) tabanlıdır: kaynak, yorumlar da dahil olmak üzere
// sözcük birimlerine ayrılır; yalnızca aralarındaki boşluklar ve girintiler yeniden
// yazılır. Sözcük birimlerinin kendisi (metin sabitleri bayt bayt) ve yorumların içeriği
// korunur. Satır yapısı da korunur; tek istisna, bir gövdenin '{' işaretinin ve
// 'değilse'/'yakala' sözcüklerinin önceki satıra alınmasıdır.
//
// Standart biçim: 4 boşluk girinti, ikili operatörlerin iki yanında ve virgülden sonra
// birer boşluk, '{' aynı satırda, satır sonunda boşluk yok, art arda en fazla bir boş
// satır ve dosya sonunda tek bir satır sonu.
//
// Güvenlik: Biçimlenmiş kod yeniden ayrıştırılır; sözcük birimi dizisi ya da AST dökümü
// (ast.Dump) değişmişse sonuç reddedilir. Böylece biçimlendirici programın anlamını asla
// değiştirmez.
//
// Layer: developer tools (internal/devtools); dil, motor ve hesap katmanlarını içe aktarabilir.
package formatter

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"strings"
)

const indentUnit = "    "

// tokComment bir yorum öğesinin türüdür (sözcük birimi türleriyle karışmaz).
const tokComment lexer.TokenType = "YORUM"

// item biçimlendirilecek akıştaki bir öğedir: sözcük birimi ya da yorum.
type item struct {
	kind     lexer.TokenType
	text     string // kaynaktaki ham metin
	lineA    int    // öncesindeki boşlukta geçen satır sonu sayısı
	line     int    // sözcük biriminin kaynak satırı (yorumlarda 0)
	col      int
	adjacent bool // kaynakta bir önceki öğeye boşluksuz bitişik mi ('->' tanımak için)
	unary    bool // tekli (önek) operatör mü: -x, !x
	dict     bool // sözlük sabitinin '{' ya da '}' işareti mi (içine boşluk konmaz: {"a": 1})
	body     bool // bir gövdenin '{' işareti mi (eğer, döngü, fonksiyon, ...; sözlük ve sondaki blok değil)
}

// MeaningChangedError biçimlenmiş kodun anlamı özgün koddan farklı çıktığında döner.
// Bu bir biçimlendirici hatasıdır; dosya yazılmamalıdır.
type MeaningChangedError struct {
	Detail string
}

func (e *MeaningChangedError) Error() string {
	return messages.Text(messages.H8400, e.Detail)
}

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *MeaningChangedError) MessageText(lang string) string {
	return messages.TextIn(lang, messages.H8400, e.Detail)
}

// SyntaxError kaynak ayrıştırılamadığında döner; hatalı kod biçimlenmez.
type SyntaxError struct {
	Errors []error
}

func (e *SyntaxError) Error() string {
	return messages.Text(messages.H8401, e.Errors[0])
}

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *SyntaxError) MessageText(lang string) string {
	return messages.TextIn(lang, messages.H8401, e.Errors[0])
}

// Format kaynağı standart biçime getirir. Kaynak ayrıştırılamıyorsa *SyntaxError,
// sonuç özgün kodla aynı anlama gelmiyorsa *MeaningChangedError döner.
func Format(text string) (string, error) {
	// Dosya başındaki UTF-8 BOM korunur (source.Parts); başlık ve '#!' birer yorumdur, olduğu
	// gibi kalır. Kodlu dosyanın gövdesi kodlu biçimde biçimlenir (kimlikler olduğu gibi kalır).
	b, err := source.Split(text)
	if err != nil {
		return "", &SyntaxError{Errors: []error{err}}
	}
	// Satır sonları LF yazılır; metinlerin içindekiler de (sözcük çözümleyici çok satırlı
	// metindeki CRLF'yi zaten '\n' okur, anlam değişmez): karışık satır sonu kalmaz (2.6).
	b.Body = strings.ReplaceAll(b.Body, "\r\n", "\n")
	p := parser.New(b.Lexer())
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return "", &SyntaxError{Errors: errs}
	}
	fresh := b
	fresh.Body = transform(b.Body, b.Coded, braceKinds(prog))
	if err := SameMeaning(b.Join(), fresh.Join()); err != nil {
		return "", err
	}
	return fresh.Join(), nil
}

// SameMeaning iki kaynağın (dosyanın tamamı) aynı sözcük birimi dizisine ve aynı AST'ye sahip
// olduğunu denetler; farklıysa *MeaningChangedError döner. Sözcükler türleriyle, kodla
// yazılmış adlar kodlarıyla (kodlu dosyada), kullanıcının adları metinleriyle karşılaştırılır.
func SameMeaning(before, after string) error {
	b1, err1 := source.Split(before)
	b2, err2 := source.Split(after)
	if err1 != nil || err2 != nil || b1.Coded != b2.Coded {
		return &MeaningChangedError{Detail: messages.Text(messages.H8402)}
	}
	p1 := parser.New(b1.Lexer())
	prog1 := p1.ParseProgram()
	p2 := parser.New(b2.Lexer())
	prog2 := p2.ParseProgram()
	if errs := p2.Errs(); len(errs) > 0 {
		return &MeaningChangedError{Detail: messages.Text(messages.H8403, errs[0])}
	}
	t1, t2 := tokens(b1.Body, b1.Coded), tokens(b2.Body, b2.Coded)
	if len(t1) != len(t2) {
		return &MeaningChangedError{Detail: messages.Text(messages.H8404)}
	}
	for i := range t1 {
		if t1[i].Type != t2[i].Type || t1[i].Code != t2[i].Code || t1[i].Literal != t2[i].Literal {
			return &MeaningChangedError{Detail: messages.Text(messages.H8405, t2[i].Line, t1[i].Literal)}
		}
	}
	if prog1.String() != prog2.String() {
		return &MeaningChangedError{Detail: messages.Text(messages.H8406)}
	}
	return nil
}

// newLexer bir gövdenin sözcük çözümleyicisidir (kodlu gövdede kodları tanır).
func newLexer(body string, coded bool) *lexer.Lexer {
	return source.Parts{Coded: coded, Body: body}.Lexer()
}

func tokens(body string, coded bool) []lexer.Token {
	l := newLexer(body, coded)
	var t []lexer.Token
	for {
		tok := l.NextToken()
		t = append(t, tok)
		if tok.Type == lexer.TokEOF {
			return t
		}
	}
}

type pos struct{ line, col int }

// Süslü parantez türleri (bkz. braceKinds)
const (
	braceDict  = 1 // sözlük sabiti: {"a": 1}
	braceBlock = 2 // sondaki blok (o-lambda): liste.filtrele { o > 2 }
)

// braceKinds ifade içindeki '{' işaretlerinin konumlarını türleriyle toplar: sözlük sabitleri
// ve sondaki bloklar. Bunlar ifadenin parçasıdır; yeni satırdaki böyle bir '{' önceki satıra
// alınırsa anlam değişebilir. Listede olmayan her '{' bir gövde başlangıcıdır (eğer, döngü,
// fonksiyon, yapı, dene...) ve ayrıştırıcı onu satırdan bağımsız okur.
func braceKinds(prog *ast.Program) map[pos]int {
	result := map[pos]int{}
	ast.Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *ast.DictLiteral:
			result[pos{n.Token.Line, n.Token.Column}] = braceDict
		case *ast.FunctionLiteral:
			if n.Token.Type == lexer.TokLBrace {
				result[pos{n.Token.Line, n.Token.Column}] = braceBlock
			}
		}
		return true
	})
	return result
}

// items kaynağı sözcük birimleri ve yorumlardan oluşan bir akışa çevirir.
func items(src string, coded bool) []item {
	l := newLexer(src, coded)
	input := l.Input()
	var result []item
	prev := 0 // önceki öğenin bittiği indeks
	for {
		tok := l.NextToken()
		head, last := l.LastRange()
		if head > len(input) {
			head = len(input)
		}
		result = append(result, commentsInRange(input[prev:head])...)
		if tok.Type == lexer.TokEOF {
			// Dosya sonundaki boşluk satır sayısı için işaretçi öğe
			result = append(result, item{kind: lexer.TokEOF, lineA: lineCount(input[prev:head])})
			return result
		}
		o := item{
			kind:     tok.Type,
			text:     string(input[head:last]),
			lineA:    lineCount(input[prev:head]),
			line:     tok.Line,
			col:      tok.Column,
			adjacent: head == prev,
		}
		result = append(result, o)
		prev = last
	}
}

// commentsInRange iki sözcük birimi arasındaki boşluk + yorum bölgesini ayrıştırır.
// Her yorum için, öncesindeki satır sonu sayısıyla birlikte bir öğe üretir. Bölgenin
// yorumlardan sonra kalan kısmının satır sayısını çağıran (lineCount ile) hesaplar.
func commentsInRange(r []rune) []item {
	var result []item
	lineA := 0
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\n':
			lineA++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			// Yorum '#' ile başlar, satır sonuna kadar sürer (2.2); çok satırlı yorum yoktur.
			j := i
			for j < len(r) && r[j] != '\n' {
				j++
			}
			result = append(result, item{kind: tokComment, text: strings.TrimRight(string(r[i:j]), " \t\r"), lineA: lineA})
			lineA = 0
			i = j
		default:
			i++
		}
	}
	return result
}

// lineCount boşluk + yorum bölgesinde, son yorumdan sonra gelen satır sonlarını sayar.
func lineCount(r []rune) int {
	n := 0
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\n':
			n++
			i++
		case c == '#':
			for i < len(r) && r[i] != '\n' {
				i++
			}
			n = 0
		default:
			i++
		}
	}
	return n
}

// Operatör sınıfları
var binaryOps = map[lexer.TokenType]bool{
	lexer.TokPlus: true, lexer.TokMinus: true, lexer.TokAsterisk: true, lexer.TokSlash: true,
	lexer.TokSlashSlash: true, lexer.TokPercent: true, lexer.TokPower: true, lexer.TokMatmul: true,
	lexer.TokEq: true, lexer.TokNotEq: true, lexer.TokLt: true, lexer.TokGt: true,
	lexer.TokLte: true, lexer.TokGte: true, lexer.TokAnd: true, lexer.TokOr: true,
	lexer.TokPipe: true, lexer.TokNullCoalesc: true, lexer.TokArrow: true,
	lexer.TokBitAnd: true, lexer.TokBitOr: true, lexer.TokBitXor: true, lexer.TokShl: true, lexer.TokShr: true,
	lexer.TokAssign: true, lexer.TokWalrus: true, lexer.TokPlusAssign: true,
	lexer.TokMinusAssign: true, lexer.TokMulAssign: true, lexer.TokDivAssign: true,
	lexer.TokFloorDivAssign: true, lexer.TokModAssign: true, lexer.TokPowAssign: true, lexer.TokBitAndAssign: true,
	lexer.TokBitOrAssign: true, lexer.TokBitXorAssign: true, lexer.TokShlAssign: true, lexer.TokShrAssign: true,
}

// Satır sonunda kalırsa sonraki satırı devam satırı yapan sözcükler
var continuingEnd = map[lexer.TokenType]bool{
	lexer.TokKwAnd: true, lexer.TokKwOr: true, lexer.TokDot: true, lexer.TokSafeDot: true,
	lexer.TokColon: true, lexer.TokRange: true, lexer.TokRangeIncl: true,
}

// Satır başında gelirse o satırı devam satırı yapan sözcükler ('-' bilerek yok: satır başında
// tekli eksi gibi görünür)
var continuingStart = map[lexer.TokenType]bool{
	lexer.TokPipe: true, lexer.TokDot: true, lexer.TokSafeDot: true, lexer.TokNullCoalesc: true,
	lexer.TokAnd: true, lexer.TokOr: true, lexer.TokKwAnd: true, lexer.TokKwOr: true,
	lexer.TokPlus: true, lexer.TokAsterisk: true, lexer.TokSlash: true, lexer.TokPercent: true,
	lexer.TokPower: true, lexer.TokMatmul: true, lexer.TokEq: true, lexer.TokNotEq: true,
	lexer.TokLt: true, lexer.TokGt: true, lexer.TokLte: true, lexer.TokGte: true,
	lexer.TokRange: true, lexer.TokRangeIncl: true, lexer.TokBitAnd: true, lexer.TokBitOr: true,
	lexer.TokBitXor: true, lexer.TokShl: true, lexer.TokShr: true,
}

// isUnary '-' işaretinin önceki sözcüğe bakarak önek (tekli) olup olmadığını belirler.
func isUnary(prev *item) bool {
	if prev == nil {
		return true
	}
	t := prev.kind
	switch {
	case t == lexer.TokRParen || t == lexer.TokRBracket || t == lexer.TokRBrace:
		return false
	case t == lexer.TokIdent || t == lexer.TokInt || t == lexer.TokFloat || t == lexer.TokString:
		return false
	case t == lexer.TokTrue || t == lexer.TokFalse || t == lexer.TokNil || t == lexer.TokSelf:
		return false
	}
	return true
}

// spaceBetween aynı satırdaki iki komşu öğe arasında bir boşluk olup olmayacağını söyler.
func spaceBetween(p, c *item) bool {
	pt, ct := p.kind, c.kind
	switch {
	case pt == tokComment || ct == tokComment:
		return true
	case ct == lexer.TokComma || ct == lexer.TokSemicolon || ct == lexer.TokRParen || ct == lexer.TokRBracket:
		return false
	case pt == lexer.TokLParen || pt == lexer.TokLBracket:
		return false
	case pt == lexer.TokDot || pt == lexer.TokSafeDot || ct == lexer.TokDot || ct == lexer.TokSafeDot:
		return false
	case pt == lexer.TokLBrace:
		return ct != lexer.TokRBrace && !p.dict
	case ct == lexer.TokRBrace:
		return !c.dict
	case p.unary:
		return false
	// Dönüş tipi oku: fonksiyon f() -> Tam
	case pt == lexer.TokMinus && ct == lexer.TokGt && c.adjacent:
		return false
	case ct == lexer.TokLParen || ct == lexer.TokLBracket:
		switch pt {
		case lexer.TokIdent, lexer.TokRParen, lexer.TokRBracket, lexer.TokRBrace,
			lexer.TokString, lexer.TokSelf, lexer.TokFunction, lexer.TokBreakpoint:
			return false
		}
		return true
	case pt == lexer.TokRange || pt == lexer.TokRangeIncl || ct == lexer.TokRange || ct == lexer.TokRangeIncl:
		return false
	case ct == lexer.TokColon:
		return false
	}
	return true
}

// transform Format'ın kullandığı dönüşümdür; testler anlam denetimini sınamak için
// bilerek bozuk bir dönüşümle değiştirir.
var transform = format

// mark tekli operatörleri ve süslü parantezlerin türünü işaretler; gövde '{' ve
// 'değilse'/'yakala' sözcüklerini önceki satıra alır.
func mark(og []item, braces map[pos]int) {
	var prevTok *item
	var braceStack []*item // açık '{' öğeleri (sözlük kapanışını eşlemek için)
	for i := range og {
		o := &og[i]
		if o.kind == tokComment || o.kind == lexer.TokEOF {
			continue
		}
		if o.kind == lexer.TokBang || o.kind == lexer.TokTilde {
			o.unary = true
		} else if o.kind == lexer.TokMinus {
			read := i+1 < len(og) && og[i+1].kind == lexer.TokGt && og[i+1].adjacent &&
				prevTok != nil && (prevTok.kind == lexer.TokRParen)
			o.unary = !read && isUnary(prevTok)
		}
		switch o.kind {
		case lexer.TokLBrace:
			kind := braces[pos{o.line, o.col}]
			o.dict, o.body = kind == braceDict, kind == 0
			braceStack = append(braceStack, o)
		case lexer.TokRBrace:
			if len(braceStack) > 0 {
				o.dict = braceStack[len(braceStack)-1].dict
				braceStack = braceStack[:len(braceStack)-1]
			}
		}
		if o.lineA > 0 && i > 0 && og[i-1].kind != tokComment {
			prev := og[i-1].kind
			switch {
			case o.body:
				o.lineA = 0
			case (o.kind == lexer.TokElse || o.kind == lexer.TokCatch) && prev == lexer.TokRBrace:
				o.lineA = 0
			}
		}
		prevTok = o
	}
}

// format asıl biçimlendirme işini yapar (anlam denetimi Format'tadır).
func format(src string, coded bool, braces map[pos]int) string {
	og := items(src, coded)

	mark(og, braces)

	// 2. Satırlara böl.
	type line struct {
		items       []*item
		blankBefore bool // öncesinde boş satır var mı
	}
	var lines []line
	for i := range og {
		o := &og[i]
		if o.kind == lexer.TokEOF {
			break
		}
		if len(lines) == 0 || o.lineA > 0 {
			lines = append(lines, line{blankBefore: o.lineA >= 2 && len(lines) > 0})
		}
		s := &lines[len(lines)-1]
		s.items = append(s.items, o)
	}

	// 3. Girintileri hesapla ve yaz. Devam satırının fazladan girintisi o satırda açılan
	// gövdeye geçmez: 'eğer a ve\n    b {' satırının gövdesi ve '}' işareti deyimin ('eğer')
	// girintisine göredir. stmtIndent ve stmtDepth devam satırı olmayan son satırın girintisi
	// ve o satırdaki açık parantez derinliğidir.
	type open struct{ indent int }
	var stack []open
	var sb strings.Builder
	var prevLineEndTok *item // önceki satırın son sözcük birimi (yorumlar hariç)
	stmtIndent, stmtDepth := 0, 0
	for si, s := range lines {
		first := s.items[0]
		indent, continued := 0, false
		// Satır başındaki kapanışlar
		startsWithClose := 0
		for _, o := range s.items {
			if !o.kind.IsCloser() {
				break
			}
			startsWithClose++
		}
		if startsWithClose > 0 {
			k := len(stack) - startsWithClose
			if k < 0 {
				k = 0
			}
			if k < len(stack) {
				indent = stack[k].indent
			}
		} else {
			if len(stack) > 0 {
				indent = stack[len(stack)-1].indent + 1
			}
			if prevLineEndTok != nil && first.kind != tokComment &&
				(binaryOps[prevLineEndTok.kind] && !prevLineEndTok.unary || continuingEnd[prevLineEndTok.kind] || continuingStart[first.kind]) {
				indent++
				continued = true
			}
		}
		if !continued {
			stmtIndent, stmtDepth = indent, max(len(stack)-startsWithClose, 0)
		}

		// Boş satır kuralları: dosya başında, '{' / '(' / '[' ile biten satırdan sonra ve
		// kapanışla başlayan satırdan önce boş satır olmaz.
		if s.blankBefore && si > 0 && startsWithClose == 0 && !(prevLineEndTok != nil && prevLineEndTok.kind.IsOpener() && lastItem(lines[si-1].items) == prevLineEndTok) {
			sb.WriteString("\n")
		}

		var lineSb strings.Builder
		lineSb.WriteString(strings.Repeat(indentUnit, indent))
		for j, o := range s.items {
			if j > 0 && spaceBetween(s.items[j-1], o) {
				lineSb.WriteString(" ")
			}
			lineSb.WriteString(o.text)
			if o.kind.IsOpener() {
				push := indent
				if o.body && len(stack) == stmtDepth {
					push = stmtIndent
				}
				stack = append(stack, open{indent: push})
			} else if o.kind.IsCloser() && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if o.kind != tokComment {
				prevLineEndTok = o
			}
		}
		sb.WriteString(strings.TrimRight(lineSb.String(), " \t"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func lastItem(o []*item) *item {
	return o[len(o)-1]
}
