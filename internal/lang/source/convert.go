package source

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Çevirici (17.3, 17.4): Türkçe yazılmış kaynağı kodlu biçime (Kodla) ve kodlu kaynağı bir
// yazım diline (Coz) çevirir. Çeviri yeniden yazdırma değildir: kaynakta yalnızca dilin
// sözcüklerinin aralıkları değişir; boşluklar, satır düzeni, yorumlar, kullanıcının adları,
// metinler ve sayılar bayt bayt korunur. Metin içi yerleştirmelerdeki sözcükler çevrilir; ham
// metnin ve yorumların içi hiçbir zaman çevrilmez. Neyin dilin sözcüğü olduğu sözcüğe bakarak
// tahmin edilmez: ayrıştırıcının ad görevlerinden (ayristir.AdRolu) ve denetleyicinin kapsam
// bilgisinden (denetci.AdBaglari) çıkarılır.

// SyntaxError is returned when the source to convert cannot be parsed: the language's words
// cannot be told apart without scope information, so the source is left untouched.
type SyntaxError struct {
	Errors []error
}

// Unwrap returns the coded message around the first error, so its position stays readable
// (messages.Pos).
func (e *SyntaxError) Unwrap() error { return messages.New(messages.H2514, e.Errors[0]) }

func (e *SyntaxError) Error() string { return e.Unwrap().Error() }

// ErrUTF8 is returned when the body to convert is not valid UTF-8.
var ErrUTF8 error = messages.New(messages.H2510)

// ErrLossy is returned when the converted result would not mean the same as the original
// program. It is a converter bug; the file must not be written.
var ErrLossy error = messages.New(messages.H2511)

// Encode converts source written in Turkish (the whole file) to coded form. Source that is
// already coded is returned as is. The '$' of '$name' escapes is dropped (2.11). The result is
// verified: the coded program's tree must equal the original's, and decoding and re-encoding
// the coded text must not change it; otherwise it returns ErrLossy.
func Encode(text string) (string, error) {
	b, err := Split(text)
	if err != nil || b.Coded {
		return text, err
	}
	c, err := analyze(b, codes.Active())
	if err != nil {
		return "", err
	}
	coded := Parts{BOM: b.BOM, Coded: true, Body: c.codedBody()}
	coded.HeaderEOL = findEOL(coded.Body)
	result := coded.Join()
	if err := c.sameMeaning(coded); err != nil {
		return "", err
	}
	if back, err := Decode(result, codes.Active()); err != nil || !reencodes(back, result) {
		return "", ErrLossy
	}
	return result, nil
}

// reencodes kodlu metnin çözülmüş hâli kodlanınca aynı kodlu metni veriyor mu?
func reencodes(decoded, coded string) bool {
	b, err := Split(decoded)
	if err != nil || b.Coded {
		return false
	}
	c, err := analyze(b, codes.Active())
	if err != nil {
		return false
	}
	fresh := Parts{BOM: b.BOM, Coded: true, Body: c.codedBody()}
	fresh.HeaderEOL = findEOL(fresh.Body)
	return fresh.Join() == coded
}

// codedBody Türkçe gövdenin kodlu biçimidir: anahtar kelimeler ve dilin öğesi olan adlar
// kodlarıyla yazılır, '$ad' kaçışlarının '$'ı atılır; başka hiçbir şey değişmez.
func (c *analysis) codedBody() string {
	var d []change
	for _, r := range c.records {
		if r.Type == lexer.TOKEN_IDENT && c.escaped(r) {
			d = append(d, change{r.Start, r.End, r.Literal})
		} else if k := c.tokenCode(r); k != codes.None {
			d = append(d, change{r.Start, r.End, k.String()})
		}
	}
	return apply(c.input, d)
}

// Decode converts coded source (the whole file) to dil's spelling; source that is not coded
// is returned as is. Codes become dil's names; user names that would read as a word of the
// language in dil are written as '$name' (2.11). The header is dropped and the BOM kept.
func Decode(text string, lang *codes.Language) (string, error) {
	b, err := Split(text)
	if err != nil || !b.Coded {
		return text, err
	}
	c, err := analyze(b, lang)
	if err != nil {
		return "", err
	}
	view := c.viewBody()
	// The view must read back as the same program. The lexer reads only the active language,
	// so the check runs when lang is the active one ('tr göster --dil' re-executes itself with
	// the view language for this reason).
	if lang == codes.Active() && !reencodesBody(view, b.Body) {
		return "", ErrLossy
	}
	return Parts{BOM: b.BOM, Body: view}.Join(), nil
}

// reencodesBody reports whether the view body, encoded again, gives the coded body back.
func reencodesBody(view, codedBody string) bool {
	c, err := analyze(Parts{Body: view}, codes.Active())
	return err == nil && c.codedBody() == codedBody
}

// viewBody kodlu gövdenin c.dil'deki yazımıdır.
func (c *analysis) viewBody() string {
	var d []change
	for _, r := range c.records {
		switch {
		case r.Code != codes.None:
			d = append(d, change{r.Start, r.End, langName(c.lang, r.Code)})
		case r.Type == lexer.TOKEN_IDENT && !c.escaped(r) && c.needsEscape(r):
			d = append(d, change{r.Start, r.End, "$" + r.Literal})
		}
	}
	return apply(c.input, d)
}

