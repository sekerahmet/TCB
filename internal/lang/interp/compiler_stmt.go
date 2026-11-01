package interp

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// --- DEYİMLER ---

func (c *compiler) statement(k *compilerScope, stmt ast.Statement) statementCode {
	switch s := stmt.(type) {
	case nil:
		return emptyStatement
	case *ast.VarStatement:
		return c.varStatement(k, s)

	case *ast.MultiVarStatement:
		return c.multiVarStatement(k, s)

	case *ast.AssignStatement:
		return c.assignStatement(k, s)

	case *ast.MultiAssignStatement:
		return c.multiAssignStatement(k, s)

	case *ast.DeferStatement:
		return c.deferStatement(k, s)

	case *ast.ReturnStatement:
		return c.returnStatement(k, s)

	case *ast.BreakStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return Null, signalBreak, nil
		}

	case *ast.ContinueStatement:
		return func(v *VM, env *Environment) (Value, flowSignal, error) {
			return Null, signalContinue, nil
		}

	case *ast.BlockStatement:
		return c.blockStatement(k, s)

	case *ast.ForInStatement:
		return c.inLoop(k, s)

	case *ast.WhileStatement:
		return c.whileStmt(k, s)

	case *ast.StructStatement:
		return c.structDef(k, s)

	case *ast.TryCatchStatement:
		return c.tryCatchStatement(k, s)

	case *ast.ThrowStatement:
		return c.throwStatement(k, s)

	case *ast.AsyncStatement:
		return c.concurrent(k, s)

	case *ast.ImportStatement:
		return c.importStatement(k, s)

	case *ast.TestStatement:
		// Test blokları normal çalıştırmada atlanır; 'tr test' bunları RunTest ile çalıştırır.
		return emptyStatement

	case *ast.BreakpointStatement:
		return c.breakpointStatement(k, s)

	case *ast.ExpressionStatement:
		return c.exprSignal(k, s.Expression)
	}
	return emptyStatement
}

// varStatement 'değişken'/'sabit' tanımını derler.
func (c *compiler) varStatement(k *compilerScope, s *ast.VarStatement) statementCode {
	value := c.expression(k, s.Value)
	definition := identifier(k, s.Name.Value)
	constant := s.IsConst
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		definition(env, val, constant)
		return val, signalNone, nil
	}
}

// multiVarStatement çoklu tanımı (değişken a, b = ...) derler.
func (c *compiler) multiVarStatement(k *compilerScope, s *ast.MultiVarStatement) statementCode {
	values := c.multiSides(k, s.Values)
	definitions := make([]func(*Environment, Value, bool), len(s.Names))
	for i, n := range s.Names {
		definitions[i] = identifier(k, n.Value)
	}
	constant := s.IsConst
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return applyUnpacked(v, env, s, values, len(definitions), func(i int, val Value) error {
			definitions[i](env, val, constant)
			return nil
		})
	}
}

// assignStatement atamayı (bileşik atama dahil) derler.
func (c *compiler) assignStatement(k *compilerScope, s *ast.AssignStatement) statementCode {
	value := c.expression(k, s.Value)
	dest := c.assignTarget(k, s.Target, opCode(string(s.Operator)), s.Line())
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if err := dest(v, env, val); err != nil {
			return nil, signalNone, err
		}
		return val, signalNone, nil
	}
}

// multiAssignStatement çoklu atamayı (a, b = b, a) derler.
func (c *compiler) multiAssignStatement(k *compilerScope, s *ast.MultiAssignStatement) statementCode {
	// Önce sağdaki tüm ifadeler hesaplanır (a, b = b, a takası için kritik!)
	values := c.multiSides(k, s.Values)
	dests := make([]func(*VM, *Environment, Value) error, len(s.Targets))
	for i, t := range s.Targets {
		dests[i] = c.assignTarget(k, t, opAssign, s.Line())
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return applyUnpacked(v, env, s, values, len(dests), func(i int, val Value) error {
			return dests[i](v, env, val)
		})
	}
}

