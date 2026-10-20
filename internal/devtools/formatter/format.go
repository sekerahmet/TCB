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
	"reflect"
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

// SyntaxError kaynak ayrıştırılamadığında döner; hatalı kod biçimlenmez.
type SyntaxError struct {
	Errors []string
}

func (e *SyntaxError) Error() string {
	return messages.Text(messages.H8401, e.Errors[0])
}

// Format kaynağı standart biçime getirir. Kaynak ayrıştırılamıyorsa *SyntaxError,
// sonuç özgün kodla aynı anlama gelmiyorsa *MeaningChangedError döner.
func Format(text string) (string, error) {
	// Dosya başındaki UTF-8 BOM ve kodlu biçim başlığı korunur (source.Parts); kodlu
	// dosyanın gövdesi kodlu biçimde biçimlenir (kodlar olduğu gibi kalır).
	b, err := source.Split(text)
	if err != nil {
		return "", &SyntaxError{Errors: []string{err.Error()}}
	}
	// Satır sonları LF yazılır; metinlerin içindekiler de (sözcük çözümleyici çok satırlı
	// metindeki CRLF'yi zaten '\n' okur, anlam değişmez): karışık satır sonu kalmaz.
	b.Body = strings.ReplaceAll(b.Body, "\r\n", "\n")
	if b.Coded {
		b.HeaderEOL = "\n"
	}
	p := parser.New(b.Lexer())
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", &SyntaxError{Errors: p.Errors()}
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
	if len(p2.Errors()) > 0 {
		return &MeaningChangedError{Detail: messages.Text(messages.H8403, p2.Errors()[0])}
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
		if tok.Type == lexer.TOKEN_EOF {
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
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			if _, ok := v.Interface().(lexer.Token); ok {
				return
			}
			switch n := v.Addr().Interface().(type) {
			case *ast.DictLiteral:
				result[pos{n.Token.Line, n.Token.Column}] = braceDict
			case *ast.FunctionLiteral:
				if n.Token.Type == lexer.TOKEN_LBRACE {
					result[pos{n.Token.Line, n.Token.Column}] = braceBlock
				}
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				walk(v.MapIndex(k))
			}
		}
	}
	walk(reflect.ValueOf(prog))
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
		if tok.Type == lexer.TOKEN_EOF {
			// Dosya sonundaki boşluk satır sayısı için işaretçi öğe
			result = append(result, item{kind: lexer.TOKEN_EOF, lineA: lineCount(input[prev:head])})
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
		case c == '/' && i+1 < len(r) && r[i+1] == '/':
			j := i
			for j < len(r) && r[j] != '\n' {
				j++
			}
			result = append(result, item{kind: tokComment, text: strings.TrimRight(string(r[i:j]), " \t\r"), lineA: lineA})
			lineA = 0
			i = j
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			j := i + 2
			for j+1 < len(r) && !(r[j] == '*' && r[j+1] == '/') {
				j++
			}
			j += 2
			if j > len(r) {
				j = len(r)
			}
			lines := strings.Split(string(r[i:j]), "\n")
			for k := range lines {
				lines[k] = strings.TrimRight(lines[k], " \t\r")
			}
			result = append(result, item{kind: tokComment, text: strings.Join(lines, "\n"), lineA: lineA})
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
		case c == '/' && i+1 < len(r) && r[i+1] == '/':
			for i < len(r) && r[i] != '\n' {
				i++
			}
			n = 0
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			i += 2
			for i+1 < len(r) && !(r[i] == '*' && r[i+1] == '/') {
				i++
			}
			i += 2
			n = 0
		default:
			i++
		}
	}
	return n
}

// Operatör sınıfları
var binaryOps = map[lexer.TokenType]bool{
	lexer.TOKEN_PLUS: true, lexer.TOKEN_MINUS: true, lexer.TOKEN_ASTERISK: true, lexer.TOKEN_SLASH: true,
	lexer.TOKEN_PERCENT: true, lexer.TOKEN_POWER: true, lexer.TOKEN_MATMUL: true,
	lexer.TOKEN_EQ: true, lexer.TOKEN_NOT_EQ: true, lexer.TOKEN_LT: true, lexer.TOKEN_GT: true,
	lexer.TOKEN_LTE: true, lexer.TOKEN_GTE: true, lexer.TOKEN_AND: true, lexer.TOKEN_OR: true,
	lexer.TOKEN_PIPE: true, lexer.TOKEN_NULL_COALESC: true, lexer.TOKEN_ARROW: true,
	lexer.TOKEN_ASSIGN: true, lexer.TOKEN_WALRUS: true, lexer.TOKEN_PLUS_ASSIGN: true,
	lexer.TOKEN_MINUS_ASSIGN: true, lexer.TOKEN_MUL_ASSIGN: true, lexer.TOKEN_DIV_ASSIGN: true,
}

// Satır sonunda kalırsa sonraki satırı devam satırı yapan sözcükler
var continuingEnd = map[lexer.TokenType]bool{
	lexer.TOKEN_KW_AND: true, lexer.TOKEN_KW_OR: true, lexer.TOKEN_DOT: true, lexer.TOKEN_SAFE_DOT: true,
	lexer.TOKEN_COLON: true, lexer.TOKEN_RANGE: true, lexer.TOKEN_RANGE_INCL: true,
}

