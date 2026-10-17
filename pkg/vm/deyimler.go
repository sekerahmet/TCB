package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

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
		kod := s.Kod
		if kod == opYok {
			kod = islecKodu(s.Operator)
			s.Kod = kod
		}
		err = v.execAssignKod(s.Target, kod, val, s.Line(), env)
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
		// Ad tanımlamayan bloklar (çoğu 'eğer' dalı) için yeni ortam gerekmez: dışarıdan
		// görünür hâle gelecek bir tanım yoktur.
		if blokBayraklari(s)&blokTanimlar == 0 {
			return v.execStatements(s.Statements, env)
		}
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
		task := v.NewTaskVM(true)
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
		for k, exportedVal := range modEnv.store {
			modDict.Set(k, exportedVal)
		}

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
		if v.Debugger == nil && YeniHataAyiklayici != nil {
			v.Debugger = YeniHataAyiklayici(v.SourceFile, v.SourceCode)
		}
		if v.Debugger != nil {
			if s.Condition != nil {
				v.Debugger.KesmeNoktasi(fmt.Sprintf("Koşullu kesme noktası (Satır %d)", s.Line()))
			} else {
				v.Debugger.KesmeNoktasi("Koddaki kesme noktası (kesme_noktası)")
			}
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

// donguOrtami bir döngünün her turunda gövdenin çalışacağı ortamı sağlar. Gövde ad
// tanımlamıyorsa (ve döngü değişkeni yoksa) dış ortam kullanılır. Gövde ortamı
// yakalayamıyorsa (içinde fonksiyon, ertele, eşzamanlı, yapı yoksa) tek bir ortam
// turlar arasında temizlenip yeniden kullanılır; aksi hâlde her tur yeni ortam açılır.
type donguOrtami struct {
	dis     *Environment
	env     *Environment
	gerekli bool
	yeniden bool
}

func yeniDonguOrtami(dis *Environment, govde *ast.BlockStatement, donguDegiskeniVar bool) donguOrtami {
	f := blokBayraklari(govde)
	return donguOrtami{
		dis:     dis,
		gerekli: donguDegiskeniVar || f&blokTanimlar != 0,
		yeniden: f&blokYakalar == 0,
	}
}

func (d *donguOrtami) yeni() *Environment {
	if !d.gerekli {
		return d.dis
	}
	if d.env != nil && d.yeniden {
		d.env.temizle()
		return d.env
	}
	d.env = NewEnvironment(d.dis)
	return d.env
}

func (v *VM) execForIn(s *ast.ForInStatement, env *Environment) (Value, flowSignal, error) {
	iterVal, err := v.evalExpression(s.Iterable, env)
	if err != nil {
		return nil, signalNone, err
	}

	var lastVal Value = NULL
	tur := yeniDonguOrtami(env, s.Body, true)
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
			loopEnv := tur.yeni()
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, NewInt(idx), false)
			}
			loopEnv.Define(s.ValueVar, NewInt(i), false)
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
			loopEnv := tur.yeni()
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, NewInt(int64(idx)), false)
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
			loopEnv := tur.yeni()
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
			loopEnv := tur.yeni()
			if s.KeyVar != "" {
				loopEnv.Define(s.KeyVar, NewInt(int64(idx)), false)
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
	tur := yeniDonguOrtami(env, s.Body, false)
	for {
		cond, err := v.evalExpression(s.Condition, env)
		if err != nil {
			return nil, signalNone, err
		}
		if !cond.IsTruthy() {
			break
		}
		loopEnv := tur.yeni()
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
