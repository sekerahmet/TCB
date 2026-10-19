package source

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// The stages where preparation can stop (Result.Stage). Module errors are reported with
// these names.
const (
	StageSyntax = "sözdizimi"
	StageType   = "tip denetimi"
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
	InGlobalScope bool
	GlobalNames   []string
	GlobalDynamic bool
}

// Result is the outcome of preparation. On error, Stage is the error's stage and Program is
// the tree built up to that stage (it may be incomplete after a syntax error).
type Result struct {
	Parts
	// Text is the source text for error frames, the debugger and the interpreter: the body
	// for a file written in Turkish, the Turkish view of the body for a coded file. Lines and
	// columns in error messages refer to this text (17.5).
	Text     string
	Program  *ast.Program
	Stage    string
	Errors   []string
	Warnings []string
}

// Prepare turns source text (the whole file, BOM included) into a program: header parsing,
// lexing, parsing and, when requested, type checking.
func Prepare(text string, s Options) *Result {
	b, err := Split(text)
	r := &Result{Parts: b, Text: b.Body}
	if b.Coded {
		r.Text = view(b)
	}
	if err != nil {
		r.Program = &ast.Program{}
		r.Stage, r.Errors = StageSyntax, []string{err.Error()}
		return r
	}
	p := parser.New(b.Lexer())
	if s.Module {
		r.Program = p.ParseModule()
	} else {
		r.Program = p.ParseProgram()
	}
	if errs := p.Errors(); len(errs) > 0 {
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
	errs := chk.CheckProgram(r.Program)
	r.Warnings = limitErrors(chk.Warnings())
	if len(errs) > 0 {
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
func limitErrors(errs []string) []string {
	if len(errs) <= MaxErrors {
		return errs
	}
	out := append([]string(nil), errs[:MaxErrors]...)
	return append(out, fmt.Sprintf("… ve %d hata daha (ilk %d hata gösterildi)", len(errs)-MaxErrors, MaxErrors))
}
