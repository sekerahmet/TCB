package interp

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (B2b): Liste ve Metin'de .TR_slice yoktur (Tensör'ün metodu); ileti dilimin köşeli
// parantezle alındığını söyler (H2526).
func TestSliceMethodOnListGetsRangeHint(t *testing.T) {
	for _, code := range []string{"[1, 2, 3].TR_slice(0, 1)", `"abc".TR_slice(0, 1)`} {
		_, err := New().Run(mustParseProgram(t, code+"\n"))
		if !messages.Has(err, messages.H2526) {
			t.Errorf("%s: %v", code, err)
		}
	}
}

// Kural (13.2): büyük_harf ve küçük_harf herkes için Unicode'un varsayılan harf dönüşümüdür
// (Python str.upper, Swift uppercased(), Go strings.ToUpper gibi); dosyanın dili yoktur, kişinin dili
// sonucu değiştirmez ("title" → "TITLE", "istanbul" → "ISTANBUL", "LIST" → "list").
func TestCaseConversionIsTheSameInEveryLanguage(t *testing.T) {
	want := "TITLE ISTANBUL list\n"
	if got := runCoded(t, `TR_print("title".TR_upper(), "istanbul".TR_upper(), "LIST".TR_lower())`+"\n", nil); got != want {
		t.Errorf("%q, beklenen %q", got, want)
	}
}

// Kural (10.2): metnin metotları değişmez girdiyi okuyup yeni değer kurar; büyük metinde
// (longTextBytes) kilidi bırakır, küçük metinde bırakmaz (bırakmanın bedeli işten büyük olurdu).
// Kilidin kaç kez alındığı görev kancasıyla sayılır. Eskiden hep tutuyordu.
func TestLongTextMethodsReleaseTheLock(t *testing.T) {
	takes := func(code string) int {
		machine := New()
		machine.Out = io.Discard
		n := 0
		machine.AddTaskHook(func(*VM) { n++ })
		if _, err := machine.Run(mustParseProgram(t, code)); err != nil {
			t.Fatalf("%v\n%s", err, code)
		}
		return n
	}
	big := fmt.Sprintf("TR_var s = \"\"\nTR_for i TR_in 0..%d {\n    s += \"aa \"\n}\n", longTextBytes/3+1)
	small := "TR_var s = \"aa aa\"\n"
	base := takes(big)
	for _, call := range []string{"s.TR_upper()", "s.TR_lower()", "s.TR_split()", `s.TR_replace("a", "b")`, `s.TR_contains("b")`, "s.TR_get(-1)", "s.TR_reverse()"} {
		if got := takes(big+"TR_var r = "+call+"\n") - base; got != 1 {
			t.Errorf("büyük metinde %s kilidi %d kez bıraktı, 1 beklenirdi", call, got)
		}
		if got := takes(small+"TR_var r = "+call+"\n") - takes(small); got != 0 {
			t.Errorf("küçük metinde %s kilidi %d kez bıraktı, 0 beklenirdi", call, got)
		}
	}
}

// Kural (13.10): .TR_append değer döndürmez; listeyi yerinde değiştirir. Tipi bilinen yazımda dönüşün
// kullanılması H1143'tür (TestNoValueCallInAValuePlace); burada tipi bilinmeyen alıcının motordaki yolu
// sınanır: tek argümanlı hızlı yol ve çok argümanlı yöntem aynı sonucu verir.
func TestAppendGivesNoValue(t *testing.T) {
	for call, want := range map[string]string{"l.TR_append(2)": "[none, 2]", "l.TR_append(2, 3)": "[none, 3]"} {
		_, val, err := runTRWithin(t, 5*time.Second, "TR_var l = [1]\nTR_var result = ["+call+", l.TR_length]")
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if val.Inspect() != want {
			t.Errorf("%s: %s bekleniyordu, %s alındı", call, want, val.Inspect())
		}
	}
}
