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

// Görünüm (17.7): kodlu dosyayı bir yazım dilinde okunacak metne çevirir (Show). Görünüm yalnız
// okunur: çalışmaz ve koda geri çevrilmez (ViewError). Kaynakta yalnızca dilin sözcüklerinin
// aralıkları değişir; boşluklar, satır düzeni, başlık dışındaki yorumlar, kullanıcının adları,
// metinler ve sayılar korunur. Metin içi yerleştirmelerdeki sözcükler çevrilir; ham metnin ve
// yorumların içi çevrilmez. Hangi sözcüğün dilin sözcüğü olduğu kodlu metinde yazımından (TR_print)
// bellidir; görünümde dilin bir sözcüğü gibi okunacak kullanıcı adını işaretlemek ('$ad') için
// ayrıştırıcının ad görevleri (parser.NameRole) ve denetleyicinin kapsam bilgisi
// (checker.NameBindings) kullanılır.

// SyntaxError is returned when the source to show cannot be parsed: its words cannot be told
// apart, so no view is written.
type SyntaxError struct {
	Errors []error
}

// Unwrap returns the coded message around the first error, so its position stays readable
// (messages.Pos).
func (e *SyntaxError) Unwrap() error { return messages.New(messages.H2514, e.Errors[0]) }

func (e *SyntaxError) Error() string { return e.Unwrap().Error() }

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *SyntaxError) MessageText(lang string) string { return messages.ErrorIn(e.Unwrap(), lang) }

// ErrUTF8 is returned when the body to show is not valid UTF-8.
var ErrUTF8 error = messages.New(messages.H2510)

// Show converts coded source (the whole file) to lang's spelling; source without the header is
// returned as is. Codes become lang's names; user names that would read as a word of the language
// in lang are written as '$name' (a mark for the reader, 17.7). The BOM is kept and the header
// becomes the view's header ('# TR 4 view=tr'): the view has the file's lines and says it is a
// view. A view is not shown again (ViewError).
func Show(text string, lang *codes.Language) (string, error) {
	b, err := Split(text)
	if err != nil || !b.Coded {
		return text, err
	}
	c, err := analyze(b.Body, lang)
	if err != nil {
		return "", err
	}
	return Parts{BOM: b.BOM, Body: setHeader(c.viewBody(), viewHeader(lang))}.Join(), nil
}

// ShowBody is the view in lang of a coded body without the header (a code's example, an editor
// template): the body's view, without the view's header.
func ShowBody(body string, lang *codes.Language) (string, error) {
	c, err := analyze(body, lang)
	if err != nil {
		return "", err
	}
	return c.viewBody(), nil
}

// unnormalized kullanıcı adının metinde NFC biçiminde yazılmadığıdır (ayrık o + U+0308): görünüm
// adı sözcük çözümleyicinin okuduğu NFC biçimiyle yazar (2.3).
func (c *analysis) unnormalized(r lexer.Record) bool {
	return r.Type == lexer.TokIdent && r.Code == codes.None && r.End <= len(c.input) &&
		string(c.input[r.Start:r.End]) != r.Literal
}

// viewBody kodlu gövdenin c.lang'deki yazımıdır.
func (c *analysis) viewBody() string { return apply(c.input, uniqueChanges(c.viewChanges())) }

// viewChanges gövdeyi gösterime çeviren değişimlerdir: kodlar adlarına, dilin bir sözcüğü
// gibi okunacak kullanıcı adları '$ad' yazımına.
func (c *analysis) viewChanges() []change {
	var d []change
	for _, r := range c.records {
		switch {
		case r.Code != codes.None && c.spelled[r.Code]:
			// Kullanıcı fonksiyonunun TR_ parametresi yazımıyla kalır (gösterimde kimlik okunur).
		case r.Code != codes.None:
			d = append(d, change{r.Start, r.End, langName(c.lang, r.Code)})
		case r.Type == lexer.TokIdent && c.needsEscape(r):
			d = append(d, change{r.Start, r.End, "$" + r.Literal})
		case c.unnormalized(r):
			d = append(d, change{r.Start, r.End, r.Literal})
		}
	}
	return d
}

// uniqueChanges değişimleri başlangıca göre sıralar (yerinde, kararlı) ve önceki bir değişimin
// aralığıyla çakışanları eler. apply bunun çıktısını bekler.
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

// langName kodun lang'deki adıdır; dil kodu tanımıyorsa (başka dillerde ölü kodlar isteğe
// bağlıdır) kimliğin yazımı kalır.
func langName(lang *codes.Language, k codes.Code) string {
	if g, ok := lang.Entry(k); ok {
		return g.Name
	}
	return codes.Spelling(k)
}

// analysis kodlu bir gövdenin gösterim için çıkarılmış bilgisidir: bütün sözcükler
// (yerleştirmelerdekiler dahil) kaynak aralıklarıyla, ad sözcüklerinin görevleri ve adların bağları
// (lang'in adlarıyla).
type analysis struct {
	lang     *codes.Language
	input    []rune
	records  []lexer.Record
	roles    map[[2]int]parser.NameRole
	bindings map[[2]int]checker.NameBinding
	// conflicts gösterimde kodlu bir sözcüğü yakalayacak kullanıcı adlarıdır; her yerde
	// işaretle yazılırlar (checker.NameBindings).
	conflicts map[string]bool
	// declarations kitaplık bildirimlerinin ad sözcükleridir (libraryDeclarations).
	declarations map[[2]int]codes.Code
	// spelled kodlu gövdede kullanıcı fonksiyonlarının TR_ ile yazılmış parametreleridir
	// (userParameterCodes): gösterimde yazımlarıyla kalırlar.
	spelled map[codes.Code]bool
	prog    *ast.Program
}

// analyze kimliklerle yazılmış kodlu gövdeyi okur.
func analyze(body string, lang *codes.Language) (*analysis, error) {
	if !utf8.ValidString(body) {
		return nil, ErrUTF8
	}
	l := lexer.NewBody(body, true)
	l.StartRecording()
	p := parser.New(l)
	p.RecordRoles()
	prog := p.ParseProgram()
	if errs := p.Errs(); len(errs) > 0 {
		return nil, &SyntaxError{Errors: errs}
	}
	c := &analysis{lang: lang, input: l.Input(), records: l.Records(), roles: p.Roles(),
		bindings: map[[2]int]checker.NameBinding{}, prog: prog}
	bindings, conflicts := checker.NameBindings(prog, l.Language(), lang, nil, false)
	for _, bg := range bindings {
		c.bindings[[2]int{bg.Token.Line, bg.Token.Column}] = bg
	}
	c.conflicts = conflicts
	c.declarations = c.libraryDeclarations()
	c.spelled = userParameterCodes(prog)
	return c, nil
}

// needsEscape kodlu kaynaktaki bir kullanıcı adı lang'de yazılınca dilin bir sözcüğü gibi okunur
// mu? Öyleyse gösterimde '$ad' yazılır.
func (c *analysis) needsEscape(r lexer.Record) bool {
	if k, ok := c.lang.Find(codes.NamespaceGlobal, r.Literal); ok && k.IsKeyword() {
		return true
	}
	return c.conflicts[r.Literal] || c.langElement(r) != codes.None
}

// change kaynakta [start, end) karakter aralığının yerine yazılacak metindir.
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
