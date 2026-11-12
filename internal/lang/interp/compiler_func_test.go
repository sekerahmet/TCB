package interp

import (
	"bytes"
	"runtime"
	"testing"
	"weak"
)

// Kural (15.43, 10.9): iptal edilen görevin yarıda kalan çağrısının ortamı görev durunca hiçbir yoldan
// tutulmaz. Havuzlanan bir fonksiyonun ortamı havuzdan alınırken havuzun dilimindeki yuvası boşaltılır:
// iptal yığını çözdüğünde ortam havuza dönmez, dilimin eski yuvası da onu (ve yerellerinin gösterdiğini)
// bir sonraki çağrıya kadar tutmaz. Üç çağrı yolu: fonksiyon (callAt), metot (callMethod), yerleşiğin
// çalıştırdığı blok (apply). Her yolda fonksiyon önce bir kez normal çağrılır (ortamı havuza girer),
// sonra görevde çağrılıp beklerken iptal edilir.
func TestACancelledCallLeavesNoEnvironmentInThePool(t *testing.T) {
	cases := map[string]string{
		"fonksiyon": `TR_func tut(x: TR_List[TR_Int]?, s: TR_Float) -> TR_Int {
    TR_sleep(s)
    TR_return 0
}
tut(TR_none, 0)
TR_var g = TR_async tut(büyük, 10)`,
		"metot": `TR_struct Kutu {
    TR_func tut(x: TR_List[TR_Int]?, s: TR_Float) -> TR_Int {
        TR_sleep(s)
        TR_return 0
    }
}
TR_var k = Kutu()
k.tut(TR_none, 0)
TR_var g = TR_async k.tut(büyük, 10)`,
		"blok": `TR_func tut(x: TR_List[TR_Int]?, s: TR_Float) -> TR_String {
    TR_return TR_error_code { TR_sleep(s) }
}
tut(TR_none, 0)
TR_var g = TR_async tut(büyük, 10)`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			source := `TR_var büyük: TR_List[TR_Int]? = [1, 2, 3]
izle(büyük)
` + body + `
TR_sleep(0.05)
g.TR_cancel()
TR_print(TR_error_code { g.TR_wait() })
büyük = TR_none
`
			var watched weak.Pointer[ListVal]
			var out, failure bytes.Buffer
			machine := New()
			machine.Out, machine.Err = &out, &failure
			machine.RegisterBuiltin("izle", func(y *VM, args []Value, named map[string]Value) (Value, error) {
				watched = weak.Make(args[0].(*ListVal))
				return Null, nil
			})
			if _, err := machine.Run(mustParseProgram(t, source)); err != nil || out.String() != "H2920\n" || failure.Len() > 0 {
				t.Fatalf("program: %v; çıktı %q, hata çıktısı %q", err, out.String(), failure.String())
			}
			runtime.GC()
			runtime.GC()
			if watched.Value() != nil {
				t.Fatal("iptal edilen çağrının argümanı programdan ulaşılamazken hâlâ tutuluyor (havuzun dilimindeki ortam)")
			}
			runtime.KeepAlive(machine)
		})
	}
}
