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

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *SyntaxError) MessageText(lang string) string { return messages.ErrorIn(e.Unwrap(), lang) }

// ErrUTF8 is returned when the body to convert is not valid UTF-8.
var ErrUTF8 error = messages.New(messages.H2510)

// ErrLossy is returned when the converted result would not mean the same as the original
// program. It is a converter bug; the file must not be written.
var ErrLossy error = messages.New(messages.H2511)

// Encode converts source written in Turkish (a file; the whole file) to coded form; see
// EncodeIn.
func Encode(text string) (string, error) { return EncodeIn(text, Turkish()) }

// EncodeIn converts source written in lang (the whole text) to coded form. Source that is
// already coded is returned as is. The '$' of '$name' escapes is dropped (2.11). The result is
// verified: the coded program read in lang must give the original's tree, and decoding to lang
// and re-encoding must not change it; otherwise it returns ErrLossy.
func EncodeIn(text string, lang *codes.Language) (string, error) {
	b, err := Split(text)
	if err != nil || b.Coded {
		return text, err
	}
	c, err := analyze(b, lang, nil, false)
	if err != nil {
		return "", err
	}
	coded := c.codedParts(b)
	result := coded.Join()
	if err := c.sameMeaning(coded); err != nil {
		return "", err
	}
	if back, err := Decode(result, lang); err != nil || !reencodes(back, result, lang) {
		return "", ErrLossy
	}
	return result, nil
}

// toCoded lang'de yazılmış kodsuz gövdeyi denetimsiz kodlu biçime çevirir (Encode'un
// doğrulaması olmadan; gösterim ve kabuk satırı için); gövde ayrıştırılamıyorsa b ve hatayı
// döner. globalNames metnin içinde okunduğu dış kapsamın adlarıdır (kabukta önceki satırlar,
// modülde aktaran, 11.1): onlardan biriyle aynı yazılan ad kullanıcının adıdır, dilin öğesi
// sayılıp kodlanmaz; dynamic, dış kapsamın adlarının tam bilinmediğini söyler.
func toCoded(b Parts, lang *codes.Language, globalNames []string, dynamic bool) (Parts, error) {
	c, err := analyze(b, lang, globalNames, dynamic)
	if err != nil {
		return b, err
	}
	return c.codedParts(b), nil
}

// codedParts b'nin (kodsuz, çözümlenmiş) kodlu biçimidir: BOM korunur, başlık satır sonu bulunur.
func (c *analysis) codedParts(b Parts) Parts {
	coded := Parts{BOM: b.BOM, Coded: true, Language: c.lang.Code, Body: c.codedBody()}
	coded.HeaderEOL = findEOL(coded.Body)
	return coded
}

// Show is the text of a file (the whole file) in lang's spelling: a coded file is decoded
// (Decode); a file written in Turkish is shown in lang through its coded form (17.7), and
// returned as is when lang is Turkish.
func Show(text string, lang *codes.Language) (string, error) {
	b, err := Split(text)
	if err != nil || b.Coded || lang.Code == codes.SourceLanguage {
		return Decode(text, lang)
	}
	coded, err := toCoded(b, Turkish(), nil, false)
	if err != nil {
		return "", err
	}
	return Decode(coded.Join(), lang)
}

// reencodes kodlu metnin lang'e çözülmüş hâli kodlanınca aynı kodlu metni veriyor mu?
func reencodes(decoded, coded string, lang *codes.Language) bool {
	b, err := Split(decoded)
	if err != nil || b.Coded {
		return false
	}
	fresh, err := toCoded(b, lang, nil, false)
	return err == nil && fresh.Join() == coded
}

// codedBody Türkçe gövdenin kodlu biçimidir: anahtar kelimeler ve dilin öğesi olan adlar
// kodlarıyla yazılır, '$ad' kaçışlarının '$'ı atılır; başka hiçbir şey değişmez.
func (c *analysis) codedBody() string {
	var d []change
	for _, r := range c.records {
		if r.Type == lexer.TokIdent && c.escaped(r) {
			d = append(d, change{r.Start, r.End, r.Literal})
		} else if k := c.tokenCode(r); k != codes.None {
			d = append(d, change{r.Start, r.End, k.String()})
		}
	}
	return apply(c.input, uniqueChanges(d))
}

// Decode converts coded source (the whole file) to dil's spelling; source that is not coded
// is returned as is. Codes become dil's names; user names that would read as a word of the
// language in dil are written as '$name' (2.11). The header is dropped and the BOM kept.
func Decode(text string, lang *codes.Language) (string, error) {
	b, err := Split(text)
	if err != nil || !b.Coded {
		return text, err
	}
	c, err := analyze(b, lang, nil, false)
	if err != nil {
		return "", err
	}
	view := c.viewBody()
	// The view must read back as the same program: read in lang and encoded again, it gives
	// the coded body back.
	if !reencodesBody(view, b.Body, lang) {
		return "", ErrLossy
	}
	return Parts{BOM: b.BOM, Body: view}.Join(), nil
}

// reencodesBody reports whether the view body, read in lang and encoded again, gives the coded
// body back.
func reencodesBody(view, codedBody string, lang *codes.Language) bool {
	c, err := analyze(Parts{Body: view}, lang, nil, false)
	return err == nil && c.codedBody() == codedBody
}

// viewBody kodlu gövdenin c.dil'deki yazımıdır.
func (c *analysis) viewBody() string { return apply(c.input, uniqueChanges(c.viewChanges())) }

