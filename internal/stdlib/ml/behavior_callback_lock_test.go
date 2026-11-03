package ml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Kural (10.2, S-kilit, adım 4): paketin TR koduna geri çağrıları (ApplyFunction) yalnız
// applyFunction'dan geçer; o da çağrıyı yorumlayıcı kilidi tutularak yapar. Yeni bir doğrudan
// m.ApplyFunction çağrısı testi düşürür ve yerini söyler.
func TestCallbacksGoThroughApplyFunction(t *testing.T) {
	files, _ := reporoot.Glob("*.go")
	fset := token.NewFileSet()
	inHelper := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range tree.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ApplyFunction" {
					if fd.Name.Name == "applyFunction" {
						inHelper++
					} else {
						t.Errorf("%s: %s doğrudan ApplyFunction çağırıyor; applyFunction(m, ...) kullanın (kilit tutularak geri çağrı)",
							fset.Position(call.Pos()), fd.Name.Name)
					}
				}
				return true
			})
		}
	}
	if inHelper != 1 {
		t.Fatalf("applyFunction'da %d ApplyFunction çağrısı bulundu, 1 olmalı", inHelper)
	}
}

// Kural (10.2, S-kilit, adım 4): kilit bırakılmış bir bölgeden (Blocking) yapılan geri çağrı,
// TR kodu süresince kilidi geri alır ve sonra yeniden bırakır.
func TestCallbackFromAReleasedRegionHoldsTheLock(t *testing.T) {
	var inCallback, inRegion, after bool
	m := interp.New()
	m.Out = io.Discard
	m.GlobalEnv.Define("kilitte", &interp.BuiltinVal{Name: "kilitte", Fn: func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		inCallback = y.HoldsLock()
		return interp.Null, nil
	}}, true)
	m.GlobalEnv.Define("bırakıp_çağır", &interp.BuiltinVal{Name: "bırakıp_çağır", Fn: func(y *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		var err error
		y.Blocking(func() {
			_, err = applyFunction(y, args[0], nil, nil)
			inRegion = !y.HoldsLock()
		})
		after = y.HoldsLock()
		return interp.Null, err
	}}, true)
	_, err := m.Run(trrun.Parse(t, `TR_func g() { kilitte() }
TR_async { TR_sleep(0.001) }
bırakıp_çağır(g)
TR_wait_all()
`))
	m.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
	if !inCallback || !inRegion || !after {
		t.Fatalf("geri çağrıda kilit %v (doğru olmalı), çağrıdan sonra bölgede bırakılmış %v, bölgeden sonra kilit %v", inCallback, inRegion, after)
	}
}

// Kural (10.2, S-kilit, adım 4; kilitlenmesizlik): geri çağrı kilidi tutmayan bir görünümde koşabilir
// ama goroutine kilidi başka bir görünümle tutuyor olabilir. Go'dan bağlamsız başlatılan geri
// yayılımda özel türevin geri hesabı ileri hesabı yapan görevin (bitmiş) görünümüyle çağrılır.
// Locked yalnız o görünümün kendi Blocking'inde kilidi geri alır; görünümün kilidi tutmamasına
// bakıp almaya kalksaydı goroutine kendini beklerdi. Test 5 sn'de bitmezse kalır, asılı kalmaz.
func TestCallbackOnAnotherViewDoesNotDeadlock(t *testing.T) {
	m := interp.New()
	var out strings.Builder
	m.Out = &out
	m.GlobalEnv.Define("goda_geri", &interp.BuiltinVal{Name: "goda_geri", Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		args[0].(*TensorValue).t.Backward() // bağlamsız: geri hesap ileri görevin görünümüyle
		return interp.Null, nil
	}}, true)
	prog := trrun.Parse(t, `TR_var kare = TR_custom_grad(
    TR_forward: (x) => { TR_return [x * x, [x]] },
    TR_backward: (saklananlar, g) => { TR_return [g * 2 * saklananlar[0]] }
)
TR_var x = TR_Tensor([3.0], TR_requires_grad: TR_true)
TR_var y = TR_none
TR_async { y = TR_sum(kare(x)) }
TR_wait_all()
goda_geri(y)
TR_print(x.TR_grad.TR_tolist)
`)
	done := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		m.WaitAsync()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("başka görünümdeki geri çağrı 5 sn'de bitmedi: kilitlenme")
	}
	if out.String() != "[6.0]\n" {
		t.Fatalf("gradyan %q, [6.0] olmalı", out.String())
	}
}