// Satır başında gelirse o satırı devam satırı yapan sözcükler ('-' bilerek yok: satır başında
// tekli eksi gibi görünür)
var continuingStart = map[lexer.TokenType]bool{
	lexer.TOKEN_PIPE: true, lexer.TOKEN_DOT: true, lexer.TOKEN_SAFE_DOT: true, lexer.TOKEN_NULL_COALESC: true,
	lexer.TOKEN_AND: true, lexer.TOKEN_OR: true, lexer.TOKEN_KW_AND: true, lexer.TOKEN_KW_OR: true,
	lexer.TOKEN_PLUS: true, lexer.TOKEN_ASTERISK: true, lexer.TOKEN_SLASH: true, lexer.TOKEN_PERCENT: true,
	lexer.TOKEN_POWER: true, lexer.TOKEN_MATMUL: true, lexer.TOKEN_EQ: true, lexer.TOKEN_NOT_EQ: true,
	lexer.TOKEN_LT: true, lexer.TOKEN_GT: true, lexer.TOKEN_LTE: true, lexer.TOKEN_GTE: true,
	lexer.TOKEN_RANGE: true, lexer.TOKEN_RANGE_INCL: true,
}

func opening(t lexer.TokenType) bool {
	return t == lexer.TOKEN_LBRACE || t == lexer.TOKEN_LPAREN || t == lexer.TOKEN_LBRACKET
}

func closing(t lexer.TokenType) bool {
	return t == lexer.TOKEN_RBRACE || t == lexer.TOKEN_RPAREN || t == lexer.TOKEN_RBRACKET
}

// isUnary '-' işaretinin önceki sözcüğe bakarak önek (tekli) olup olmadığını belirler.
func isUnary(prev *item) bool {
	if prev == nil {
		return true
	}
	t := prev.kind
	switch {
	case t == lexer.TOKEN_RPAREN || t == lexer.TOKEN_RBRACKET || t == lexer.TOKEN_RBRACE:
		return false
	case t == lexer.TOKEN_IDENT || t == lexer.TOKEN_INT || t == lexer.TOKEN_FLOAT || t == lexer.TOKEN_STRING:
		return false
	case t == lexer.TOKEN_TRUE || t == lexer.TOKEN_FALSE || t == lexer.TOKEN_NIL || t == lexer.TOKEN_SELF:
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
	case ct == lexer.TOKEN_COMMA || ct == lexer.TOKEN_SEMICOLON || ct == lexer.TOKEN_RPAREN || ct == lexer.TOKEN_RBRACKET:
		return false
	case pt == lexer.TOKEN_LPAREN || pt == lexer.TOKEN_LBRACKET:
		return false
	case pt == lexer.TOKEN_DOT || pt == lexer.TOKEN_SAFE_DOT || ct == lexer.TOKEN_DOT || ct == lexer.TOKEN_SAFE_DOT:
		return false
	case pt == lexer.TOKEN_LBRACE:
		return ct != lexer.TOKEN_RBRACE && !p.dict
	case ct == lexer.TOKEN_RBRACE:
		return !c.dict
	case p.unary:
		return false
	// Dönüş tipi oku: fonksiyon f() -> Tam
	case pt == lexer.TOKEN_MINUS && ct == lexer.TOKEN_GT && c.adjacent:
		return false
	case ct == lexer.TOKEN_LPAREN || ct == lexer.TOKEN_LBRACKET:
		switch pt {
		case lexer.TOKEN_IDENT, lexer.TOKEN_RPAREN, lexer.TOKEN_RBRACKET, lexer.TOKEN_RBRACE,
			lexer.TOKEN_STRING, lexer.TOKEN_SELF, lexer.TOKEN_FUNCTION, lexer.TOKEN_BREAKPOINT:
			return false
		}
		return true
	case pt == lexer.TOKEN_RANGE || pt == lexer.TOKEN_RANGE_INCL || ct == lexer.TOKEN_RANGE || ct == lexer.TOKEN_RANGE_INCL:
		return false
	case ct == lexer.TOKEN_COLON:
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
		if o.kind == tokComment || o.kind == lexer.TOKEN_EOF {
			continue
		}
		if o.kind == lexer.TOKEN_BANG {
			o.unary = true
		} else if o.kind == lexer.TOKEN_MINUS {
			read := i+1 < len(og) && og[i+1].kind == lexer.TOKEN_GT && og[i+1].adjacent &&
				prevTok != nil && (prevTok.kind == lexer.TOKEN_RPAREN)
			o.unary = !read && isUnary(prevTok)
		}
		switch o.kind {
		case lexer.TOKEN_LBRACE:
			kind := braces[pos{o.line, o.col}]
			o.dict, o.body = kind == braceDict, kind == 0
			braceStack = append(braceStack, o)
		case lexer.TOKEN_RBRACE:
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
			case (o.kind == lexer.TOKEN_ELSE || o.kind == lexer.TOKEN_CATCH) && prev == lexer.TOKEN_RBRACE:
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
		if o.kind == lexer.TOKEN_EOF {
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
			if !closing(o.kind) {
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
		if s.blankBefore && si > 0 && startsWithClose == 0 && !(prevLineEndTok != nil && opening(prevLineEndTok.kind) && lastItem(lines[si-1].items) == prevLineEndTok) {
			sb.WriteString("\n")
		}

		var lineSb strings.Builder
		lineSb.WriteString(strings.Repeat(indentUnit, indent))
		for j, o := range s.items {
			if j > 0 && spaceBetween(s.items[j-1], o) {
				lineSb.WriteString(" ")
			}
			lineSb.WriteString(o.text)
			if opening(o.kind) {
				push := indent
				if o.body && len(stack) == stmtDepth {
					push = stmtIndent
				}
				stack = append(stack, open{indent: push})
			} else if closing(o.kind) && len(stack) > 0 {
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
