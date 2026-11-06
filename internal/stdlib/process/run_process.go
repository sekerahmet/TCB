package process

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

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

// stopGrace programın nazik istekten sonra durmak için aldığı süredir (21.5); program bittikten sonra
// boruyu tutan torun da bu kadar beklenir.
const stopGrace = 2 * time.Second

// runCall bir program_çalıştır çağrısının okunmuş argümanlarıdır.
type runCall struct {
	name  string // iletilerde programın adı: verilen ad ya da yol
	path  string // çalışacak dosya
	args  []string
	input *interp.BytesVal // yoksa girdi boş aygıttır (21.1)
	codes []int            // kabul edilen çıkış kodları (21.4)
	env   []string         // programın ortamı (21.6)
	// filtered ortamdan süzülen gizli adlardır; başarısız bitişin iletisi söyler (21.6).
	filtered []string
	limit    interp.Value  // TR_timeout: değeri; yoksa nil (21.5)
	wait     time.Duration // limit'in süresi
}

// runProcess program_çalıştır(program, argümanlar) yerleşiğidir.
func runProcess(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnRunProcess)
	line := m.BuiltinLine()
	if err := bind.CheckNamed(fn, named, nameInput, nameEnv, nameTimeout, nameExitCodes); err != nil {
		return nil, err
	}
	call, err := readRunCall(m, fn, args, named)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	// exec.Command kullanılmaz: yolu yeniden arar (21.3, program_lookup.go).
	c := &exec.Cmd{Path: call.path, Args: append([]string{call.path}, call.args...), Env: call.env}
	if call.input != nil {
		c.Stdin = bytes.NewReader(call.input.Data())
	}
	c.Stdout = &out
	tail := &stderrTail{longest: m.LongestSecret}
	c.Stderr = tail // ayrı okunur: iki çıktı birbirini kilitlemez (21.4)
	c.WaitDelay = stopGrace
	j, err := startJob(c)
	if err != nil {
		return nil, messages.New(messages.H7501, fn, call.name, err)
	}
	defer j.release()
	if call.limit != nil {
		timer := time.AfterFunc(call.wait, func() { j.stopFor(&j.timedOut) })
		defer timer.Stop()
	}
	if err := m.AwaitExternal(j.done, j.stop); err != nil {
		return nil, err
	}
	if j.timedOut.Load() {
		return nil, messages.New(messages.H2921, fn, interp.Shown(call.limit), line)
	}
	// Boruyu tutan torun programın sonucunu değiştirmez: okunan çıktı verilir, sonucu çıkış kodu
	// belirler (exec.ErrWaitDelay).
	if c.ProcessState == nil {
		return nil, messages.New(messages.H7501, fn, call.name, j.waitErr)
	}
	if err := call.endError(fn, c.ProcessState, tail.text(m.Mask)); err != nil {
		return nil, err
	}
	return interp.NewBytes(out.Bytes()), nil
}

// endError biten programın hatasıdır: sinyalle ölmek ya da kabul edilmeyen kod (21.4).
func (r *runCall) endError(fn any, state *os.ProcessState, stderr string) error {
	if signal, killed := exitSignal(state); killed {
		return messages.New(messages.H7504, fn, r.name, signal, stderr, filteredHint(r.filtered))
	}
	code := state.ExitCode()
	if slices.Contains(r.codes, code) {
		return nil
	}
	accepted := make([]string, 0, len(r.codes)+1)
	for _, c := range append(slices.Clone(r.codes), code) {
		accepted = append(accepted, strconv.Itoa(c))
	}
	return messages.New(messages.H7503, fn, r.name, code, "["+strings.Join(accepted, ", ")+"]", stderr, filteredHint(r.filtered))
}

// exitCodes TR_exit_codes: argümanını okur; verilmezse yalnız 0 kabul edilir (21.4). Boş liste hiçbir
// bitişi kabul etmez: yazım hatasıdır, program başlamaz.
func exitCodes(fn any, named map[string]interp.Value) ([]int, error) {
	v, ok := bind.GetNamed(named, nameExitCodes)
	if !ok {
		return []int{0}, nil
	}
	list, ok := v.(*interp.ListVal)
	if !ok {
		return nil, messages.New(messages.H7506, fn, nameExitCodes, v.Type())
	}
	if len(list.Elements) == 0 {
		return nil, messages.New(messages.H7508, fn)
	}
	out := make([]int, len(list.Elements))
	for i, e := range list.Elements {
		n, ok := bind.Int(e)
		if !ok {
			return nil, messages.New(messages.H7506, fn, nameExitCodes, e.Type())
		}
		out[i] = int(n)
	}
	return out, nil
}

// readRunCall çağrının argümanlarını okur ve denetler; program başlamadan biter.
func readRunCall(m *interp.VM, fn any, args []interp.Value, named map[string]interp.Value) (*runCall, error) {
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
	if call.codes, err = exitCodes(fn, named); err != nil {
		return nil, err
	}
	if call.env, call.filtered, err = programEnv(m, fn, named); err != nil {
		return nil, err
	}
	if v, ok := bind.GetNamed(named, nameTimeout); ok {
		if call.wait, err = interp.Seconds(codes.FnRunProcess, v); err != nil {
			return nil, err
		}
		call.limit = v
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