// viewChanges gövdeyi gösterime çeviren değişimlerdir: kodlar adlarına, dilin bir sözcüğü
// gibi okunacak kullanıcı adları '$ad' yazımına.
func (c *analysis) viewChanges() []change {
	var d []change
	for _, r := range c.records {
		switch {
		case r.Code != codes.None:
			d = append(d, change{r.Start, r.End, langName(c.lang, r.Code)})
		case r.Type == lexer.TokIdent && !c.escaped(r) && c.needsEscape(r):
			d = append(d, change{r.Start, r.End, "$" + r.Literal})
		}
	}
	return d
}

// view kodlu bir gövdenin kanonik (Türkçe) yazımıdır: hata çerçeveleri ve hata ayıklayıcı
// bu metni gösterir (17.5). Gövde ayrıştırılamıyorsa yalnızca dosya düzeyindeki kodlar
// adlarına çevrilir (yerleştirmelerdekiler kod kalır). İkinci değer gösterimin sütunlarını
// veren sözcük çözümleyicidir: konumlar tam bu metne göredir, '$ad' kaçışları dahil.
func view(b Parts) (string, *lexer.Lexer) {
	if c, err := analyze(b, codes.Source(), nil, false); err == nil {
		d := uniqueChanges(c.viewChanges())
		return apply(c.input, d), lexer.NewBodyView(b.Body, viewEdits(d))
	}
	l := b.scanLexer(codes.Source())
	l.StartRecording()
	for l.NextToken().Type != lexer.TokEOF {
	}
	var d []change
	for _, r := range l.Records() {
		if r.Code != codes.None {
			d = append(d, change{r.Start, r.End, langName(codes.Source(), r.Code)})
		}
	}
	d = uniqueChanges(d)
	return apply(l.Input(), d), lexer.NewBodyView(b.Body, viewEdits(d))
}

// viewEdits değişimleri sözcük çözümleyicinin sütun kaymalarına çevirir; apply gibi aynı
// aralığı bir kez sayar.
// d uniqueChanges'ten geçmiş olmalıdır.
func viewEdits(d []change) []lexer.ViewEdit {
	var out []lexer.ViewEdit
	for _, x := range d {
		out = append(out, lexer.ViewEdit{Start: x.start, End: x.end, Len: utf8.RuneCountInString(x.text)})
	}
	return out
}

// uniqueChanges değişimleri başlangıca göre sıralar (yerinde, kararlı) ve önceki bir değişimin
// aralığıyla çakışanları eler. apply ve viewEdits bunun çıktısını bekler: bir değişim kümesi
// iki biçime çevrilecekse bir kez sıralanır.
func uniqueChanges(d []change) []change {
	sort.SliceStable(d, func(i, j int) bool { return d[i].start < d[j].start })
	out := d[:0:0]
	prev := 0
	for _, x := range d {
		if x.start < prev {
			continue
		}
		out = append(out, x)
		prev = x.end
	}
	return out
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
	// escapedNames metinde kaçışla ($ad) yazılmış kullanıcı adlarıdır (bkz. langElement).
	escapedNames map[string]bool
	// declarations kitaplık bildirimlerinin ad sözcükleridir (libraryDeclarations).
	declarations map[[2]int]codes.Code
	prog         *ast.Program
}

// analyze b'yi lang'de okur; globalNames ve dynamic metnin dış kapsamıdır (toCoded).
func analyze(b Parts, lang *codes.Language, globalNames []string, dynamic bool) (*analysis, error) {
	if !utf8.ValidString(b.Body) {
		return nil, ErrUTF8
	}
	l := b.scanLexer(lang)
	l.StartRecording()
	p := parser.New(l)
	p.RecordRoles()
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return nil, &SyntaxError{Errors: errs}
	}
	c := &analysis{lang: lang, input: l.Input(), records: l.Records(), roles: p.Roles(),
		bindings: map[[2]int]checker.NameBinding{}, prog: prog}
	bindings, conflicts := checker.NameBindings(prog, l.Language(), lang, globalNames, dynamic)
	for _, bg := range bindings {
		c.bindings[[2]int{bg.Token.Line, bg.Token.Column}] = bg
	}
	c.conflicts = conflicts
	c.declarations = c.libraryDeclarations()
	c.escapedNames = map[string]bool{}
	for _, r := range c.records {
		if r.Type == lexer.TokIdent && c.escaped(r) {
			c.escapedNames[r.Literal] = true
		}
	}
	return c, nil
}

// sameMeaning kodlu gövdenin ağacının özgün ağaçla aynı olduğunu denetler (sözcük birimleri
// konumlarıyla atlanır; kodlu sözcükler c.lang'in adlarıyla okunduğu için ağaçlar birebir
// aynı olmalıdır).
func (c *analysis) sameMeaning(coded Parts) error {
	p := parser.New(lexer.NewBodyIn(coded.Body, true, c.lang))
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
	if r.Type == lexer.TokIdent {
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

// apply değişimleri girdiye uygular; d uniqueChanges'ten geçmiş olmalıdır (aynı aralık iki kez
// kaydedilmişse, örneğin bir yerleştirme iki kez ayrıştırıldıysa, orada bir kez sayılır).
func apply(input []rune, d []change) string {
	var sb strings.Builder
	prev := 0
	for _, x := range d {
		sb.WriteString(string(input[prev:x.start]))
		sb.WriteString(x.text)
		prev = x.end
	}
	sb.WriteString(string(input[prev:]))
	return sb.String()
}
