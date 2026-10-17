package vm

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/turkce"
)

type flowSignal int

const (
	signalNone flowSignal = iota
	signalReturn
	signalBreak
	signalContinue
)

// MatrixOperator Faz 2'deki Yapay Zeka Matris/Tensör motorunun VM'ye bağlanması için kancadır.
type MatrixOperator interface {
	Add(other Value) (Value, error)
	Sub(other Value) (Value, error)
	Mul(other Value) (Value, error)
	Div(other Value) (Value, error)
	MatMul(other Value) (Value, error)
	GetProperty(prop string) (Value, error)
}

// vmShared tüm görevlerin (ana program, eşzamanlı bloklar, web/GUI istekleri) paylaştığı yorumlayıcı durumudur.
type vmShared struct {
	GlobalEnv *Environment
	Out       io.Writer
	// Stdin girdi() için okunacak kaynaktır (nil ise os.Stdin). VS Code hata ayıklama
	// modunda standart giriş protokole ayrıldığı için burası değiştirilir.
	Stdin      io.Reader
	wg         sync.WaitGroup
	Debugger   *Debugger
	SourceFile string
	SourceCode string
	importedMu sync.Mutex
	Imported   map[string]*DictVal
	importing  map[string]bool

	// gil (Yorumlayıcı Kilidi): Aynı anda yalnızca bir görev TR kodu çalıştırır. Böylece
	// paylaşılan Liste/Sözlük/Yapı değerleri eşzamanlı yazımda bozulmaz ve 's += 1' gibi
	// işlemler bölünmez. Kilit; bekle(), bekle_hepsi(), ağ/disk/süreç beklemeleri gibi
	// engelleyici işlemlerde bırakılır (bkz. Blocking), böylece görevler yine iç içe ilerler.
	gil sync.Mutex
}

// VM, bir yürütme görevinin görünümüdür: paylaşılan durum + göreve özel çağrı yığını.
// Her 'eşzamanlı' blok ve her web/GUI isteği kendi VM görünümüyle çalışır (bkz. newTaskVM).
type VM struct {
	*vmShared
	stackMu   sync.RWMutex
	CallStack []StackFrame

	holdsGIL  bool // bu görünüm şu an yorumlayıcı kilidini tutuyor mu (yalnızca kendi goroutine'i okur/yazar)
	asyncTask bool // 'eşzamanlı' bloğu içinde mi çalışıyor
}

func New() *VM {
	env := NewEnvironment(nil)
	v := &VM{
		vmShared: &vmShared{
			GlobalEnv: env,
			Out:       os.Stdout,
			Imported:  make(map[string]*DictVal),
			importing: make(map[string]bool),
		},
		CallStack: []StackFrame{},
	}
	v.registerBuiltins()
	v.registerAIBuiltins()
	v.registerGUIAndWebBuiltins()
	v.registerDatabaseBuiltins()
	v.registerTestBuiltins()
	return v
}

// newTaskVM aynı paylaşılan durum üzerinde, kendi çağrı yığınına sahip yeni bir görev görünümü üretir.
func (v *VM) newTaskVM(async bool) *VM {
	return &VM{vmShared: v.vmShared, CallStack: v.StackSnapshot(), asyncTask: async}
}

func (v *VM) acquireGIL() {
	v.gil.Lock()
	v.holdsGIL = true
}

func (v *VM) releaseGIL() {
	v.holdsGIL = false
	v.gil.Unlock()
}

// Blocking fn'i yorumlayıcı kilidini bırakarak çalıştırır; uyku, ağ isteği, dış süreç gibi
// uzun süren işlemler sırasında diğer görevlerin (ve web isteklerinin) ilerlemesini sağlar.
func (v *VM) Blocking(fn func()) {
	if !v.holdsGIL {
		fn()
		return
	}
	v.releaseGIL()
	defer v.acquireGIL()
	fn()
}

// runTask fn'i bu görünüm üzerinde yorumlayıcı kilidini alarak çalıştırır ve olası bir
// paniği (programı çökertmek yerine) hataya dönüştürür. Web/GUI istek işleyicileri kullanır.
func (v *VM) runTask(fn func() (Value, error)) (res Value, err error) {
	v.acquireGIL()
	defer v.releaseGIL()
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("İç hata (çökme önlendi): %v", r)
		}
	}()
	return fn()
}

func (v *VM) StackDepth() int {
	v.stackMu.RLock()
	defer v.stackMu.RUnlock()
	return len(v.CallStack)
}

func (v *VM) StackSnapshot() []StackFrame {
	v.stackMu.RLock()
	defer v.stackMu.RUnlock()
	snap := make([]StackFrame, len(v.CallStack))
	copy(snap, v.CallStack)
	return snap
}

func (v *VM) RegisterBuiltin(name string, asciiAlias string, fn BuiltinFunc) {
	b := &BuiltinVal{Name: name, Fn: fn}
	v.GlobalEnv.Define(name, b, true)
	if asciiAlias != "" && asciiAlias != name {
		v.GlobalEnv.Define(asciiAlias, b, true)
	}
}

func (v *VM) Run(prog *ast.Program) (result Value, err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	// Yerleşik fonksiyonlardaki beklenmedik paniklerin tüm süreci çökertmesini engelle
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, fmt.Errorf("İç hata (çökme önlendi): %v", r)
		}
	}()
	defer v.GlobalEnv.RunDefers()
	val, _, err := v.execStatements(prog.Statements, v.GlobalEnv)
	if err != nil {
		return nil, err
	}
	// En üst düzey 'ertele' blokları, başlatılmış eşzamanlı görevler bittikten SONRA çalışmalı
	v.Blocking(v.wg.Wait)
	return val, nil
}

// WaitAsync başlatılmış tüm 'eşzamanlı' görevlerin bitmesini bekler.
func (v *VM) WaitAsync() {
	v.Blocking(v.wg.Wait)
}

