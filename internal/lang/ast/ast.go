// Package ast is TR's syntax tree: the statement and expression nodes the parser builds and
// the checker and compiler read. Nodes only carry data; resolution, checking and execution
// live in other packages.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package ast

import (
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

type Node interface {
	TokenLiteral() string
	Line() int
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// Program is the root node of a whole source file.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}
func (p *Program) Line() int {
	if len(p.Statements) > 0 {
		return p.Statements[0].Line()
	}
	return 1
}

// --- STATEMENTS ---

type VarStatement struct {
	Token    lexer.Token // değişken / sabit / :=
	Name     *Identifier
	TypeName string // optional type name (e.g. "Tam", "Metin")
	Value    Expression
	IsConst  bool
}

func (vs *VarStatement) statementNode()       {}
func (vs *VarStatement) TokenLiteral() string { return vs.Token.Literal }
func (vs *VarStatement) Line() int            { return vs.Token.Line }

type AssignStatement struct {
	Token    lexer.Token // =, +=, -=, *=, /=
	Target   Expression  // Identifier, IndexExpression or MemberExpression
	Operator string      // "=", "+=", "-=", "*=", "/="
	Value    Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) Line() int            { return as.Token.Line }

type ReturnStatement struct {
	Token       lexer.Token // sonuç / döndür
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Line() int            { return rs.Token.Line }

type BreakStatement struct {
	Token lexer.Token // çık / kır
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Line() int            { return bs.Token.Line }

type ContinueStatement struct {
	Token lexer.Token // devam
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) Line() int            { return cs.Token.Line }

type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Line() int            { return es.Token.Line }

type BlockStatement struct {
	Token      lexer.Token // {
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Line() int            { return bs.Token.Line }

type ForInStatement struct {
	Token    lexer.Token // döngü
	KeyVar   string      // optional index/key variable
	ValueVar string      // element variable
	Iterable Expression
	Body     *BlockStatement
}

func (fs *ForInStatement) statementNode()       {}
func (fs *ForInStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForInStatement) Line() int            { return fs.Token.Line }

type WhileStatement struct {
	Token     lexer.Token // iken
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) Line() int            { return ws.Token.Line }

type StructField struct {
	Name         string
	TypeName     string
	DefaultValue Expression
}

type StructStatement struct {
	Token   lexer.Token // yapı / sınıf
	Name    string
	Fields  []StructField
	Methods map[string]*FunctionLiteral
}

func (ss *StructStatement) statementNode()       {}
func (ss *StructStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *StructStatement) Line() int            { return ss.Token.Line }

type TryCatchStatement struct {
	Token    lexer.Token // dene
	TryBody  *BlockStatement
	ErrorVar string
	// ImplicitErrorVar reports that the error variable's name was not written, so it is the
	// contextual word 'hata' (TR_ctx_error); in coded form its uses in the body are coded (17.3).
	ImplicitErrorVar bool
	CatchBody        *BlockStatement
}

func (tc *TryCatchStatement) statementNode()       {}
func (tc *TryCatchStatement) TokenLiteral() string { return tc.Token.Literal }
func (tc *TryCatchStatement) Line() int            { return tc.Token.Line }

type ThrowStatement struct {
	Token lexer.Token // fırlat
	Value Expression
}

func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) Line() int            { return ts.Token.Line }

type ImportStatement struct {
	Token lexer.Token // aktar
	Path  string
	Alias string
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) Line() int            { return is.Token.Line }

type AsyncStatement struct {
	Token lexer.Token // eşzamanlı
	Body  *BlockStatement
	Call  Expression
}

func (as *AsyncStatement) statementNode()       {}
func (as *AsyncStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AsyncStatement) Line() int            { return as.Token.Line }

// --- EXPRESSIONS ---

type Identifier struct {
	Token lexer.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) Line() int            { return i.Token.Line }

type IntegerLiteral struct {
	Token lexer.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) Line() int            { return il.Token.Line }

type FloatLiteral struct {
	Token lexer.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) Line() int            { return fl.Token.Line }

type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) Line() int            { return sl.Token.Line }

type InterpolatedString struct {
	Token lexer.Token
	Parts []Expression // ordered StringLiterals and inner expressions
}

func (is *InterpolatedString) expressionNode()      {}
func (is *InterpolatedString) TokenLiteral() string { return is.Token.Literal }
func (is *InterpolatedString) Line() int            { return is.Token.Line }

type BooleanLiteral struct {
	Token lexer.Token
	Value bool
}

func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) Line() int            { return bl.Token.Line }

type NullLiteral struct {
	Token lexer.Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) Line() int            { return nl.Token.Line }

type ListLiteral struct {
	Token    lexer.Token // [
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) Line() int            { return ll.Token.Line }

type DictPair struct {
	Key   Expression
	Value Expression
}

type DictLiteral struct {
	Token lexer.Token // {
	Pairs []DictPair
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DictLiteral) Line() int            { return dl.Token.Line }

type RangeExpression struct {
	Token     lexer.Token // .. or ..=
	Start     Expression
	End       Expression
	Inclusive bool
}

func (re *RangeExpression) expressionNode()      {}
func (re *RangeExpression) TokenLiteral() string { return re.Token.Literal }
func (re *RangeExpression) Line() int            { return re.Token.Line }

type PrefixExpression struct {
	Token    lexer.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) Line() int            { return pe.Token.Line }

type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) Line() int            { return ie.Token.Line }

type ElseIfBranch struct {
	Condition   Expression
	Consequence *BlockStatement
}

type IfExpression struct {
	Token       lexer.Token // eğer
	Condition   Expression
	Consequence *BlockStatement
	ElseIfs     []ElseIfBranch
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) Line() int            { return ie.Token.Line }

type MatchCase struct {
	Patterns  []Expression // values or ranges to match (nil for the default case)
	IsDefault bool
	Body      *BlockStatement
	ValueExpr Expression // the one-line '=> expr' form
}

type MatchExpression struct {
	Token   lexer.Token // eşle
	Subject Expression
	Cases   []MatchCase
}

func (me *MatchExpression) expressionNode()      {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MatchExpression) Line() int            { return me.Token.Line }

type Param struct {
	Name         string
	TypeName     string
	DefaultValue Expression
}

type FunctionLiteral struct {
	Token      lexer.Token // fonksiyon or (
	Name       string
	Parameters []Param
	ReturnType string
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) Line() int            { return fl.Token.Line }

type NamedArg struct {
	Token lexer.Token // the name's token
	Name  string
	Value Expression
}

type CallExpression struct {
	Token     lexer.Token // (
	Function  Expression
	Arguments []Expression
	NamedArgs []NamedArg
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Line() int            { return ce.Token.Line }

type IndexExpression struct {
	Token lexer.Token // [
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) Line() int            { return ie.Token.Line }

type MemberExpression struct {
	Token     lexer.Token // . or ?.
	Object    Expression
	Property  string
	NameToken lexer.Token // the member name's token
	Safe      bool        // true for ?.
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) Line() int            { return me.Token.Line }

// --- MULTI-ASSIGNMENT, LIST COMPREHENSION, DEFER ---

type MultiVarStatement struct {
	Token   lexer.Token
	Names   []*Identifier
	Values  []Expression
	IsConst bool
}

func (mv *MultiVarStatement) statementNode()       {}
func (mv *MultiVarStatement) TokenLiteral() string { return mv.Token.Literal }
func (mv *MultiVarStatement) Line() int            { return mv.Token.Line }

type MultiAssignStatement struct {
	Token   lexer.Token
	Targets []Expression
	Values  []Expression
}

func (ma *MultiAssignStatement) statementNode()       {}
func (ma *MultiAssignStatement) TokenLiteral() string { return ma.Token.Literal }
func (ma *MultiAssignStatement) Line() int            { return ma.Token.Line }

type DeferStatement struct {
	Token lexer.Token // ertele
	Body  *BlockStatement
	Call  Expression
}

func (ds *DeferStatement) statementNode()       {}
func (ds *DeferStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeferStatement) Line() int            { return ds.Token.Line }

type ListComprehension struct {
	Token     lexer.Token // [
	Output    Expression  // x * x
	VarName   string      // x
	Iterable  Expression  // 1..=10
	Condition Expression  // optional: eğer x % 2 == 0
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }
func (lc *ListComprehension) Line() int            { return lc.Token.Line }

type BreakpointStatement struct {
	Token     lexer.Token // kesme_noktası
	Condition Expression  // optional: kesme_noktası eğer x > 10
}

func (bp *BreakpointStatement) statementNode()       {}
func (bp *BreakpointStatement) TokenLiteral() string { return bp.Token.Literal }
func (bp *BreakpointStatement) Line() int            { return bp.Token.Line }

// TestStatement is a test block: test "toplama doğru" { doğrula(topla(2, 2) == 4) }
// It is skipped when the file runs normally; only 'tr test' runs it.
type TestStatement struct {
	Token lexer.Token // test
	Name  string
	Body  *BlockStatement
}

func (ts *TestStatement) statementNode()       {}
func (ts *TestStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TestStatement) Line() int            { return ts.Token.Line }
