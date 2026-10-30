package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// FileKind is what the checked file is (1.1): the same file is a program when it is run and a
// module when it is imported; a file with TR_test is a test file (12.1).
type FileKind uint8

const (
	ProgramFile FileKind = iota
	ModuleFile
	TestFile
)

// SetFile says what the checked file is and gives its text (the lines the positions count: the
// file with its header). A module and a test file hold only definitions at the top level, and
// their top-level values are constant expressions (1.1); the text gives the messages' fix.
func (c *Checker) SetFile(kind FileKind, text string) {
	c.fileKind, c.fileLines = kind, strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// recordValueType keeps the inferred type of every TR_var's and TR_const's first value (nested
// ones too): a module's top-level ones give the fix of H1108, all of them the inference counter.
func (c *Checker) recordValueType(value ast.Expression, typ string) {
	if c.valueTypes == nil {
		c.valueTypes = map[ast.Expression]string{}
	}
	c.valueTypes[value] = typ
}

// HasTests says prog has a TR_test at the top level: it is a test file (12.1).
func HasTests(prog *ast.Program) bool {
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*ast.TestStatement); ok {
			return true
		}
	}
	return false
}

// fileSelect is the messages' select key for the file's kind.
func (c *Checker) fileSelect() string {
	if c.fileKind == TestFile {
		return "testfile"
	}
	return "module"
}

// checkTopLevel applies 1.1 to a module and a test file: at the top level only definitions
// (TR_func, TR_struct, TR_enum, TR_interface, TR_const, TR_var, TR_import and TR_test, which a module
// may hold too, 12.1; H1107), and a TR_const's or TR_var's first value is a constant expression (H1108). A module is
// checked on its own lines; the importer is named by the module error around these (1.3).
func (c *Checker) checkTopLevel(prog *ast.Program) {
	if c.fileKind == ProgramFile {
		return
	}
	k := newConstScope(prog)
	for i, stmt := range prog.Statements {
		next := len(c.fileLines) + 1
		if i+1 < len(prog.Statements) {
			next = startOf(prog.Statements[i+1]).Line
		}
		switch s := stmt.(type) {
		case *ast.VarStatement:
			c.checkConstValue(k, s.Name.Value, s.Value, next)
			k.define(s.Name.Value, s.IsConst)
		case *ast.MultiVarStatement:
			for j, v := range s.Values {
				name := s.Names[min(j, len(s.Names)-1)].Value
				c.checkConstValue(k, name, v, next)
			}
			for _, name := range s.Names {
				k.define(name.Value, s.IsConst)
			}
		case *ast.StructStatement, *ast.ImportStatement, *ast.TestStatement:
		case *ast.ExpressionStatement:
			if f, ok := s.Expression.(*ast.FunctionLiteral); !ok || f.Name == "" || f.Token.Type != lexer.TokFunction {
				c.topLevelStatement(s, next)
			}
		default:
			c.topLevelStatement(stmt, next)
		}
	}
}

// topLevelStatement reports a statement at a module's or a test file's top level (H1107); the fix
// takes the statement's lines into a function.
func (c *Checker) topLevelStatement(stmt ast.Statement, next int) {
	r := startOf(stmt)
	body := c.statementLines(r.Line, 0, next)
	c.addErrorAt(stmt, messages.New(messages.H1107, c.fileSelect(), c.word(codes.KwFunc), indent(body)))
}

// checkConstValue reports a top-level first value that is not a constant expression (H1108); the
// fix makes the value a function's result.
func (c *Checker) checkConstValue(k *constScope, name string, value ast.Expression, next int) {
	bad := k.nonConstant(value)
	if bad == nil {
		return
	}
	r := startOf(value)
	text := c.statementLines(r.Line, r.Col, next)
	shown := ""
	if typ := c.valueTypes[value]; typ != "" && typ != typeAny && typ != typeNull {
		shown = c.typeArg(typ).MessageText(messages.ActiveLanguage())
	}
	c.addErrorAt(bad, messages.New(messages.H1108, c.fileSelect(), name, c.nodeText(bad),
		c.word(codes.KwFunc), snake(name), shown, c.word(codes.KwReturn), strings.ReplaceAll(text, "\n", "\n    ")))
}

// statementLines is the text of a top-level statement: from line start (from column col, 1-based;
// 0: the whole line) up to the line before next, without the trailing empty and comment lines.
func (c *Checker) statementLines(start, col, next int) string {
	end := min(max(next-1, start), len(c.fileLines))
	if start < 1 || start > end {
		return ""
	}
	lines := append([]string(nil), c.fileLines[start-1:end]...)
	if col > 1 {
		lines[0] = string([]rune(lines[0])[min(col-1, len([]rune(lines[0]))):])
	}
	for len(lines) > 1 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "" && !strings.HasPrefix(last, "#") {
			break
		}
		lines = lines[:len(lines)-1]
	}
	lines[0] = strings.TrimSpace(lines[0])
	return strings.TrimRight(strings.Join(lines, "\n"), " \t")
}

// indent writes text one level (four spaces) in, as a function body.
func indent(text string) string {
	return "    " + strings.ReplaceAll(text, "\n", "\n    ")
}

// nodeText is how a message names the part of a value that is not constant: the called name or
// the variable, as the file writes it.
func (c *Checker) nodeText(n ast.Node) string {
	if call, ok := n.(*ast.CallExpression); ok {
		n = call.Function
	}
	r := startOf(n)
	if r.Line >= 1 && r.Line <= len(c.fileLines) && r.EndLine == r.Line && r.Col >= 1 {
		line := []rune(c.fileLines[r.Line-1])
		if r.EndCol-1 <= len(line) && r.Col < r.EndCol {
			return string(line[r.Col-1 : r.EndCol-1])
		}
	}
	return n.TokenLiteral()
}

// startOf is where n starts; a node without a token of its own place starts on its line.
func startOf(n ast.Node) messages.Range {
	r := ast.RangeOf(n)
	if r.Line == 0 {
		r.Line = n.Line()
	}
	return r
}