// reportAsyncError eşzamanlı görevde oluşan hatayı sessizce yutmak yerine standart hataya yazar.
// suggestName kapsamdaki adlar arasında yazıma en yakın olanı "Bunu mu demek istediniz?" olarak önerir.
func suggestName(name string, env *Environment) string {
	if o := turkce.EnYakin(name, env.Names()); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

func (v *VM) reportAsyncError(line int, err error) {
	fmt.Fprintf(os.Stderr, "⚠️  Eşzamanlı görev hatası (%s başlatılan): %v\n", "satır "+turkce.SayiEk(int64(line), turkce.Bulunma), err)
}

func (v *VM) execStatements(stmts []ast.Statement, env *Environment) (Value, flowSignal, error) {
	var lastVal Value = NULL
	for i := 0; i < len(stmts); i++ {
		stmt := stmts[i]
		if v.Debugger != nil && stmt != nil {
			// kesme_noktası satırının kendisinde değil, ya da kesme_noktası tetiklendikten sonra dur
			if _, isBp := stmt.(*ast.BreakpointStatement); !isBp {
				jumpLine := v.Debugger.BeforeStatement(v, stmt, env)
				if jumpLine > 0 {
					jumped := false
					for j, targetStmt := range stmts {
						if targetStmt != nil && targetStmt.Line() >= jumpLine {
							i = j - 1 // döngü sonunda i++ olacağı için j-1
							jumped = true
							break
						}
					}
					if jumped {
						continue
					}
				}
			}
		}
		val, sig, err := v.execStatement(stmt, env)
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

func (v *VM) execStatement(stmt ast.Statement, env *Environment) (Value, flowSignal, error) {
	if stmt == nil {
		return NULL, signalNone, nil
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		val, err := v.evalExpression(s.Value, env)
		if err != nil {
			return nil, signalNone, err
		}
		env.Define(s.Name.Value, val, s.IsConst)
		return val, signalNone, nil

	case *ast.MultiVarStatement:
		var evalVals []Value
		for _, expr := range s.Values {
			val, err := v.evalExpression(expr, env)
			if err != nil {
				return nil, signalNone, err
			}
			evalVals = append(evalVals, val)
		}
		if len(evalVals) == 1 && len(s.Names) > 1 {
			if lv, ok := evalVals[0].(*ListVal); ok {
				evalVals = lv.Elements
			}
		}
		var lastVal Value = NULL
		for i, name := range s.Names {
			var val Value = NULL
			if i < len(evalVals) {
				val = evalVals[i]
			}
			env.Define(name.Value, val, s.IsConst)
			lastVal = val
		}
		return lastVal, signalNone, nil

	case *ast.AssignStatement:
		val, err := v.evalExpression(s.Value, env)
		if err != nil {
			return nil, signalNone, err
		}
		err = v.execAssign(s.Target, s.Operator, val, s.Line(), env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalNone, nil

	case *ast.MultiAssignStatement:
		// Önce sağdaki tüm ifadeleri hesapla (a, b = b, a takası için kritik!)
		var evalVals []Value
		for _, expr := range s.Values {
			val, err := v.evalExpression(expr, env)
			if err != nil {
				return nil, signalNone, err
			}
			evalVals = append(evalVals, val)
		}
		if len(evalVals) == 1 && len(s.Targets) > 1 {
			if lv, ok := evalVals[0].(*ListVal); ok {
				evalVals = lv.Elements
			}
		}
		var lastVal Value = NULL
		for i, target := range s.Targets {
			var val Value = NULL
			if i < len(evalVals) {
				val = evalVals[i]
			}
			if err := v.execAssign(target, "=", val, s.Line(), env); err != nil {
				return nil, signalNone, err
			}
			lastVal = val
		}
		return lastVal, signalNone, nil

	case *ast.DeferStatement:
		body := s.Body
		call := s.Call
		capturedEnv := env
		env.AddDefer(func() {
			if body != nil {
				_, _, _ = v.execStatements(body.Statements, NewEnvironment(capturedEnv))
			} else if call != nil {
				_, _ = v.evalExpression(call, capturedEnv)
			}
		})
		return NULL, signalNone, nil

	case *ast.ReturnStatement:
		val, err := v.evalExpression(s.ReturnValue, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalReturn, nil

	case *ast.BreakStatement:
		return NULL, signalBreak, nil

	case *ast.ContinueStatement:
		return NULL, signalContinue, nil

	case *ast.BlockStatement:
		blockEnv := NewEnvironment(env)
		return v.execStatements(s.Statements, blockEnv)

	case *ast.ForInStatement:
		return v.execForIn(s, env)

	case *ast.WhileStatement:
		return v.execWhile(s, env)

	case *ast.StructStatement:
		return v.execStructDef(s, env)

	case *ast.TryCatchStatement:
		val, sig, err := v.execStatement(s.TryBody, env)
		if err != nil {
			catchEnv := NewEnvironment(env)
			catchEnv.Define(s.ErrorVar, &StringVal{Val: err.Error()}, false)
			return v.execStatements(s.CatchBody.Statements, catchEnv)
		}
		return val, sig, nil

	case *ast.ThrowStatement:
		val, err := v.evalExpression(s.Value, env)
		if err != nil {
			return nil, signalNone, err
		}
		return nil, signalNone, fmt.Errorf("%s", val.Inspect())

	case *ast.AsyncStatement:
		v.wg.Add(1)
		asyncEnv := NewEnvironment(env)
		asyncEnv.IsFuncRoot = true // görev içindeki 'ertele' blokları görev bitince çalışır
		task := v.newTaskVM(true)
		go func(body *ast.BlockStatement, call ast.Expression, e *Environment) {
			defer v.wg.Done()
			task.acquireGIL()
			defer task.releaseGIL()
			defer func() {
				if r := recover(); r != nil {
					task.reportAsyncError(s.Line(), fmt.Errorf("İç hata (çökme önlendi): %v", r))
				}
			}()
			defer e.RunDefers()
			var err error
			if body != nil {
				_, _, err = task.execStatements(body.Statements, e)
			} else if call != nil {
				_, err = task.evalExpression(call, e)
			}
			if err != nil {
				task.reportAsyncError(s.Line(), err)
			}
		}(s.Body, s.Call, asyncEnv)
		return TRUE, signalNone, nil

	case *ast.ImportStatement:
		path := s.Path
		if !strings.HasSuffix(path, ".tr") {
			path += ".tr"
		}
		resolvedPath := path
		if _, err := os.Stat(resolvedPath); err != nil && !filepath.IsAbs(path) && v.SourceFile != "" && v.SourceFile != "<derlenmiş-uygulama>" {
			candidate := filepath.Join(filepath.Dir(v.SourceFile), path)
			if _, err2 := os.Stat(candidate); err2 == nil {
				resolvedPath = candidate
			}
		}
		absPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			absPath = resolvedPath
		}

		v.importedMu.Lock()
		if v.importing[absPath] {
			v.importedMu.Unlock()
			return nil, signalNone, fmt.Errorf("Satır %d: %q modülünde döngüsel aktarım (circular import) tespit edildi", s.Line(), path)
		}
		cachedMod, hasCached := v.Imported[absPath]
		if !hasCached {
			v.importing[absPath] = true
		}
		v.importedMu.Unlock()

		if hasCached {
			if s.Alias != "" {
				env.Define(s.Alias, cachedMod, true)
				return cachedMod, signalNone, nil
			}
			for _, k := range cachedMod.Order {
				env.Define(k, cachedMod.Pairs[k], false)
			}
			return cachedMod, signalNone, nil
		}

		defer func() {
			v.importedMu.Lock()
			delete(v.importing, absPath)
			v.importedMu.Unlock()
		}()

		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, signalNone, fmt.Errorf("Satır %d: %q modülü okunamadı: %v", s.Line(), path, err)
		}
		code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
		l := lexer.New(code)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return nil, signalNone, fmt.Errorf("Satır %d: %q modülünde sözdizimi hatası: %s", s.Line(), path, p.Errors()[0])
		}

		modEnv := NewEnvironment(v.GlobalEnv)
		prevSource := v.SourceFile
		v.SourceFile = resolvedPath
		val, sig, execErr := v.execStatements(prog.Statements, modEnv)
		v.SourceFile = prevSource
		if execErr != nil {
			return nil, signalNone, execErr
		}

		modDict := NewDictVal()
		modEnv.mu.RLock()
		for k, exportedVal := range modEnv.store {
			modDict.Set(k, exportedVal)
		}
		modEnv.mu.RUnlock()

		v.importedMu.Lock()
		v.Imported[absPath] = modDict
		v.importedMu.Unlock()

		if s.Alias != "" {
			env.Define(s.Alias, modDict, true)
			return modDict, signalNone, nil
		}
		for _, k := range modDict.Order {
			env.Define(k, modDict.Pairs[k], false)
		}
		return val, sig, nil

	case *ast.TestStatement:
		// Test blokları normal çalıştırmada atlanır; 'tr test' bunları RunTest ile çalıştırır.
		return NULL, signalNone, nil

	case *ast.BreakpointStatement:
		if s.Condition != nil {
			condVal, err := v.evalExpression(s.Condition, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !condVal.IsTruthy() {
				return NULL, signalNone, nil
			}
		}
		if v.Debugger == nil {
			v.Debugger = NewDebugger(v.SourceFile, v.SourceCode, true)
		}
		v.Debugger.Mode = StepInto
		v.Debugger.pauseKind = "breakpoint"
		if s.Condition != nil {
			v.Debugger.PauseReason = fmt.Sprintf("Koşullu kesme noktası (Satır %d)", s.Line())
		} else {
			v.Debugger.PauseReason = "Koddaki kesme noktası (kesme_noktası)"
		}
		return NULL, signalNone, nil

	case *ast.ExpressionStatement:
		val, sig, err := v.evalExpressionWithSignal(s.Expression, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, sig, nil
	}
	return NULL, signalNone, nil
}

func (v *VM) execAssign(target ast.Expression, op string, rightVal Value, line int, env *Environment) error {
	applyOp := func(cur Value, rhs Value) (Value, error) {
		switch op {
		case "=":
			return rhs, nil
		case "+=":
			return v.evalInfix("+", cur, rhs, line)
		case "-=":
			return v.evalInfix("-", cur, rhs, line)
		case "*=":
			return v.evalInfix("*", cur, rhs, line)
		case "/=":
			return v.evalInfix("/", cur, rhs, line)
		default:
			return rhs, nil
		}
	}

	switch t := target.(type) {
	case *ast.Identifier:
		var finalVal Value = rightVal
		if op != "=" {
			cur, ok := env.Get(t.Value)
			if !ok {
				return fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken yok%s",
					line, t.Token.Column, t.Value, suggestName(t.Value, env))
			}
			var err error
			finalVal, err = applyOp(cur, rightVal)
			if err != nil {
				return err
			}
		}
		if err := env.Assign(t.Value, finalVal); err != nil {
			var tanimsiz *TanimsizDegiskenHatasi
			if errors.As(err, &tanimsiz) {
				return fmt.Errorf("Satır %d, Sütun %d: %v%s", line, t.Token.Column, err, suggestName(t.Value, env))
			}
			return fmt.Errorf("Satır %d: %v", line, err)
		}
		return nil

	case *ast.IndexExpression:
		// 2 boyutlu Matris hücre ataması kontrolü: A[satır][sütun] = değer
		if innerIdx, is2D := t.Left.(*ast.IndexExpression); is2D {
			baseObj, err := v.evalExpression(innerIdx.Left, env)
			if err != nil {
				return err
			}
			if mat, isMat := baseObj.(*MatrixVal); isMat {
				rIdxVal, err := v.evalExpression(innerIdx.Index, env)
				if err != nil {
					return err
				}
				cIdxVal, err := v.evalExpression(t.Index, env)
				if err != nil {
					return err
				}
				rInt, ok1 := rIdxVal.(*IntVal)
				cInt, ok2 := cIdxVal.(*IntVal)
				if !ok1 || !ok2 {
					return fmt.Errorf("Satır %d: Matris satır ve sütun indeksleri Tam sayı olmalıdır", line)
				}
				rPos := int(rInt.Val)
				if rPos < 0 {
					rPos = mat.Rows + rPos
				}
				cPos := int(cInt.Val)
				if cPos < 0 {
					cPos = mat.Cols + cPos
				}
				if rPos < 0 || rPos >= mat.Rows || cPos < 0 || cPos >= mat.Cols {
					return fmt.Errorf("Satır %d: Matris indeksi sınır dışı ([%d][%d])", line, rInt.Val, cInt.Val)
				}
				finalVal, err := applyOp(&FloatVal{Val: mat.Data[rPos][cPos]}, rightVal)
				if err != nil {
					return err
				}
				fv, ok := toFloat(finalVal)
				if !ok {
					return fmt.Errorf("Satır %d: Matris hücresine sayısal değer atanmalıdır", line)
				}
				mat.Data[rPos][cPos] = fv
				return nil
			}
		}

		left, err := v.evalExpression(t.Left, env)
		if err != nil {
			return err
		}
		idx, err := v.evalExpression(t.Index, env)
		if err != nil {
			return err
		}
		switch container := left.(type) {
		case *ListVal:
			iv, ok := idx.(*IntVal)
			if !ok {
				return fmt.Errorf("Satır %d: Liste indeksi Tam sayı olmalıdır", line)
			}
			pos := int(iv.Val)
			if pos < 0 {
				pos = len(container.Elements) + pos
			}
			if pos < 0 || pos >= len(container.Elements) {
				return fmt.Errorf("Satır %d: Liste indeksi sınır dışı (%d)", line, iv.Val)
			}
			finalVal, err := applyOp(container.Elements[pos], rightVal)
			if err != nil {
				return err
			}
			container.Elements[pos] = finalVal
			return nil
		case *MatrixVal:
			iv, ok := idx.(*IntVal)
			if !ok {
				return fmt.Errorf("Satır %d: Matris satır indeksi Tam sayı olmalıdır", line)
			}
			rPos := int(iv.Val)
			if rPos < 0 {
				rPos = container.Rows + rPos
			}
			if rPos < 0 || rPos >= container.Rows {
				return fmt.Errorf("Satır %d: Matris satır indeksi sınır dışı (%d)", line, iv.Val)
			}
			rowList, ok := rightVal.(*ListVal)
			if !ok || len(rowList.Elements) != container.Cols {
				return fmt.Errorf("Satır %d: Matris satır ataması %d elemanlı sayısal Liste bekler", line, container.Cols)
			}
			for j, el := range rowList.Elements {
				fv, ok := toFloat(el)
				if !ok {
					return fmt.Errorf("Satır %d: Matris elemanları sayısal olmalıdır", line)
				}
				container.Data[rPos][j] = fv
			}
			return nil
		case *DictVal:
			cur, _ := container.GetValue(idx)
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			container.SetValue(idx, finalVal)
			return nil
		}
		return fmt.Errorf("Satır %d: Bu tipe indeks ile atama yapılamaz (%s)", line, left.Type())

	case *ast.MemberExpression:
		obj, err := v.evalExpression(t.Object, env)
		if err != nil {
			return err
		}
		switch inst := obj.(type) {
		case *StructInstanceVal:
			cur := inst.Fields[t.Property]
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			inst.Fields[t.Property] = finalVal
			return nil
		case *DictVal:
			cur := inst.Pairs[t.Property]
			if cur == nil {
				cur = NULL
			}
			finalVal, err := applyOp(cur, rightVal)
			if err != nil {
				return err
			}
			inst.Set(t.Property, finalVal)
			return nil
		}
		return fmt.Errorf("Satır %d: %s tipinin %q alanına atama yapılamaz", line, obj.Type(), t.Property)
	}
	return fmt.Errorf("Satır %d: Geçersiz atama hedefi", line)
}

func (v *VM) execForIn(s *ast.ForInStatement, env *Environment) (Value, flowSignal, error) {
	iterVal, err := v.evalExpression(s.Iterable, env)
	if err != nil {
		return nil, signalNone, err
	}

	var lastVal Value = NULL
	switch it := iterVal.(type) {
	case *RangeVal:
		end := it.End
		if it.Inclusive {
			end++
		}
		idx := int64(0)
		step := int64(1)
		if it.Start > it.End {
			step = -1
			if it.Inclusive {
				end = it.End - 1
			} else {
				end = it.End
			}
		}
		for i := it.Start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			loopEnv := NewEnvironment(env)
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, &IntVal{Val: idx}, false)
			}
			loopEnv.Define(s.ValueVar, &IntVal{Val: i}, false)
			val, sig, err := v.execStatements(s.Body.Statements, loopEnv)
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
			idx++
		}

	case *ListVal:
		for idx, item := range it.Elements {
			loopEnv := NewEnvironment(env)
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, &IntVal{Val: int64(idx)}, false)
			}
			loopEnv.Define(s.ValueVar, item, false)
			val, sig, err := v.execStatements(s.Body.Statements, loopEnv)
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
		}

	case *DictVal:
		for _, k := range it.Order {
			loopEnv := NewEnvironment(env)
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, it.KeyValue(k), false)
				loopEnv.Define(s.ValueVar, it.Pairs[k], false)
			} else {
				loopEnv.Define(s.ValueVar, it.KeyValue(k), false)
			}
			val, sig, err := v.execStatements(s.Body.Statements, loopEnv)
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
		}

	case *StringVal:
		for idx, ch := range []rune(it.Val) {
			loopEnv := NewEnvironment(env)
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, &IntVal{Val: int64(idx)}, false)
			}
			loopEnv.Define(s.ValueVar, &StringVal{Val: string(ch)}, false)
			val, sig, err := v.execStatements(s.Body.Statements, loopEnv)
			if err != nil {
				return nil, signalNone, err
			}
			if sig == signalReturn {
				return val, signalReturn, nil
			}
			if sig == signalBreak {
				break
			}
			lastVal = val
		}

	default:
		return nil, signalNone, fmt.Errorf("Satır %d: %s tipi üzerinde döngü kurulamaz", s.Line(), iterVal.Type())
	}

	return lastVal, signalNone, nil
}

