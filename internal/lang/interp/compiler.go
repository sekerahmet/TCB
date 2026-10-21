package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// --- KAPANIŞ DERLEYİCİSİ ---
//
// AST, çalıştırılmadan önce bir kez Go kapanışlarına (closure) çevrilir; böylece ağaç her
// turda yeniden dolaşılmaz, işleç metinleri ve sabitler derleme anında çözülür. Değişkenler
// mümkün olduğunda derleme anında (derinlik, yuva) çiftine çözülür ve çalışma anında ad
// araması yapılmaz.
//
// Derleme anındaki her kapsam (derleyiciKapsam) çalışma anındaki tam olarak bir ortama
// karşılık gelir: fonksiyon çağrısı, ad tanımlayan blok, döngü turu, yakala bloğu,
// eşzamanlı/ertele gövdesi, liste üreteci öğesi, modül ve genel kapsam. Ad tanımlamayan
// bloklar ortam açmaz ve kapsam da oluşturmaz. Bu birebirlik sayesinde derinlik çalışma
// anında da geçerlidir.
//
// Bir yuva nil ise (ad henüz tanımlanmamış: tanım ilerideyse ya da hata ayıklayıcı
// 'zıpla' ile atlandıysa) arama adla, ağaç yorumlayıcısındaki gibi sürdürülür; bu yüzden
// anlam değişmez.

type exprCode func(v *VM, env *Environment) (Value, error)
type statementCode func(v *VM, env *Environment) (Value, flowSignal, error)

// compilerScope derleme anındaki bir kapsamdır.
type compilerScope struct {
	// info kapsamın ad tablosudur. nil ise kapsam, düzeni derleme anında bilinmeyen ve
	// çalışma anında verilen bir ortamdır (hata ayıklayıcının değerlendirdiği ifadeler):
	// buradan geçen adlar adla aranır, buradaki tanımlar adla yapılır.
	info   *scopeInfo
	parent *compilerScope
	// global, bu kapsam genel kapsamsa onun ortamıdır; genel adlara derinlik yürümeden
	// doğrudan erişilir.
	global *Environment
	// dynamic: kapsamda takma adsız bir 'aktar' var; adları çalışma anında eklenir ve dış
	// kapsamlardaki aynı adları gölgeleyebilir. Bu kapsamdan geçen adlar adla aranır.
	dynamic bool
	// shadow bu kapsamda yerleşik adlarını gölgeleyen kullanıcı tanımlarıdır (shadow.go);
	// yalnızca hata mesajı için tutulur, çoğu kapsamda nil'dir.
	shadow map[string]shadowDef
	// notYet bir parametrenin varsayılan değeri derlenirken o parametre ve sonrakilerin
	// adlarıdır: varsayılan yalnızca önceki parametreleri görür; bu adlar dış kapsamdaki
	// aynı adlı değişkene çözülmez, okunursa hata verir.
	notYet map[string]bool
}

// define bir adı kapsamın tablosuna ekler (derleme anı).
func (k *compilerScope) define(name string) {
	if name == "" {
		k.dynamic = true
		return
	}
	if k.info != nil {
		k.info.slot(name)
	}
}

// predefine deyimlerin bu kapsama tanımlayacağı tüm adlara, deyimler derlenmeden önce yuva
// ayırır. Böylece tanımdan önce gelen bir kullanım da (ör. hata ayıklayıcı geri atladığında)
// doğru yuvaya çözülür.
func (k *compilerScope) predefine(stmts []ast.Statement) {
	for _, st := range stmts {
		statementNames(st, k.define)
		k.statementShadows(st)
	}
}

const (
	resolveDynamic = iota
	resolveLocal
	resolveGlobal
	resolveNotYet // varsayılan değerde henüz bağlanmamış bir parametre (bkz. henuzYok)
)

type resolution struct {
	kind      int
	nestDepth int
	slot      int
	global    *Environment
}

