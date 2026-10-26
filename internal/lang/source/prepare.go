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
	// Module parses the source as a module (a file brought in with 'aktar': a top-level
	// 'sonuç' is an error).
	Module bool
	// TypeCheck runs the type check and the undefined-name check; warnings come with it.
	TypeCheck bool
	// Checker, when set, is used for the type check (the REPL uses the same checker across
	// lines; Result.Errors and Warnings are then cumulative lists).
	Checker *checker.Checker
	// InGlobalScope says the source runs under an importer's global scope (11.1):
	// GlobalNames are that scope's names, GlobalDynamic says they are not fully known.
	// Text without the coded header is encoded knowing GlobalNames (also a REPL line: the
	// earlier lines' names), so a user's name is not read as a language element.
	InGlobalScope bool
	GlobalNames   []string
	GlobalDynamic bool
	// Typed says the text is not a file but a line typed in the person's writing language
	// (codes.Display; the REPL); a file without the coded header is written in Turkish (17.7).
	Typed bool
}

// Result is the outcome of preparation. On error, Stage is the error's stage and Program is
// the tree built up to that stage (it may be incomplete after a syntax error).
type Result struct {
	Parts
	// Text is the source text for error frames, the debugger and the interpreter: the body of
	// a file written in Turkish, the Turkish (canonical) view of a coded body or of a REPL
	// line typed in another writing language. Lines and columns in error messages refer to
	// this text (17.5).
	Text    string
	Program *ast.Program
	Stage   string
	// Errors and Warnings carry their position as a field (messages.PosError): frames and
	// tools read it with messages.Pos, never from the text. A summary line (limitErrors) and
	// a header error have no position of their own.
	Errors   []error
	Warnings []error
}

// Prepare turns source text (the whole file, BOM included) into a program: header parsing,
// lexing, parsing and, when requested, type checking.
func Prepare(text string, s Options) *Result {
	b, err := Split(text)
	r := &Result{Parts: b, Text: b.Body}
	lex := b.Lexer
	coded := b
	// DENEME (iç biçim kod): kodsuz dosya Türkçe yazılmış metindir, kabuk satırı kişinin dilindedir;
	// ikisi de kodlu biçime çevrilip okunur (içeride kimlikler).
	lang, _ := codes.LoadLanguage(codes.SourceLanguage)
	if s.Typed {
		lang = codes.Display()
	}
	if !b.Coded && err == nil && lang != nil {
		// Kabuğa yazılan satır kişinin yazım dilindedir: kodlu biçime, oradan kanonik
		// görünümüne çevrilir (kodlu dosya gibi). Ayrıştırılamayan satır o dilde okunur;
		// sözdizimi hataları satırın kendi metnine göredir.
		if coded, err = toCoded(b, lang, s.GlobalNames, s.GlobalDynamic); err != nil {
			coded, err = b, nil
			lex = func() *lexer.Lexer { return lexer.NewBodyIn(b.Body, false, lang) }
		}
	}
	if coded.Coded {
		var viewLexer *lexer.Lexer
		r.Text, viewLexer = view(coded, codes.Display())
		lex = func() *lexer.Lexer { return viewLexer }
	}
	if err != nil {
		r.Program = &ast.Program{}
		r.Stage, r.Errors = StageSyntax, []error{err}
		return r
	}
	p := parser.New(lex())
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
	chk.SetFileLanguage(coded.Language) // değer metni dosyanın dilinde (kabuk satırı: kişinin dili)
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