func (v *VM) execWhile(s *ast.WhileStatement, env *Environment) (Value, flowSignal, error) {
	var lastVal Value = NULL
	for {
		cond, err := v.evalExpression(s.Condition, env)
		if err != nil {
			return nil, signalNone, err
		}
		if !cond.IsTruthy() {
			break
		}
		loopEnv := NewEnvironment(env)
		val, sig, err := v.execStatements(s.Body.Statements, loopEnv)
		if err != nil {
			return nil, signalNone, err
		}
		if sig == signalReturn {
			return val, signalReturn, nil
		}
		if sig == signalBreak {
			break
		}
		lastVal = val
	}
	return lastVal, signalNone, nil
}

func (v *VM) execStructDef(s *ast.StructStatement, env *Environment) (Value, flowSignal, error) {
	methods := make(map[string]*FuncVal)
	for name, fnLit := range s.Methods {
		methods[name] = &FuncVal{
			Name:       name,
			Parameters: fnLit.Parameters,
			Body:       fnLit.Body,
			Env:        env,
		}
	}
	def := &StructDefVal{
		Name:    s.Name,
		Fields:  s.Fields,
		Methods: methods,
	}
	env.Define(s.Name, def, true)
	return def, signalNone, nil
}

func (v *VM) evalExpression(expr ast.Expression, env *Environment) (Value, error) {
	val, _, err := v.evalExpressionWithSignal(expr, env)
	return val, err
}

