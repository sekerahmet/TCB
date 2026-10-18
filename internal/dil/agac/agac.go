// Package agac TR'nin sözdizimi ağacıdır: ayrıştırıcının kurduğu, denetçinin ve
// derleyicinin okuduğu deyim ve ifade düğümleri. Düğümler yalnızca veri taşır;
// çözümleme, denetim ve çalıştırma başka paketlerdedir.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package agac

import (
	"tr_dili/internal/dil/sozcuk"
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

// Program tüm kaynak kodun kök düğümüdür.
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

// --- DEYİMLER (STATEMENTS) ---

type VarStatement struct {
	Token    sozcuk.Token // değişken / sabit / :=
	Name     *Identifier
	TypeName string // İsteğe bağlı tip adı (Örn: "Tam", "Metin")
	Value    Expression
	IsConst  bool
}

func (vs *VarStatement) statementNode()       {}
func (vs *VarStatement) TokenLiteral() string { return vs.Token.Literal }
func (vs *VarStatement) Line() int            { return vs.Token.Line }

type AssignStatement struct {
	Token    sozcuk.Token // =, +=, -=, *=, /=
	Target   Expression   // Identifier, IndexExpression veya MemberExpression
	Operator string       // "=", "+=", "-=", "*=", "/="
	Value    Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) Line() int            { return as.Token.Line }

type ReturnStatement struct {
	Token       sozcuk.Token // sonuç / döndür
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Line() int            { return rs.Token.Line }

type BreakStatement struct {
	Token sozcuk.Token // çık / kır
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Line() int            { return bs.Token.Line }

type ContinueStatement struct {
	Token sozcuk.Token // devam
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) Line() int            { return cs.Token.Line }

type ExpressionStatement struct {
	Token      sozcuk.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Line() int            { return es.Token.Line }

type BlockStatement struct {
	Token      sozcuk.Token // {
	Statements []Statement
	// Bayraklar yorumlayıcının blok için bir kez hesapladığı kapsam bilgisidir
	// (ad tanımlıyor mu, ortamı yakalayabilir mi; 0: henüz hesaplanmadı).
	Bayraklar uint8
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Line() int            { return bs.Token.Line }

type ForInStatement struct {
	Token    sozcuk.Token // döngü
	KeyVar   string       // İsteğe bağlı indeks/anahtar değişkeni
	ValueVar string       // Eleman değişkeni
	Iterable Expression
	Body     *BlockStatement
}

func (fs *ForInStatement) statementNode()       {}
func (fs *ForInStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForInStatement) Line() int            { return fs.Token.Line }

type WhileStatement struct {
	Token     sozcuk.Token // iken
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
	Token   sozcuk.Token // yapı / sınıf
	Name    string
	Fields  []StructField
	Methods map[string]*FunctionLiteral
}

func (ss *StructStatement) statementNode()       {}
func (ss *StructStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *StructStatement) Line() int            { return ss.Token.Line }

type TryCatchStatement struct {
	Token     sozcuk.Token // dene
	TryBody   *BlockStatement
	ErrorVar  string
	CatchBody *BlockStatement
}

func (tc *TryCatchStatement) statementNode()       {}
func (tc *TryCatchStatement) TokenLiteral() string { return tc.Token.Literal }
func (tc *TryCatchStatement) Line() int            { return tc.Token.Line }

type ThrowStatement struct {
	Token sozcuk.Token // fırlat
	Value Expression
}

func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) Line() int            { return ts.Token.Line }

type ImportStatement struct {
	Token sozcuk.Token // aktar
	Path  string
	Alias string
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) Line() int            { return is.Token.Line }

type AsyncStatement struct {
	Token sozcuk.Token // eşzamanlı
	Body  *BlockStatement
	Call  Expression
}

func (as *AsyncStatement) statementNode()       {}
func (as *AsyncStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AsyncStatement) Line() int            { return as.Token.Line }

// --- İFADELER (EXPRESSIONS) ---

type Identifier struct {
	Token sozcuk.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) Line() int            { return i.Token.Line }

type IntegerLiteral struct {
	Token sozcuk.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) Line() int            { return il.Token.Line }

type FloatLiteral struct {
	Token sozcuk.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) Line() int            { return fl.Token.Line }

type StringLiteral struct {
	Token sozcuk.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) Line() int            { return sl.Token.Line }

type InterpolatedString struct {
	Token sozcuk.Token
	Parts []Expression // StringLiteral ve iç ifadelerin sıralı listesi
}

func (is *InterpolatedString) expressionNode()      {}
func (is *InterpolatedString) TokenLiteral() string { return is.Token.Literal }
func (is *InterpolatedString) Line() int            { return is.Token.Line }

type BooleanLiteral struct {
	Token sozcuk.Token
	Value bool
}

func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) Line() int            { return bl.Token.Line }

type NullLiteral struct {
	Token sozcuk.Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) Line() int            { return nl.Token.Line }

type ListLiteral struct {
	Token    sozcuk.Token // [
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
	Token sozcuk.Token // {
	Pairs []DictPair
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DictLiteral) Line() int            { return dl.Token.Line }

type RangeExpression struct {
	Token     sozcuk.Token // .. veya ..=
	Start     Expression
	End       Expression
	Inclusive bool
}

func (re *RangeExpression) expressionNode()      {}
func (re *RangeExpression) TokenLiteral() string { return re.Token.Literal }
func (re *RangeExpression) Line() int            { return re.Token.Line }

type PrefixExpression struct {
	Token    sozcuk.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) Line() int            { return pe.Token.Line }

type InfixExpression struct {
	Token    sozcuk.Token
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
	Token       sozcuk.Token // eğer
	Condition   Expression
	Consequence *BlockStatement
	ElseIfs     []ElseIfBranch
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) Line() int            { return ie.Token.Line }

type MatchCase struct {
	Patterns  []Expression // eşleşecek değer veya aralıklar (varsayılan ise nil)
	IsDefault bool
	Body      *BlockStatement
	ValueExpr Expression // Tek satırlık '=> ifade' durumu
}

type MatchExpression struct {
	Token   sozcuk.Token // eşle
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
	Token      sozcuk.Token // fonksiyon veya (
	Name       string
	Parameters []Param
	ReturnType string
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) Line() int            { return fl.Token.Line }

type NamedArg struct {
	Name  string
	Value Expression
}

type CallExpression struct {
	Token     sozcuk.Token // (
	Function  Expression
	Arguments []Expression
	NamedArgs []NamedArg
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Line() int            { return ce.Token.Line }

type IndexExpression struct {
	Token sozcuk.Token // [
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) Line() int            { return ie.Token.Line }

type MemberExpression struct {
	Token    sozcuk.Token // . veya ?.
	Object   Expression
	Property string
	Safe     bool // ?. ise true
}

func (me *MemberExpression) expressionNode()      {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) Line() int            { return me.Token.Line }

// --- SÜPER GÜÇ DÜĞÜMLERİ (Çoklu Atama, Liste Üreteci, Ertele) ---

type MultiVarStatement struct {
	Token   sozcuk.Token
	Names   []*Identifier
	Values  []Expression
	IsConst bool
}

func (mv *MultiVarStatement) statementNode()       {}
func (mv *MultiVarStatement) TokenLiteral() string { return mv.Token.Literal }
func (mv *MultiVarStatement) Line() int            { return mv.Token.Line }

type MultiAssignStatement struct {
	Token   sozcuk.Token
	Targets []Expression
	Values  []Expression
}

func (ma *MultiAssignStatement) statementNode()       {}
func (ma *MultiAssignStatement) TokenLiteral() string { return ma.Token.Literal }
func (ma *MultiAssignStatement) Line() int            { return ma.Token.Line }

type DeferStatement struct {
	Token sozcuk.Token // ertele
	Body  *BlockStatement
	Call  Expression
}

func (ds *DeferStatement) statementNode()       {}
func (ds *DeferStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeferStatement) Line() int            { return ds.Token.Line }

type ListComprehension struct {
	Token     sozcuk.Token // [
	Output    Expression   // x * x
	VarName   string       // x
	Iterable  Expression   // 1..=10
	Condition Expression   // İsteğe bağlı: eğer x % 2 == 0
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }
func (lc *ListComprehension) Line() int            { return lc.Token.Line }

type BreakpointStatement struct {
	Token     sozcuk.Token // kesme_noktası
	Condition Expression   // İsteğe bağlı: kesme_noktası eğer x > 10
}

func (bp *BreakpointStatement) statementNode()       {}
func (bp *BreakpointStatement) TokenLiteral() string { return bp.Token.Literal }
func (bp *BreakpointStatement) Line() int            { return bp.Token.Line }

// TestStatement bir test bloğudur: test "toplama doğru" { doğrula(topla(2, 2) == 4) }
// Dosya normal çalıştırıldığında atlanır; yalnızca 'tr test' çalıştırır.
type TestStatement struct {
	Token sozcuk.Token // test
	Name  string
	Body  *BlockStatement
}

func (ts *TestStatement) statementNode()       {}
func (ts *TestStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TestStatement) Line() int            { return ts.Token.Line }