// applyUnpacked çoklu tanım/atamanın sağ tarafını hesaplar ve her değeri sırayla apply ile
// i. hedefe uygular; sonuç son değerdir.
func applyUnpacked(v *VM, env *Environment, stmt ast.Statement, values []exprCode, count int, apply func(i int, val Value) error) (Value, flowSignal, error) {
	evalVals, err := multiValues(v, env, stmt, values, count)
	if err != nil {
		return nil, signalNone, err
	}
	var lastVal Value = Null
	for i := 0; i < count; i++ {
		val := evalVals[i]
		if err := apply(i, val); err != nil {
			return nil, signalNone, err
		}
		lastVal = val
	}
	return lastVal, signalNone, nil
}

// deferStatement 'ertele' deyimini derler.
func (c *compiler) deferStatement(k *compilerScope, s *ast.DeferStatement) statementCode {
	var bodyBlock *blockCode
	var bodyInfo *scopeInfo
	var call exprCode
	if s.Body != nil {
		sub := subScope(k)
		sub.predefine(s.Body.Statements)
		bodyBlock = c.statements(sub, s.Body.Statements)
		bodyInfo = sub.info
	} else if s.Call != nil {
		call = c.expression(k, s.Call)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		capturedEnv := env
		// Ertelenen işin hatası yutulmaz; fonksiyonun hatasına eklenir (9.4).
		env.AddDefer(func() error {
			if bodyBlock != nil {
				// Ertelenen bloğun kendisi bir kapsamdır (9.1): içinde kaydedilen ertele bloğun sonunda
				// çalışır. Fonksiyonun kökü kendi listesini çoktan boşalttığından oraya kaydedilen iş
				// kaybolurdu.
				blockEnv := newEnv(capturedEnv, bodyInfo)
				blockEnv.IsFuncRoot = true
				_, _, err := bodyBlock.run(v, blockEnv)
				return addDeferError(err, v.runDefers(blockEnv))
			} else if call != nil {
				_, err := call(v, capturedEnv)
				return err
			}
			return nil
		})
		return Null, signalNone, nil
	}
}

// returnStatement 'dön' deyimini derler.
func (c *compiler) returnStatement(k *compilerScope, s *ast.ReturnStatement) statementCode {
	value := c.expression(k, s.ReturnValue) // TR_return a, b bir demet döndürür (6.3)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return val, signalReturn, nil
	}
}

// tryCatchStatement 'dene'/'yakala' deyimini derler.
func (c *compiler) tryCatchStatement(k *compilerScope, s *ast.TryCatchStatement) statementCode {
	tryBlock := c.blockStatement(k, s.TryBody)
	sub := subScope(k)
	errSlot := sub.info.slot(s.ErrorVar)
	if s.CatchBody != nil {
		sub.recordShadow(s.ErrorVar, s.CatchBody.Line(), messages.H2453)
	}
	var catchStatements []ast.Statement
	if s.CatchBody != nil {
		catchStatements = s.CatchBody.Statements
	}
	sub.predefine(catchStatements)
	catchClause := c.scopedStatements(sub, catchStatements)
	info := sub.info
	// Yakalanan hata bir TR_Error değeridir (8.3): kodu, fırlatılan değeri, nedeni ve bastırılan
	// hatalarıyla; metni fırlatılan değerin metnidir, okuyanın diline bağlı değildir (3.14).
	// Yakalanmayan hata ekranda TR_LANG'dedir (görüntü, 8.5).
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, sig, err := tryBlock(v, env)
		if err != nil {
			if uncatchable(err) {
				return nil, signalNone, err // kesilme (Ctrl+C) ve program çıkışı isteği yakalanmaz
			}
			catchEnv := newEnv(env, info)
			catchEnv.define(errSlot, errorValue(err), false)
			if IsStopRequestedError(err) {
				// Durdurma hatasını yakalayan gövde bir temizlik işidir: orada beklemeler normal çalışır (1.6)
				v.cleanupDepth++
				defer func() { v.cleanupDepth-- }()
			}
			return catchClause.run(v, catchEnv)
		}
		return val, sig, nil
	}
}

