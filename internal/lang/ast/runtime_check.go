package ast

import "github.com/sekerahmet/TCB/internal/lang/messages"

// RuntimeCheck is the checker's mark on a value that enters a typed place while the checker
// does not know its type, or knows a container without its element types (1.2): a definition,
// an assignment, an argument to a program function, a return, a field write, a constructor's
// field, a container element. The value fit the place only because its type was not known; the
// run time checks it against Expected when it enters (interp/runtime_checks.go). A value whose
// type the checker knows gets no mark.
type RuntimeCheck struct {
	// Expected is the place's type in the checker's text (TR_List[TR_String]).
	Expected string
	// Op is the operator of a compound assignment (x += v): the value that enters is the
	// result, cur Op v; empty for every other place.
	Op string
	// Code is the place's check code (H1001 ...); the run-time error uses it with Args.
	Code messages.Code
	// Args are the error's arguments as the checker builds them; Args[Given] (the value's run-time
	// type) and Args[Hint] (H1170, the first element that does not fit) are filled at run time.
	Args        []any
	Given, Hint int
	// Spelled writes the run-time type in the file's identity spelling (TR_Int), as the checker
	// writes the place's own type; otherwise in the message's language.
	Spelled bool
	// Text is the value's spelling in the program, the start of the hint's path; empty when it
	// does not fit on one line or holds a backtick, which would close the message's code span
	// (no hint then).
	Text string
	// At is the node the check error is reported at; the run-time error has the same place.
	At Node
	// Structs are the struct names the checker knows in this program: only two known structs
	// are told apart, as in the checker (a name it does not know fits).
	Structs map[string]bool
}

// RuntimeChecks are a program's marks, by the marked value (an expression, or the assignment
// statement of a compound assignment). The checker writes them (Program.RuntimeChecks); the
// interpreter merges the marks of every program it runs or imports.
type RuntimeChecks map[Node]*RuntimeCheck