// resolve bir adı derleme anında çözer.
func (k *compilerScope) resolve(name string) resolution {
	d := 0
	for c := k; c != nil; c = c.parent {
		if c.notYet[name] {
			return resolution{kind: resolveNotYet}
		}
		if c.info == nil {
			return resolution{}
		}
		if i, ok := c.info.index[name]; ok {
			if c.global != nil {
				return resolution{kind: resolveGlobal, slot: i, global: c.global}
			}
			return resolution{kind: resolveLocal, nestDepth: d, slot: i}
		}
		if c.dynamic {
			return resolution{}
		}
		d++
	}
	return resolution{}
}

// compiler bir derleme işinin durumudur.
type compiler struct {
	// root genel kapsamdır (yapı alanlarının varsayılanları burada değerlendirilir).
	root *compilerScope
	// source derlenen kodun dosyasıdır (nil: ana program); fonksiyonlar onu taşır.
	source *Source
	// blocks blokların kapsam bayraklarının önbelleğidir (scope_analysis.go).
	blocks blockFlagCache
}

// blockFlags bloğun kapsam bayraklarıdır (blockDefines, blockCaptures).
func (c *compiler) blockFlags(b *ast.BlockStatement) uint8 {
	return c.blocks.flags(b)
}

// newCompiler görevin o an çalıştığı dosya için bir derleyici açar.
func (v *VM) newCompiler() *compiler {
	return &compiler{root: &compilerScope{info: v.GlobalEnv.scope, global: v.GlobalEnv}, source: v.source, blocks: blockFlagCache{}}
}

// subScope ust içinde yeni bir ortama karşılık gelen kapsam açar.
func subScope(parent *compilerScope) *compilerScope {
	return &compilerScope{info: &scopeInfo{}, parent: parent}
}

// boundaryScope çalışma anında verilecek, düzeni bilinmeyen bir ortamı temsil eder.
func boundaryScope() *compilerScope {
	return &compilerScope{}
}

// identifier k kapsamına ad tanımlayan bir işlev döndürür (yuvalı ya da adla).
func identifier(k *compilerScope, name string) func(env *Environment, val Value, constant bool) {
	if k.info == nil {
		return func(env *Environment, val Value, constant bool) { env.Define(name, val, constant) }
	}
	i := k.info.slot(name)
	return func(env *Environment, val Value, constant bool) { env.define(i, val, constant) }
}

// --- BLOKLAR ---

// blockCode derlenmiş bir deyim listesidir. Deyimlerin kendisi hata ayıklayıcı (her
// deyimden önce danışılır) ve 'zıpla' (satıra atlama) için saklanır.
type blockCode struct {
	statements []ast.Statement
	codeList   []statementCode
}

func (c *compiler) statements(k *compilerScope, stmts []ast.Statement) *blockCode {
	b := &blockCode{statements: stmts, codeList: make([]statementCode, len(stmts))}
	for i, st := range stmts {
		b.codeList[i] = c.statement(k, st)
	}
	return b
}