func (v *VM) evalExpressionWithSignal(expr ast.Expression, env *Environment) (Value, flowSignal, error) {
	if expr == nil {
		return NULL, signalNone, nil
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return &IntVal{Val: e.Value}, signalNone, nil
	case *ast.FloatLiteral:
		return &FloatVal{Val: e.Value}, signalNone, nil
	case *ast.StringLiteral:
		return &StringVal{Val: e.Value}, signalNone, nil
	case *ast.InterpolatedString:
		var sb strings.Builder
		for _, part := range e.Parts {
			val, err := v.evalExpression(part, env)
			if err != nil {
				return nil, signalNone, err
			}
			sb.WriteString(val.Inspect())
		}
		return &StringVal{Val: sb.String()}, signalNone, nil
	case *ast.BooleanLiteral:
		if e.Value {
			return TRUE, signalNone, nil
		}
		return FALSE, signalNone, nil
	case *ast.NullLiteral:
		return NULL, signalNone, nil
	case *ast.Identifier:
		val, ok := env.Get(e.Value)
		if !ok {
			return nil, signalNone, fmt.Errorf("Satır %d, Sütun %d: '%s' adında bir değişken ya da fonksiyon yok%s",
				e.Line(), e.Token.Column, e.Value, suggestName(e.Value, env))
		}
		return val, signalNone, nil

	case *ast.ListLiteral:
		elems := make([]Value, 0, len(e.Elements))
		for _, el := range e.Elements {
			val, err := v.evalExpression(el, env)
			if err != nil {
				return nil, signalNone, err
			}
			elems = append(elems, val)
		}
		return &ListVal{Elements: elems}, signalNone, nil

	case *ast.ListComprehension:
		val, err := v.evalListComprehension(e, env)
		return val, signalNone, err

	case *ast.DictLiteral:
		dict := NewDictVal()
		for _, pair := range e.Pairs {
			kVal, err := v.evalExpression(pair.Key, env)
			if err != nil {
				return nil, signalNone, err
			}
			vVal, err := v.evalExpression(pair.Value, env)
			if err != nil {
				return nil, signalNone, err
			}
			dict.SetValue(kVal, vVal)
		}
		return dict, signalNone, nil

	case *ast.RangeExpression:
		startVal, err := v.evalExpression(e.Start, env)
		if err != nil {
			return nil, signalNone, err
		}
		endVal, err := v.evalExpression(e.End, env)
		if err != nil {
			return nil, signalNone, err
		}
		sInt, ok1 := startVal.(*IntVal)
		eInt, ok2 := endVal.(*IntVal)
		if !ok1 || !ok2 {
			return nil, signalNone, fmt.Errorf("Satır %d: Aralık ('..') uçları Tam sayı olmalıdır", e.Line())
		}
		return &RangeVal{
			Start:     sInt.Val,
			End:       eInt.Val,
			Inclusive: e.Inclusive,
		}, signalNone, nil

	case *ast.PrefixExpression:
		right, err := v.evalExpression(e.Right, env)
		if err != nil {
			return nil, signalNone, err
		}
		if e.Operator == "!" {
			if right.IsTruthy() {
				return FALSE, signalNone, nil
			}
			return TRUE, signalNone, nil
		}
		if e.Operator == "-" {
			switch rv := right.(type) {
			case *IntVal:
				return &IntVal{Val: -rv.Val}, signalNone, nil
			case *FloatVal:
				return &FloatVal{Val: -rv.Val}, signalNone, nil
			}
			return nil, signalNone, fmt.Errorf("Satır %d: '-' operatörü %s tipine uygulanamaz", e.Line(), right.Type())
		}

	case *ast.InfixExpression:
		// Kısa devre ve özel operatörler (&&, ||, ??, |>)
		if e.Operator == "&&" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if !left.IsTruthy() {
				return FALSE, signalNone, nil
			}
			right, err := v.evalExpression(e.Right, env)
			if err != nil {
				return nil, signalNone, err
			}
			if right.IsTruthy() {
				return TRUE, signalNone, nil
			}
			return FALSE, signalNone, nil
		}
		if e.Operator == "||" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if left.IsTruthy() {
				return TRUE, signalNone, nil
			}
			right, err := v.evalExpression(e.Right, env)
			if err != nil {
				return nil, signalNone, err
			}
			if right.IsTruthy() {
				return TRUE, signalNone, nil
			}
			return FALSE, signalNone, nil
		}
		if e.Operator == "??" {
			left, err := v.evalExpression(e.Left, env)
			if err != nil {
				return nil, signalNone, err
			}
			if left.Type() != VAL_NULL {
				return left, signalNone, nil
			}
			val, err := v.evalExpression(e.Right, env)
			return val, signalNone, err
		}
		if e.Operator == "|>" {
			val, err := v.evalPipe(e.Left, e.Right, env)
			return val, signalNone, err
		}

		left, err := v.evalExpression(e.Left, env)
		if err != nil {
			return nil, signalNone, err
		}
		right, err := v.evalExpression(e.Right, env)
		if err != nil {
			return nil, signalNone, err
		}
		val, err := v.evalInfix(e.Operator, left, right, e.Line())
		return val, signalNone, err

	case *ast.IfExpression:
		cond, err := v.evalExpression(e.Condition, env)
		if err != nil {
			return nil, signalNone, err
		}
		if cond.IsTruthy() {
			return v.execStatement(e.Consequence, env)
		}
		for _, elif := range e.ElseIfs {
			elifCond, err := v.evalExpression(elif.Condition, env)
			if err != nil {
				return nil, signalNone, err
			}
			if elifCond.IsTruthy() {
				return v.execStatement(elif.Consequence, env)
			}
		}
		if e.Alternative != nil {
			return v.execStatement(e.Alternative, env)
		}
		return NULL, signalNone, nil

	case *ast.MatchExpression:
		return v.evalMatch(e, env)

	case *ast.FunctionLiteral:
		fn := &FuncVal{
			Name:       e.Name,
			Parameters: e.Parameters,
			Body:       e.Body,
			Env:        env,
		}
		if e.Name != "" {
			env.Define(e.Name, fn, false)
		}
		return fn, signalNone, nil

	case *ast.CallExpression:
		val, err := v.evalCall(e, env)
		return val, signalNone, err

	case *ast.IndexExpression:
		val, err := v.evalIndex(e, env)
		return val, signalNone, err

	case *ast.MemberExpression:
		val, err := v.evalMember(e, env)
		return val, signalNone, err
	}

	return NULL, signalNone, nil
}

