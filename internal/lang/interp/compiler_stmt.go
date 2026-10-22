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
	values := c.expressions(k, s.Values)
	definitions := make([]func(*Environment, Value, bool), len(s.Names))
	for i, n := range s.Names {
		definitions[i] = identifier(k, n.Value)
	}
	constant := s.IsConst
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return applyMultiValues(v, env, values, len(definitions), func(i int, val Value) error {
			definitions[i](env, val, constant)
			return nil
		})
	}
}

// assignStatement atamayı (bileşik atama dahil) derler.
func (c *compiler) assignStatement(k *compilerScope, s *ast.AssignStatement) statementCode {
	value := c.expression(k, s.Value)
	dest := c.assignTarget(k, s.Target, opCode(s.Operator), s.Line())
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
	values := c.expressions(k, s.Values)
	dests := make([]func(*VM, *Environment, Value) error, len(s.Targets))
	for i, t := range s.Targets {
		dests[i] = c.assignTarget(k, t, opAssign, s.Line())
	}
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		return applyMultiValues(v, env, values, len(dests), func(i int, val Value) error {
			return dests[i](v, env, val)
		})
	}
}

// applyMultiValues çoklu tanım/atamanın sağ tarafını hesaplar ve her değeri sırayla apply ile
// i. hedefe uygular (eksik değer Boş); sonuç son değerdir.
func applyMultiValues(v *VM, env *Environment, values []exprCode, count int, apply func(i int, val Value) error) (Value, flowSignal, error) {
	evalVals, err := multiValues(v, env, values, count)
	if err != nil {
		return nil, signalNone, err
	}
	var lastVal Value = Null
	for i := 0; i < count; i++ {
		var val Value = Null
		if i < len(evalVals) {
			val = evalVals[i]
		}
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
				_, _, err := bodyBlock.run(v, newEnv(capturedEnv, bodyInfo))
				return err
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
	value := c.expression(k, s.ReturnValue)
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
	catchClause := c.statements(sub, catchStatements)
	info := sub.info
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, sig, err := tryBlock(v, env)
		if err != nil {
			if uncatchable(err) {
				return nil, signalNone, err // kesilme (Ctrl+C) ve program çıkışı isteği yakalanmaz
			}
			catchEnv := newEnv(env, info)
			catchEnv.define(errSlot, &StringVal{Val: err.Error()}, false)
			return catchClause.run(v, catchEnv)
		}
		return val, sig, nil
	}
}

// throwStatement 'fırlat' deyimini derler.
func (c *compiler) throwStatement(k *compilerScope, s *ast.ThrowStatement) statementCode {
	value := c.expression(k, s.Value)
	return func(v *VM, env *Environment) (Value, flowSignal, error) {
		val, err := value(v, env)
		if err != nil {
			return nil, signalNone, err
		}
		return nil, signalNone, messages.New(messages.H2441, val.Inspect())
	}
}

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
			if !condVal.IsTruthy() {
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
				v.Debugger.Breakpoint(messages.Text(messages.H2520, codes.DisplayName(codes.A090)))
			}
		}
		return Null, signalNone, nil
	}
}

// multiValues çoklu tanım/atamanın sağ tarafını hesaplar; tek bir Liste birden çok ada
// açılır.
func multiValues(v *VM, env *Environment, values []exprCode, nameCount int) ([]Value, error) {
	evalVals := make([]Value, 0, len(values))
	for _, d := range values {
		val, err := d(v, env)
		if err != nil {
			return nil, err
		}
		evalVals = append(evalVals, val)
	}
	if len(evalVals) == 1 && nameCount > 1 {
		if lv, ok := evalVals[0].(*ListVal); ok {
			evalVals = lv.Elements
		}
	}
	return evalVals, nil
}

// structDef 'yapı' deyimini derler: metotlar ve alan varsayılanları bir kez derlenir.
func (c *compiler) structDef(k *compilerScope, s *ast.StructStatement) statementCode {
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
		exprNames(s.Call, sub.define)
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
				if err = addDeferError(err, e.RunDefers()); err != nil {
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
