package debugger

import (
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"path/filepath"
	"strconv"
)

// ---------------------------------------------------------------------------
// Duraklama: VM goroutine'i burada bekler ve gelen istekleri işler
// ---------------------------------------------------------------------------

func (s *dapServer) waitForCommand(v *interp.VM, line int, env *interp.Environment) int {
	d := s.dbg
	d.mu.Lock()
	reason, kind := d.PauseReason, d.pauseKind
	d.mu.Unlock()
	if kind == "" {
		kind = "step"
	}
	// Durum metinleri terminal ve düzenleyicide aynıdır (tuşlar da aynı anlamdadır).
	if reason == "" && kind == "step" {
		reason = durumAdim
	}

	s.mu.Lock()
	s.paused = true
	s.curVM, s.curEnv, s.curLine, s.curReason = v, env, line, reason
	s.handles = make(map[int]*dapHandle)
	s.nextHandle = 0
	s.mu.Unlock()

	s.event("stopped", map[string]interface{}{
		"reason": kind, "description": reason, "text": reason, "threadId": 1, "allThreadsStopped": true,
	})

	for req := range s.cmds {
		jump, resume := s.handlePaused(req.msg)
		if resume {
			s.mu.Lock()
			s.paused = false
			s.mu.Unlock()
			close(req.done)
			return jump
		}
		close(req.done)
	}
	return 0
}

// resume adım komutunu uygular; duraklama nedeni bir sonraki duruş için temizlenir.
func (s *dapServer) resume(mode StepMode, target int, reason, kind string) {
	d := s.dbg
	d.mu.Lock()
	d.Mode = mode
	d.TargetDepth = target
	d.PauseReason = reason
	d.pauseKind = kind
	d.mu.Unlock()
}
func (s *dapServer) handlePaused(msg *dapMessage) (jump int, resume bool) {
	switch msg.Command {
	case "continue":
		s.resume(StepContinue, 0, "", "")
		s.respond(msg, map[string]interface{}{"allThreadsContinued": true})
		return 0, true
	case "next":
		s.resume(StepOver, s.curVM.StackDepth(), "", "")
		s.respond(msg, nil)
		return 0, true
	case "stepIn":
		s.resume(StepInto, 0, "", "")
		s.respond(msg, nil)
		return 0, true
	case "stepOut":
		s.resume(StepOut, s.curVM.StackDepth(), "", "")
		s.respond(msg, nil)
		return 0, true
	case "goto":
		var a struct {
			TargetID int `json:"targetId"`
		}
		_ = json.Unmarshal(msg.Arguments, &a)
		if a.TargetID < 1 {
			s.respondErr(msg, "Geçersiz hedef satır")
			return 0, false
		}
		hedef, ok := s.curVM.JumpTarget(a.TargetID)
		if !ok {
			s.respondErr(msg, "Satır %d şu an çalışan bloğun dışında; yalnızca bulunulan bloğun (%s, %s ya da %s gövdesinin) satırlarına zıplanabilir", a.TargetID, codes.Name(codes.A020), codes.Name(codes.A010), codes.Name(codes.A001))
			return 0, false
		}
		s.resume(StepInto, 0, "Satır "+turkish.NumberSuffix(int64(hedef), turkish.Dative)+" zıplandı", "goto")
		s.respond(msg, nil)
		return hedef, true
	case "gotoTargets":
		var a struct {
			Line int `json:"line"`
		}
		_ = json.Unmarshal(msg.Arguments, &a)
		hedefler := []interface{}{}
		if hedef, ok := s.curVM.JumpTarget(a.Line); ok {
			hedefler = append(hedefler, map[string]interface{}{"id": hedef, "label": "Satır " + strconv.Itoa(hedef), "line": hedef})
		}
		s.respond(msg, map[string]interface{}{"targets": hedefler})
	case "stackTrace":
		s.stackTrace(msg)
	case "scopes":
		s.scopes(msg)
	case "variables":
		s.variables(msg)
	case "setVariable":
		s.setVariable(msg)
	case "evaluate":
		s.evaluate(msg)
	case "dataBreakpointInfo":
		s.dataBreakpointInfo(msg)
	case "setDataBreakpoints":
		s.setDataBreakpoints(msg)
	case "trTablo":
		s.table(msg)
	case "trDegiskenler":
		s.variablesTable(msg)
	default:
		s.respondErr(msg, "Desteklenmeyen istek: %s", msg.Command)
	}
	return 0, false
}
func (s *dapServer) source() map[string]interface{} {
	yol := s.programPath
	if s.gorunumAdresi != "" {
		yol = s.gorunumAdresi
	}
	return map[string]interface{}{"name": filepath.Base(s.programPath), "path": yol}
}
func (s *dapServer) stackTrace(msg *dapMessage) {
	stack := s.curVM.StackSnapshot()
	frameName := func(i int) string {
		if i < 0 {
			return "ana program"
		}
		return stack[i].FuncName + "()"
	}
	frames := []interface{}{map[string]interface{}{
		"id": 1, "name": frameName(len(stack) - 1), "line": s.curLine, "column": 1, "source": s.source(),
	}}
	for i := len(stack) - 1; i >= 0; i-- {
		line := stack[i].Line
		if line < 1 {
			line = 1
		}
		frames = append(frames, map[string]interface{}{
			"id": len(stack) - i + 1, "name": frameName(i - 1), "line": line, "column": 1, "source": s.source(),
		})
	}
	s.respond(msg, map[string]interface{}{"stackFrames": frames, "totalFrames": len(frames)})
}

// evaluate: izleme (watch), üzerine gelme (hover) ve hata ayıklama konsolu. Konsolda deyimler
// de çalıştırılabilir (ör. 'bakiye = 0' ya da 'yazdır(liste)').
func (s *dapServer) evaluate(msg *dapMessage) {
	var a struct {
		Expression string `json:"expression"`
		Context    string `json:"context"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	p := parser.New(lexer.New(a.Expression))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		s.respondErr(msg, "%s", p.Errors()[0])
		return
	}
	var val interp.Value
	var err error
	if es, ok := singleExpression(prog); ok {
		val, err = s.quietly(func() (interp.Value, error) { return s.curVM.EvalExpression(es.Expression, s.curEnv) })
	} else if a.Context == "repl" {
		val, err = s.quietly(func() (interp.Value, error) {
			return s.curVM.ExecStatements(prog.Statements, s.curEnv)
		})
	} else {
		err = fmt.Errorf("yalnızca ifade değerlendirilebilir")
	}
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	if val == nil {
		val = interp.NULL
	}
	s.respond(msg, map[string]interface{}{
		"result": display(val), "type": string(val.Type()), "variablesReference": s.refFor(val, a.Expression),
	})
}
func singleExpression(prog *ast.Program) (*ast.ExpressionStatement, bool) {
	if len(prog.Statements) != 1 {
		return nil, false
	}
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	return es, ok
}