func (v *VM) evalPipe(left ast.Expression, right ast.Expression, env *Environment) (Value, error) {
	leftVal, err := v.evalExpression(left, env)
	if err != nil {
		return nil, err
	}

	// Sağ taraf bir fonksiyon çağrısı ise (örn: veri |> filtrele((x) => x > 2))
	if callExpr, ok := right.(*ast.CallExpression); ok {
		fnVal, err := v.evalExpression(callExpr.Function, env)
		if err != nil {
			return nil, err
		}
		args := []Value{leftVal}
		for _, a := range callExpr.Arguments {
			av, err := v.evalExpression(a, env)
			if err != nil {
				return nil, err
			}
			args = append(args, av)
		}
		named := make(map[string]Value)
		for _, na := range callExpr.NamedArgs {
			nv, err := v.evalExpression(na.Value, env)
			if err != nil {
				return nil, err
			}
			named[na.Name] = nv
		}
		return v.ApplyFunction(fnVal, args, named, callExpr.Line())
	}

	// Sağ taraf doğrudan bir fonksiyon ise (örn: veri |> sırala)
	fnVal, err := v.evalExpression(right, env)
	if err != nil {
		return nil, err
	}
	return v.ApplyFunction(fnVal, []Value{leftVal}, nil, right.Line())
}