// view kodlu bir gövdenin etkin dildeki yazımıdır: hata çerçeveleri ve hata ayıklayıcı
// bu metni gösterir (konumlar ona göredir, 17.5). Gövde ayrıştırılamıyorsa yalnızca dosya
// düzeyindeki kodlar adlarına çevrilir (yerleştirmelerdekiler kod kalır).
func view(b Parts) string {
	if c, err := analyze(b, codes.Active()); err == nil {
		return c.viewBody()
	}
	l := b.Lexer()
	l.StartRecording()
	for l.NextToken().Type != lexer.TOKEN_EOF {
	}
	var d []change
	for _, r := range l.Records() {
		if r.Code != codes.None {
			d = append(d, change{r.Start, r.End, langName(codes.Active(), r.Code)})
		}
	}
	return apply(l.Input(), d)
}

// langName kodun dil'deki adıdır; dil kodu tanımıyorsa (başka dillerde ölü kodlar isteğe
// bağlıdır, 17.6) kod olduğu gibi kalır.
func langName(lang *codes.Language, k codes.Code) string {
	if g, ok := lang.Entry(k); ok {
		return g.Name
	}
	return k.String()
}

// analysis bir gövdenin çeviri için çıkarılmış bilgisidir: bütün sözcükler (yerleştirmelerdekiler
// dahil) kaynak aralıklarıyla, ad sözcüklerinin görevleri ve adların bağları (dil'in adlarıyla).
type analysis struct {
	lang     *codes.Language
	input    []rune
	records  []lexer.Record
	roles    map[[2]int]parser.NameRole
	bindings map[[2]int]checker.NameBinding
	// conflicts gösterimde kodlu bir sözcüğü yakalayacak kullanıcı adlarıdır; her yerde
	// kaçışla yazılırlar (denetci.AdBaglari).
	conflicts map[string]bool
	prog      *ast.Program
}

func analyze(b Parts, lang *codes.Language) (*analysis, error) {
	if !utf8.ValidString(b.Body) {
		return nil, ErrUTF8
	}
	l := b.Lexer()
	l.StartRecording()
	p := parser.New(l)
	p.RecordRoles()
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return nil, &SyntaxError{Errors: errs}
	}
	c := &analysis{lang: lang, input: l.Input(), records: l.Records(), roles: p.Roles(),
		bindings: map[[2]int]checker.NameBinding{}, prog: prog}
	bindings, conflicts := checker.NameBindings(prog, lang, nil, false)
	for _, bg := range bindings {
		c.bindings[[2]int{bg.Token.Line, bg.Token.Column}] = bg
	}
	c.conflicts = conflicts
	return c, nil
}

// sameMeaning kodlu gövdenin ağacının özgün ağaçla aynı olduğunu denetler (sözcük birimleri
// konumlarıyla atlanır; kodlu sözcükler etkin dilin adlarıyla okunduğu için ağaçlar birebir
// aynı olmalıdır).
func (c *analysis) sameMeaning(coded Parts) error {
	p := parser.New(coded.Lexer())
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || ast.Dump(prog) != ast.Dump(c.prog) {
		return ErrLossy
	}
	return nil
}

// escaped sözcük kaynakta '$ad' kaçışıyla mı yazılmış?
func (c *analysis) escaped(r lexer.Record) bool {
	return r.Start < len(c.input) && c.input[r.Start] == '$'
}

// tokenCode bir sözcüğün kodlu biçimdeki kodudur: anahtar kelime, kodla yazılmış sözcük ya da
// dilin öğesi olan ad; kullanıcının adında kod.Yok.
func (c *analysis) tokenCode(r lexer.Record) codes.Code {
	if r.Code != codes.None {
		return r.Code
	}
	if r.Type == lexer.TOKEN_IDENT {
		return c.langElement(r)
	}
	return lexer.KeywordCode(r.Type)
}

// needsEscape kodlu kaynaktaki bir kullanıcı adı dil'de yazılınca dilin bir sözcüğü olarak
// okunur mu? Öyleyse gösterimde '$ad' yazılır.
func (c *analysis) needsEscape(r lexer.Record) bool {
	if k, ok := c.lang.Find(codes.NamespaceGlobal, r.Literal); ok && k.IsKeyword() {
		return true
	}
	return c.conflicts[r.Literal] || c.langElement(r) != codes.None
}

// change kaynakta [bas, son) karakter aralığının yerine yazılacak metindir.
type change struct {
	start, end int
	text       string
}

// apply değişimleri girdiye uygular; aynı aralık iki kez kaydedilmişse (bir yerleştirme iki
// kez ayrıştırıldıysa) bir kez uygulanır.
func apply(input []rune, d []change) string {
	sort.SliceStable(d, func(i, j int) bool { return d[i].start < d[j].start })
	var sb strings.Builder
	prev := 0
	for _, x := range d {
		if x.start < prev {
			continue
		}
		sb.WriteString(string(input[prev:x.start]))
		sb.WriteString(x.text)
		prev = x.end
	}
	sb.WriteString(string(input[prev:]))
	return sb.String()
}
