package process

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// program_çalıştır (TR_fn_run_process, bölüm 21): dış programı çalıştırır, bitmesini bekler, standart
// çıktısını bayt olarak verir. Kabuk yoktur: her liste öğesi programa tek argüman olarak, değiştirilmeden
// gider (21.1, 21.8). Bekleme kilidi bırakır ve süren bir işi bitirir (10.3, 1.6): interp.AwaitExternal.

// Adlı argümanlar; adları kod listesindedir.
var (
	nameInput     = bind.NamedArg(codes.ArgInput)
	nameEnv       = bind.NamedArg(codes.ArgEnv)
	nameTimeout   = bind.NamedArg(codes.ArgTimeout)
	nameExitCodes = bind.NamedArg(codes.ArgExitCodes)
)

// runCall bir program_çalıştır çağrısının okunmuş argümanlarıdır.
type runCall struct {
	name  string // iletilerde programın adı: verilen ad ya da yol
	path  string // çalışacak dosya
	args  []string
	input *interp.BytesVal // yoksa girdi boş aygıttır (21.1)
}

// runProcess program_çalıştır(program, argümanlar) yerleşiğidir.
func runProcess(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnRunProcess)
	if err := bind.CheckNamed(fn, named, nameInput, nameEnv, nameTimeout, nameExitCodes); err != nil {
		return nil, err
	}
	call, err := readRunCall(fn, args, named)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	// exec.Command kullanılmaz: yolu yeniden arar (21.3, program_lookup.go).
	c := &exec.Cmd{Path: call.path, Args: append([]string{call.path}, call.args...)}
	if call.input != nil {
		c.Stdin = bytes.NewReader(call.input.Data())
	}
	c.Stdout = &out
	if err := c.Start(); err != nil {
		return nil, messages.New(messages.H7501, fn, call.name, err)
	}
	var waitErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		waitErr = c.Wait()
	}()
	if err := m.AwaitExternal(done, func() { _ = c.Process.Kill() }); err != nil {
		return nil, err
	}
	if waitErr != nil {
		return nil, messages.New(messages.H7501, fn, call.name, waitErr)
	}
	return interp.NewBytes(out.Bytes()), nil
}

// readRunCall çağrının argümanlarını okur ve denetler; program başlamadan biter.
func readRunCall(fn any, args []interp.Value, named map[string]interp.Value) (*runCall, error) {
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.FnRunProcess, 2, len(args))
	}
	call := &runCall{}
	var err error
	if call.name, call.path, err = findProgram(fn, args[0]); err != nil {
		return nil, err
	}
	if call.args, err = programArguments(fn, args[1]); err != nil {
		return nil, err
	}
	if v, ok := bind.GetNamed(named, nameInput); ok {
		b, ok := v.(*interp.BytesVal)
		if !ok {
			return nil, messages.New(messages.H7493, fn, nameInput, v.Type())
		}
		call.input = b
	}
	return call, nil
}

// programArguments argüman listesini okur. Listede yol olması denetim hatasıdır (21.1); tipi
// bilinmeyen listede aynı kod çalışma anında gelir.
func programArguments(fn any, v interp.Value) ([]string, error) {
	list, ok := v.(*interp.ListVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnRunProcess, 2, v, codes.TypeList)
	}
	out := make([]string, len(list.Elements))
	for i, e := range list.Elements {
		switch x := e.(type) {
		case *interp.StringVal:
			if strings.IndexByte(x.Val, 0) >= 0 {
				return nil, messages.New(messages.H7492, fn, i+1)
			}
			out[i] = x.Val
		case interp.PathValuer:
			return nil, messages.New(messages.H7490, fn, i+1, codes.Spelling(codes.TypePath)+"("+strconv.Quote(x.FilePath())+")")
		default:
			return nil, messages.New(messages.H7491, fn, i+1, e.Type())
		}
	}
	return out, nil
}