func (v *VM) evalMatch(e *ast.MatchExpression, env *Environment) (Value, flowSignal, error) {
	subj, err := v.evalExpression(e.Subject, env)
	if err != nil {
		return nil, signalNone, err
	}

	for _, mc := range e.Cases {
		matched := mc.IsDefault
		if !matched {
			for _, pat := range mc.Patterns {
				patVal, err := v.evalExpression(pat, env)
				if err != nil {
					return nil, signalNone, err
				}
				if rng, ok := patVal.(*RangeVal); ok {
					if iv, ok := subj.(*IntVal); ok {
						if rng.Inclusive && iv.Val >= rng.Start && iv.Val <= rng.End {
							matched = true
							break
						} else if !rng.Inclusive && iv.Val >= rng.Start && iv.Val < rng.End {
							matched = true
							break
						}
					}
				} else if valuesEqual(subj, patVal) {
					matched = true
					break
				}
			}
		}

		if matched {
			if mc.Body != nil {
				return v.execStatement(mc.Body, env)
			}
			if mc.ValueExpr != nil {
				val, err := v.evalExpression(mc.ValueExpr, env)
				return val, signalNone, err
			}
			return NULL, signalNone, nil
		}
	}

	return NULL, signalNone, nil
}

func (v *VM) evalCall(e *ast.CallExpression, env *Environment) (Value, error) {
	fnVal, err := v.evalExpression(e.Function, env)
	if err != nil {
		return nil, err
	}

	args := make([]Value, 0, len(e.Arguments))
	for _, arg := range e.Arguments {
		av, err := v.evalExpression(arg, env)
		if err != nil {
			return nil, err
		}
		args = append(args, av)
	}

	var named map[string]Value
	if len(e.NamedArgs) > 0 {
		named = make(map[string]Value)
		for _, na := range e.NamedArgs {
			nv, err := v.evalExpression(na.Value, env)
			if err != nil {
				return nil, err
			}
			named[na.Name] = nv
		}
	}

	return v.ApplyFunction(fnVal, args, named, e.Line())
}

func (v *VM) ApplyFunction(fnVal Value, args []Value, named map[string]Value, line int) (Value, error) {
	switch fn := fnVal.(type) {
	case *FuncVal:
		fnEnv := NewEnvironment(fn.Env)
		fnEnv.IsFuncRoot = true
		defer fnEnv.RunDefers()
		fnName := fn.Name
		if fnName == "" {
			fnName = "<anonim>"
		}
		v.stackMu.Lock()
		v.CallStack = append(v.CallStack, StackFrame{
			FuncName: fnName,
			Line:     line,
		})
		v.stackMu.Unlock()
		defer func() {
			v.stackMu.Lock()
			if len(v.CallStack) > 0 {
				v.CallStack = v.CallStack[:len(v.CallStack)-1]
			}
			v.stackMu.Unlock()
		}()
		if fn.BoundSelf != nil {
			fnEnv.Define("kendi", fn.BoundSelf, false)
		}
		for i, param := range fn.Parameters {
			if i < len(args) {
				fnEnv.Define(param.Name, args[i], false)
			} else if named != nil && named[param.Name] != nil {
				fnEnv.Define(param.Name, named[param.Name], false)
			} else if param.DefaultValue != nil {
				defVal, err := v.evalExpression(param.DefaultValue, fn.Env)
				if err != nil {
					return nil, err
				}
				fnEnv.Define(param.Name, defVal, false)
			} else {
				fnEnv.Define(param.Name, NULL, false)
			}
		}
		val, _, err := v.execStatements(fn.Body.Statements, fnEnv)
		if err != nil {
			return nil, err
		}
		return val, nil

	case *BuiltinVal:
		res, err := fn.Fn(v, args, named)
		if err != nil {
			return nil, fmt.Errorf("Satır %d: %v", line, err)
		}
		return res, nil

	case *StructDefVal:
		inst := &StructInstanceVal{
			Def:    fn,
			Fields: make(map[string]Value),
		}
		for i, f := range fn.Fields {
			if i < len(args) {
				inst.Fields[f.Name] = args[i]
			} else if named != nil && named[f.Name] != nil {
				inst.Fields[f.Name] = named[f.Name]
			} else if f.DefaultValue != nil {
				defVal, err := v.evalExpression(f.DefaultValue, v.GlobalEnv)
				if err != nil {
					return nil, err
				}
				inst.Fields[f.Name] = defVal
			} else {
				inst.Fields[f.Name] = NULL
			}
		}
		if initMethod, ok := fn.Methods["kurucu"]; ok {
			boundInit := &FuncVal{
				Name:       initMethod.Name,
				Parameters: initMethod.Parameters,
				Body:       initMethod.Body,
				Env:        initMethod.Env,
				BoundSelf:  inst,
			}
			_, err := v.ApplyFunction(boundInit, args, named, line)
			if err != nil {
				return nil, err
			}
		}
		return inst, nil
	}

	return nil, fmt.Errorf("Satır %d: %s tipi bir fonksiyon olarak çağrılamaz", line, fnVal.Type())
}