// throwStatement 'fırlat' deyimini derler. Fırlatılabilen değer metin, ileti, durumlu tip değeri ya
// da hatadır (8.2); başkası çalışma hatasıdır. Hata fırlatıldığı satırı taşır: yakalanmazsa program
// konum ve kod çerçevesiyle durur, görev raporu satırı söyler; yakalanırsa fırlatılan değeriyle bir
// TR_Error olur (8.3). Yakalanmış hata (TR_throw e) olduğu gibi yeniden fırlar.
func (c *compiler) throwStatement(k *compilerScope, s *ast.ThrowStatement) statementCode {
	value := c.expression(k, s.Value)
	line := s.Line()
	// Fırlatılamayan değerin önerisi programın kendi yazımıyladır (TR_throw TR_String(sayı)):
	// kopyalanınca çalışır; değer bir ad değilse öneri yalnız TR_String'i anar.
	example := ""
	if name := ast.TargetName(s.Value); name != "" {
		example = codes.Spelling(codes.KwThrow) + " " + codes.Spelling(codes.TypeString) + "(" + name + ")"
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		if e, ok := val.(*ErrorVal); ok {
			return nil, signalNone, e.raise(line)
		}
		if !throwable(val) {
			return nil, signalNone, messages.At(line, 0, messages.New(messages.H2911, codes.NameRef(codes.KwThrow), TypeAndSummary(val), example))
		}
		return nil, signalNone, (&ErrorVal{thrown: val}).raise(line)
	}
}

// thrownError 'fırlat' ile başlatılan hatadır; metni fırlatılan değerin metnidir. caught onu
// yakalayanın göreceği TR_Error'dır (errorValue).
type thrownError struct {
	value  *messages.Error
	caught *ErrorVal
}

func (e *thrownError) Error() string { return e.value.Error() }

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *thrownError) MessageText(lang string) string { return messages.ErrorIn(e.value, lang) }

func (e *thrownError) Unwrap() error { return e.value }

// importStatement 'aktar' deyimini derler.
func (c *compiler) importStatement(k *compilerScope, s *ast.ImportStatement) statementCode {
	var alias func(*Environment, Value, bool)
	if s.Alias != "" {
		alias = identifier(k, s.Alias)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return v.imp(s, env, alias)
	}
}

// breakpointStatement 'kesme_noktası' deyimini derler.
func (c *compiler) breakpointStatement(k *compilerScope, s *ast.BreakpointStatement) statementCode {
	var condition exprCode
	if s.Condition != nil {
		condition = c.expression(k, s.Condition)
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		if condition != nil {
			condVal, err := condition(v, env)
			if err != nil {
				return nil, signalNone, err
			}
			if ok, err := truth(condVal, s.Line()); err != nil {
				return nil, signalNone, err
			} else if !ok {
				return Null, signalNone, nil
			}
		}
		if v.Debugger == nil && NewDebugger != nil {
			k := v.source
			if k == nil {
				k = &Source{Path: v.SourceFile, Code: v.SourceCode}
			}
			v.Debugger = NewDebugger(k.Path, k.Code)
		}
		if v.Debugger != nil {
			if s.Condition != nil {
				v.Debugger.Breakpoint(messages.Text(messages.H2446, s.Line()))
			} else {
				v.Debugger.Breakpoint(messages.Text(messages.H2520, codes.NameRef(codes.KwBreakpoint)))
			}
		}
		return Null, signalNone, nil
	}
}

// multiSides çoklu tanım ve atamanın sağ yanının kodudur.
func (c *compiler) multiSides(k *compilerScope, values []ast.Expression) []exprCode {
	return c.expressions(k, values)
}

// multiValues çoklu tanım/atamanın sağ tarafını hesaplar; tek bir demet ya da liste birden çok
// ada açılır (5.3, 6.3). Değer sayısı ad sayısını tutmazsa çalışma hatasıdır: eksik ya da fazla
// değer sessizce kaybolmaz. Denetleyici sayıyı yalnız sağ yan bir demet yazımı ya da dönüşü
// demet tipli bir fonksiyonun doğrudan çağrısıysa bilir (checker/unpack.go).
func multiValues(v *VM, env *Environment, stmt ast.Statement, values []exprCode, nameCount int) ([]Value, error) {
	line := stmt.Line()
	evalVals := make([]Value, 0, len(values))
	for _, d := range values {
		val, err := d(v, env)
		if err != nil {
			return nil, err
		}
		evalVals = append(evalVals, val)
	}
	if len(evalVals) == 1 {
		if t, ok := evalVals[0].(*TupleVal); ok {
			if len(t.Elements) != nameCount {
				kind, example := ast.UnpackSuggestion(stmt, len(t.Elements))
				return nil, messages.At(line, 0, messages.New(messages.H2709, len(t.Elements), nameCount, kind, example))
			}
			return t.Elements, nil
		}
	}
	if len(evalVals) == 1 && nameCount > 1 {
		if lv, ok := evalVals[0].(*ListVal); ok {
			if len(lv.Elements) != nameCount {
				return nil, messages.At(line, 0, messages.New(messages.H2700, nameCount, len(lv.Elements)))
			}
			evalVals = lv.Elements
		}
	}
	if len(evalVals) != nameCount {
		return nil, messages.At(line, 0, messages.New(messages.H2701, nameCount, len(evalVals)))
	}
	return evalVals, nil
}

