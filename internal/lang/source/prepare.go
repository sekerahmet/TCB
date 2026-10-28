package source

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// The stages where preparation can stop (Result.Stage). Module errors are reported with
// these names.
const (
	StageSyntax = "syntax"
	StageType   = "type"
)

// Options selects the stages a source goes through.
type Options struct {
	// Module parses the source as a module (a file brought in with an import: a top-level
	// return is an error).
	Module bool
	// TypeCheck runs the type check and the undefined-name check; warnings come with it.
	TypeCheck bool
	// Checker, when set, is used for the type check (the REPL uses the same checker across
	// lines; Result.Errors and Warnings are then cumulative lists).
	Checker *checker.Checker
	// InGlobalScope says the source runs under an importer's global scope (11.1):
	// GlobalNames are that scope's names, GlobalDynamic says they are not fully known.
	// A typed line is encoded knowing GlobalNames (the earlier lines' names), so a user's name
	// is not read as a language element.
	InGlobalScope bool
	GlobalNames   []string
	GlobalDynamic bool
	// Typed says the text is not a file but a line typed in the person's writing language
	// (codes.Display; the REPL): it is encoded first.
	Typed bool
	// Body says the text is a coded body without the header (a test's snippet, a program built
	// in memory): it is read as coded, line 1 is its first line. A file needs the header (2.1).
	Body bool
}

// Result is the outcome of preparation. On error, Stage is the error's stage and Program is
// the tree built up to that stage (it may be incomplete after a syntax error).
type Result struct {
	Parts
	// Text hata çerçevelerinin, hata ayıklayıcının ve yorumlayıcının kaynak metnidir: dosyanın
	// kendi metni, başlık satırı dahil (kimlik yazımıyla, TR_break); kabuk satırı kişinin dilindeki
	// (codes.Display, TR_DIL) görünümüdür, çevrilemeyen satır olduğu gibi kalır. Hata iletilerindeki
	// satır ve sütun bu metne göredir.
	Text    string
	Program *ast.Program
	Stage   string
	// Errors and Warnings carry their position as a field (messages.PosError): frames and
	// tools read it with messages.Pos, never from the text. A summary line (limitErrors) and
	// a header error have no position of their own.
	Errors   []error
	Warnings []error
}

// Prepare turns source text (the whole file, BOM included) into a program: header check,
// lexing, parsing and, when requested, type checking. A file without the header does not run
// (2.1): the error gives the encoding command.
func Prepare(text string, s Options) *Result {
	b, err := Split(text)
	r := &Result{Parts: b, Text: b.Body}
	if err == nil && !b.Coded && !s.Typed && !s.Body {
		err = NoHeaderError()
	}
	if err != nil {
		r.Program = &ast.Program{}
		r.Stage, r.Errors = StageSyntax, []error{err}
		return r
	}
	// Dosyanın konumu dosyanındır: satır, sütun ve çerçeve diskteki metinden (TR_break yazımıyla)
	// alınır.
	lex := lexer.NewBody(b.Body, true)
	if s.Typed {
		// Kabuğa yazılan satır kişinin yazım dilindedir: kodlu biçime, oradan kişinin görünümüne
		// çevrilir; konumlar o görünüme göredir. Ayrıştırılamayan satır o dilde okunur; sözdizimi
		// hataları satırın kendi metnine göredir.
		lang := codes.Display()
		if coded, err := toCoded(b.Body, lang, s.GlobalNames, s.GlobalDynamic); err == nil {
			r.Text, lex = view(coded, lang)
		} else {
			lex = lexer.NewBodyIn(b.Body, false, lang)
		}
	}
	p := parser.New(lex)
	if s.Module {
		r.Program = p.ParseModule()
	} else {
		r.Program = p.ParseProgram()
	}
	if errs := p.Errs(); len(errs) > 0 {
		r.Stage, r.Errors = StageSyntax, limitErrors(errs)
		return r
	}
	if !s.TypeCheck {
		return r
	}
	chk := s.Checker
	if chk == nil {
		chk = checker.New()
	}
	if s.InGlobalScope {
		chk.SetGlobalScope(s.GlobalNames, s.GlobalDynamic)
	}
	if s.Typed {
		// Kabukta kullanıcının adları kişinin dilindedir: bir yerleşiğin adını gölgeleyebilir.
		chk.SetFileLanguage(codes.Display().Code)
	}
	chk.CheckProgram(r.Program)
	r.Warnings = limitErrors(chk.Warnings())
	if errs := chk.Errs(); len(errs) > 0 {
		r.Stage, r.Errors = StageType, limitErrors(errs)
	}
	return r
}

// MaxErrors is the number of errors (and warnings) a Result reports at most. A file with many
// thousands of errors (often not a TR file at all) would otherwise take minutes to report:
// every error is shown with a code frame.
const MaxErrors = 100

// limitErrors keeps the first MaxErrors messages and says how many were left out. The summary
// line has no position, so no code frame is drawn for it.
func limitErrors(errs []error) []error {
	if len(errs) <= MaxErrors {
		return errs
	}
	out := append([]error(nil), errs[:MaxErrors]...)
	return append(out, messages.New(messages.H2513, len(errs)-MaxErrors, MaxErrors))
}