func (v *VM) evalListComprehension(lc *ast.ListComprehension, env *Environment) (Value, error) {
	iterVal, err := v.evalExpression(lc.Iterable, env)
	if err != nil {
		return nil, err
	}

	var items []Value
	switch it := iterVal.(type) {
	case *RangeVal:
		end := it.End
		if it.Inclusive {
			end++
		}
		step := int64(1)
		if it.Start > it.End {
			step = -1
			if it.Inclusive {
				end = it.End - 1
			} else {
				end = it.End
			}
		}
		for i := it.Start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			items = append(items, &IntVal{Val: i})
		}
	case *ListVal:
		items = it.Elements
	case *MatrixVal:
		for i := 0; i < it.Rows; i++ {
			row := make([]Value, it.Cols)
			for j := 0; j < it.Cols; j++ {
				row[j] = &FloatVal{Val: it.Data[i][j]}
			}
			items = append(items, &ListVal{Elements: row})
		}
	case *DictVal:
		for _, k := range it.Order {
			items = append(items, it.KeyValue(k))
		}
	case *StringVal:
		for _, ch := range []rune(it.Val) {
			items = append(items, &StringVal{Val: string(ch)})
		}
	default:
		return nil, fmt.Errorf("Satır %d: Liste üretecinde %s tipi üzerinde döngü kurulamaz", lc.Line(), iterVal.Type())
	}

	var results []Value
	for _, item := range items {
		compEnv := NewEnvironment(env)
		compEnv.Define(lc.VarName, item, false)
		if lc.Condition != nil {
			condVal, err := v.evalExpression(lc.Condition, compEnv)
			if err != nil {
				return nil, err
			}
			if !condVal.IsTruthy() {
				continue
			}
		}
		outVal, err := v.evalExpression(lc.Output, compEnv)
		if err != nil {
			return nil, err
		}
		results = append(results, outVal)
	}

	return &ListVal{Elements: results}, nil
}

func (v *VM) evalIndex(e *ast.IndexExpression, env *Environment) (Value, error) {
	left, err := v.evalExpression(e.Left, env)
	if err != nil {
		return nil, err
	}
	idx, err := v.evalExpression(e.Index, env)
	if err != nil {
		return nil, err
	}

	switch container := left.(type) {
	case *ListVal:
		// Dilimleme (Slicing): liste[1..4]
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(container.Elements) {
				end = len(container.Elements)
			}
			if start > end {
				return &ListVal{Elements: []Value{}}, nil
			}
			sliced := make([]Value, end-start)
			copy(sliced, container.Elements[start:end])
			return &ListVal{Elements: sliced}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Liste indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(container.Elements) + pos
		}
		if pos < 0 || pos >= len(container.Elements) {
			return NULL, nil
		}
		return container.Elements[pos], nil

	case *MatrixVal:
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Matris indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = container.Rows + pos
		}
		if pos < 0 || pos >= container.Rows {
			return NULL, nil
		}
		rowElems := make([]Value, container.Cols)
		for j := 0; j < container.Cols; j++ {
			rowElems[j] = &FloatVal{Val: container.Data[pos][j]}
		}
		return &ListVal{Elements: rowElems}, nil

	case *DictVal:
		val, exists := container.GetValue(idx)
		if !exists {
			return NULL, nil
		}
		return val, nil

	case *StringVal:
		runes := []rune(container.Val)
		if rng, ok := idx.(*RangeVal); ok {
			start := int(rng.Start)
			end := int(rng.End)
			if rng.Inclusive {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start > end {
				return &StringVal{Val: ""}, nil
			}
			return &StringVal{Val: string(runes[start:end])}, nil
		}
		iv, ok := idx.(*IntVal)
		if !ok {
			return nil, fmt.Errorf("Satır %d: Metin indeksi Tam sayı olmalıdır", e.Line())
		}
		pos := int(iv.Val)
		if pos < 0 {
			pos = len(runes) + pos
		}
		if pos < 0 || pos >= len(runes) {
			return NULL, nil
		}
		return &StringVal{Val: string(runes[pos])}, nil
	}

	return nil, fmt.Errorf("Satır %d: %s tipi indekslenemez", e.Line(), left.Type())
}

func (v *VM) evalMember(e *ast.MemberExpression, env *Environment) (Value, error) {
	obj, err := v.evalExpression(e.Object, env)
	if err != nil {
		return nil, err
	}
	if obj.Type() == VAL_NULL {
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: 'yok' (null) değerin %q özelliğine erişilemez ('?.' güvenli erişim kullanabilirsiniz)", e.Line(), e.Property)
	}

	// Matris, Tensör, SinirAğı, WebSunucu, GörselArayüz ve Veritabanı özellikleri
	if pg, ok := obj.(PropertyGetter); ok {
		return pg.GetProperty(e.Property)
	}

	switch inst := obj.(type) {
	case *StructInstanceVal:
		if val, ok := inst.Fields[e.Property]; ok {
			return val, nil
		}
		if method, ok := inst.Def.Methods[e.Property]; ok {
			return &FuncVal{
				Name:       method.Name,
				Parameters: method.Parameters,
				Body:       method.Body,
				Env:        method.Env,
				BoundSelf:  inst,
			}, nil
		}
		if e.Safe {
			return NULL, nil
		}
		return nil, fmt.Errorf("Satır %d: %s yapısında %q alanı veya metodu bulunamadı", e.Line(), inst.Def.Name, e.Property)

	case *DictVal, *ListVal, *StringVal:
		return v.evalBuiltinMember(obj, e.Property, e.Safe, e.Line())
	}

	return nil, fmt.Errorf("Satır %d: %s tipinde %q özelliği bulunamadı", e.Line(), obj.Type(), e.Property)
}