func (b *blockCode) run(v *VM, env *Environment) (Value, flowSignal, error) {
	var lastVal Value = Null
	codeList := b.codeList
	for i := 0; i < len(codeList); i++ {
		if i > 0 {
			// Önceki deyimin değeri kullanılmayacak: bağı kalmayan sahipli değerler
			// bırakılır (ownership.go).
			v.boundary()
		}
		if v.Debugger != nil {
			if j, jump := b.consultDebugger(v, i, env); jump {
				i = j - 1 // döngü sonunda i++ olacağı için j-1
				continue
			}
		}
		val, sig, err := codeList[i](v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if sig != signalNone {
			return val, sig, nil
		}
		lastVal = val
	}
	return lastVal, signalNone, nil
}

// consultDebugger i. deyimden önce hata ayıklayıcıya danışır; ayıklayıcı bloğun içinde
// bir satıra zıplamak isterse o deyimin indeksini ve doğru döner. Sıcak döngünün dışında
// tutulur (calistir'ın gövdesi küçük kalsın).
//
//go:noinline
func (b *blockCode) consultDebugger(v *VM, i int, env *Environment) (int, bool) {
	stmt := b.statements[i]
	if stmt == nil {
		return 0, false
	}
	// kesme_noktası satırının kendisinde değil, ya da kesme_noktası tetiklendikten sonra dur
	if _, isBp := stmt.(*ast.BreakpointStatement); isBp {
		return 0, false
	}
	v.activeBlock = b // hata ayıklayıcı zıplama hedeflerini bu bloğa göre sorar
	jumpLine := v.Debugger.BeforeStatement(v, stmt, env)
	if jumpLine <= 0 {
		return 0, false
	}
	dest, ok := b.jumpTarget(jumpLine)
	if !ok {
		return 0, false
	}
	for j, targetStmt := range b.statements {
		if targetStmt != nil && targetStmt.Line() == dest {
			return j, true
		}
	}
	return 0, false
}

// zıplamaHedefi blokta satir'a zıplanabiliyorsa zıplanacak deyimin satırını verir: satırdaki
// ya da ondan sonraki ilk deyim; satır bloğun ilk deyiminden önceyse ya da son deyiminin
// başından sonraysa (blok dışı) zıplanamaz.
func (b *blockCode) jumpTarget(lineNo int) (int, bool) {
	first := true
	for _, st := range b.statements {
		if st == nil {
			continue
		}
		if first && lineNo < st.Line() {
			return 0, false
		}
		first = false
		if st.Line() >= lineNo {
			return st.Line(), true
		}
	}
	return 0, false
}

// JumpTarget, while the debugger is paused, returns the line of the statement to jump to when
// execution can jump to satir inside the paused block (the editor's "goto" request). Jumps are
// only possible between the statements of the block running now; for a line outside the block
// ok is false. Must be called on the paused task's own goroutine.
func (v *VM) JumpTarget(lineNo int) (int, bool) {
	if v.activeBlock == nil {
		return 0, false
	}
	return v.activeBlock.jumpTarget(lineNo)
}

// blockStatement bir bloğu derler: blok ad tanımlıyorsa her çalışmada yeni bir ortam açar,
// tanımlamıyorsa çevredeki ortamda çalışır.
func (c *compiler) blockStatement(k *compilerScope, b *ast.BlockStatement) statementCode {
	if b == nil {
		return emptyStatement
	}
	if c.blockFlags(b)&blockDefines == 0 {
		return c.statements(k, b.Statements).run
	}
	sub := subScope(k)
	sub.predefine(b.Statements)
	bk := c.statements(sub, b.Statements)
	info := sub.info
	info.local = c.blockFlags(b)&blockCaptures == 0
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		e := newEnv(env, info)
		val, sig, err := bk.run(v, e)
		e.closeEnv()
		return val, sig, err
	}
}

func emptyStatement(v *VM, env *Environment) (Value, flowSignal, error) {
	return Null, signalNone, nil
}

// --- GİRİŞ NOKTALARI ---

// runProgram deyimleri genel kapsamda derleyip genel ortamda çalıştırır.
func (v *VM) runProgram(stmts []ast.Statement) (Value, flowSignal, error) {
	c := v.newCompiler()
	c.root.predefine(stmts)
	return c.statements(c.root, stmts).run(v, v.GlobalEnv)
}

// genelAltindaCalistir deyimleri genel kapsamın altında yeni bir ortamda çalıştırır
// (testler); ortam çağırana verilir ki 'ertele' blokları çalıştırılabilsin.
func (v *VM) compileUnderGlobal(stmts []ast.Statement) (*blockCode, *scopeInfo) {
	c := v.newCompiler()
	sub := subScope(c.root)
	sub.predefine(stmts)
	return c.statements(sub, stmts), sub.info
}

// dynamicExpr ifadeyi, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) değerlendirir.
func (v *VM) dynamicExpr(expr ast.Expression, env *Environment) (Value, error) {
	return v.newCompiler().expression(boundaryScope(), expr)(v, env)
}

// dynamicStatements deyimleri, düzeni bilinmeyen bir ortamda (adla çözümlemeyle) çalıştırır.
func (v *VM) dynamicStatements(stmts []ast.Statement, env *Environment) (Value, flowSignal, error) {
	return v.newCompiler().statements(boundaryScope(), stmts).run(v, env)
}