// structDef 'yapı' deyimini derler: metotlar ve alan varsayılanları bir kez derlenir.
func (c *compiler) structDef(k *compilerScope, s *ast.StructStatement) statementCode {
	if s.Kind != ast.KindStruct { // 7.7, 7.8: enum_ops.go
		return c.typeDef(k, s)
	}
	type methodCode struct {
		name    string
		lit     *ast.FunctionLiteral
		codeVal *funcCode
	}
	methodList := make([]methodCode, 0, len(s.Methods))
	for name, fnLit := range s.Methods {
		methodList = append(methodList, methodCode{name: name, lit: fnLit, codeVal: c.function(k, fnLit.Parameters, fnLit.Body, true)})
	}
	// Alan varsayılanları yapının tanımlandığı kapsamda derlenir ve o ortamda
	// değerlendirilir (modüldeki bir yapı modülün adlarını görür).
	fieldCode := c.fieldDefaults(k, s.Fields)
	definition := identifier(k, s.Name)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		methods := make(map[string]*FuncVal, len(methodList))
		for _, m := range methodList {
			methods[m.name] = &FuncVal{
				Name:       m.name,
				Parameters: m.lit.Parameters,
				Body:       m.lit.Body,
				Env:        env,
				codeVal:    m.codeVal,
			}
		}
		def := &StructDefVal{
			Name:      s.Name,
			Fields:    s.Fields,
			Methods:   methods,
			fieldCode: fieldCode,
			fieldEnv:  env,
		}
		definition(env, def, true)
		return def, signalNone, nil
	}
}

// fieldDefaults alan varsayılanlarını k kapsamında derler (nesne oluşturulurken
// yapının tanımlandığı ortamda değerlendirilirler).
func (c *compiler) fieldDefaults(k *compilerScope, fields []ast.StructField) []exprCode {
	codeList := make([]exprCode, len(fields))
	for i, f := range fields {
		if f.DefaultValue != nil {
			codeList[i] = c.expression(k, f.DefaultValue)
		}
	}
	return codeList
}

// concurrent 'eşzamanlı' bloğunu derler: gövde, kendi kök ortamında ayrı bir görevde çalışır.
func (c *compiler) concurrent(k *compilerScope, s *ast.AsyncStatement) statementCode {
	sub := subScope(k)
	var bodyBlock *blockCode
	var call exprCode
	if s.Body != nil {
		sub.predefine(s.Body.Statements)
		bodyBlock = c.statements(sub, s.Body.Statements)
	} else if s.Call != nil {
		ast.ExprDefinedNames(s.Call, sub.define)
		call = c.expression(sub, s.Call)
	}
	info := sub.info
	line := s.Line()
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		v.wg.Add(1)
		asyncEnv := newEnv(env, info)
		asyncEnv.IsFuncRoot = true // görev içindeki 'ertele' blokları görev bitince çalışır
		task := v.NewTaskVM()
		go func(e *Environment) {
			defer v.wg.Done()
			task.acquireGIL()
			defer task.releaseGIL()
			defer func() {
				if r := recover(); r != nil && !IsInterrupt(r) {
					task.reportAsyncError(line, messages.Internal(fmt.Sprint(r)))
				}
			}()
			var err error
			defer func() {
				// Görevin ertelenen işleri görev bitince çalışır; hataları görev hatasına eklenir.
				// Durdurma ya da TR_exit yüzünden bitmek bir görev hatası değildir.
				if err = taskError(addDeferError(err, task.runDefers(e))); err != nil {
					task.reportAsyncError(line, err)
				}
			}()
			if bodyBlock != nil {
				_, _, err = bodyBlock.run(task, e)
			} else if call != nil {
				_, err = call(task, e)
			}
		}(asyncEnv)
		return True, signalNone, nil
	}
}