func (v *VM) evalInfix(op string, left Value, right Value, line int) (Value, error) {
	// Matris / Tensör operatörleri
	if mop, ok := left.(MatrixOperator); ok {
		switch op {
		case "+":
			return mop.Add(right)
		case "-":
			return mop.Sub(right)
		case "*":
			return mop.Mul(right)
		case "/":
			return mop.Div(right)
		case "@":
			return mop.MatMul(right)
		}
	}
	if mop, ok := right.(MatrixOperator); ok {
		switch op {
		case "+":
			return mop.Add(left)
		case "*":
			return mop.Mul(left)
		}
	}

	switch op {
	case "==":
		if valuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	case "!=":
		if !valuesEqual(left, right) {
			return TRUE, nil
		}
		return FALSE, nil
	}

	// Tam sayı işlemleri
	if li, ok1 := left.(*IntVal); ok1 {
		if ri, ok2 := right.(*IntVal); ok2 {
			switch op {
			case "+":
				sonuc := li.Val + ri.Val
				if (sonuc > li.Val) != (ri.Val > 0) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return &IntVal{Val: sonuc}, nil
			case "-":
				sonuc := li.Val - ri.Val
				if (sonuc < li.Val) != (ri.Val > 0) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return &IntVal{Val: sonuc}, nil
			case "*":
				if !mulFits(li.Val, ri.Val) {
					return nil, tasmaHatasi(line, li.Val, op, ri.Val)
				}
				return &IntVal{Val: li.Val * ri.Val}, nil
			case "/":
				if ri.Val == 0 {
					return nil, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
				}
				if li.Val%ri.Val == 0 {
					return &IntVal{Val: li.Val / ri.Val}, nil
				}
				return &FloatVal{Val: float64(li.Val) / float64(ri.Val)}, nil
			case "%":
				if ri.Val == 0 {
					return nil, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
				}
				return &IntVal{Val: li.Val % ri.Val}, nil
			case "**":
				if ri.Val < 0 {
					return &FloatVal{Val: math.Pow(float64(li.Val), float64(ri.Val))}, nil
				}
				res, ok := intPow(li.Val, ri.Val)
				if !ok {
					return nil, fmt.Errorf("Satır %d: Tam sayı taşması (%d ** %d); ondalık sonuç için örn. %d.0 ** %d yazın", line, li.Val, ri.Val, li.Val, ri.Val)
				}
				return &IntVal{Val: res}, nil
			case "<":
				return boolToVal(li.Val < ri.Val), nil
			case ">":
				return boolToVal(li.Val > ri.Val), nil
			case "<=":
				return boolToVal(li.Val <= ri.Val), nil
			case ">=":
				return boolToVal(li.Val >= ri.Val), nil
			}
		}
	}

	// Ondalık (veya Tam + Ondalık) sayı işlemleri
	lf, ok1 := toFloat(left)
	rf, ok2 := toFloat(right)
	if ok1 && ok2 {
		switch op {
		case "+":
			return &FloatVal{Val: lf + rf}, nil
		case "-":
			return &FloatVal{Val: lf - rf}, nil
		case "*":
			return &FloatVal{Val: lf * rf}, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("Satır %d: Sıfıra bölme hatası", line)
			}
			return &FloatVal{Val: lf / rf}, nil
		case "%":
			if rf == 0 {
				return nil, fmt.Errorf("Satır %d: Sıfıra göre mod alma hatası", line)
			}
			return &FloatVal{Val: math.Mod(lf, rf)}, nil
		case "**":
			return &FloatVal{Val: math.Pow(lf, rf)}, nil
		case "<":
			return boolToVal(lf < rf), nil
		case ">":
			return boolToVal(lf > rf), nil
		case "<=":
			return boolToVal(lf <= rf), nil
		case ">=":
			return boolToVal(lf >= rf), nil
		}
	}

	// Metin birleştirme
	if ls, ok := left.(*StringVal); ok && op == "+" {
		return &StringVal{Val: ls.Val + right.Inspect()}, nil
	}
	if rs, ok := right.(*StringVal); ok && op == "+" {
		return &StringVal{Val: left.Inspect() + rs.Val}, nil
	}

	// Liste birleştirme
	if ll, ok1 := left.(*ListVal); ok1 {
		if rl, ok2 := right.(*ListVal); ok2 && op == "+" {
			combined := make([]Value, 0, len(ll.Elements)+len(rl.Elements))
			combined = append(combined, ll.Elements...)
			combined = append(combined, rl.Elements...)
			return &ListVal{Elements: combined}, nil
		}
	}

	return nil, fmt.Errorf("Satır %d: %s ve %s tipleri arasında %q işlemi desteklenmiyor", line, left.Type(), right.Type(), op)
}

// tasmaHatasi int64 sınırını aşan tam sayı işlemleri için hata üretir (sessizce sarmak yerine).
func tasmaHatasi(line int, a int64, op string, b int64) error {
	return fmt.Errorf("Satır %d: Tam sayı taşması (%d %s %d); ondalık sonuç için örn. %d.0 %s %d yazın", line, a, op, b, a, op, b)
}

func toFloat(v Value) (float64, bool) {
	switch n := v.(type) {
	case *IntVal:
		return float64(n.Val), true
	case *FloatVal:
		return n.Val, true
	default:
		return 0, false
	}
}

func boolToVal(b bool) Value {
	if b {
		return TRUE
	}
	return FALSE
}

func valuesEqual(a, b Value) bool {
	if a.Type() == VAL_NULL && b.Type() == VAL_NULL {
		return true
	}
	if af, ok1 := toFloat(a); ok1 {
		if bf, ok2 := toFloat(b); ok2 {
			return af == bf
		}
	}
	if a.Type() != b.Type() {
		return false
	}
	return a.Inspect() == b.Inspect()
}

// intPow tam sayı üssünü kare-çarp yöntemiyle hesaplar; int64 taşmasında ok=false döner.
func intPow(base, exp int64) (int64, bool) {
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			if !mulFits(result, base) {
				return 0, false
			}
			result *= base
		}
		exp >>= 1
		if exp > 0 {
			if !mulFits(base, base) {
				return 0, false
			}
			base *= base
		}
	}
	return result, true
}

func mulFits(a, b int64) bool {
	if a == 0 || b == 0 {
		return true
	}
	c := a * b
	return c/b == a && !(a == -1 && b == math.MinInt64) && !(b == -1 && a == math.MinInt64)
}
