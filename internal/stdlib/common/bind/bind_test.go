package bind

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

func names(codeList ...codes.Code) []Named {
	var a []Named
	for _, k := range codeList {
		a = append(a, NamedArg(k))
	}
	return a
}

// NamedArg yalnızca adlı argüman kodlarını (x9xx) kabul eder; yanlış kod başlangıçta panikler.
func TestNamedArg(t *testing.T) {
	a := NamedArg(codes.ArgLr)
	if a.TR != codes.Name(codes.ArgLr) || !a.Accepts(a.TR) || a.Spellings()[0] != a.TR {
		t.Fatalf("NamedArg(T910) = %+v", a)
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "G100") {
			t.Fatalf("adlı argüman olmayan kod paniklemeli: %v", r)
		}
	}()
	NamedArg(codes.FnPrint)
}

// CheckNamed bilinmeyen adda öneriyi şu sırayla arar: (1) kod listesindeki öneri (ASCII
// yazım), yalnızca bu fonksiyonun adlarındansa; (2) başka dillerden alışılmış ad
// (büyük-küçük harf duyarsız), yine yalnızca bu fonksiyonun adlarındansa; (3) yazım
// yakınlığı. Öneri yoksa geçerli adlar listelenir; fonksiyon adlı argüman almıyorsa söylenir.
func TestCheckNamedSuggestionOrder(t *testing.T) {
	lrName, axisName, epsilon := codes.Name(codes.ArgLr), codes.Name(codes.ArgDim), codes.Name(codes.ArgEps)
	cases := []struct {
		name       string
		given      string
		names      []Named
		suggestion string   // boşsa öneri yok
		contains   []string // mesajda geçmesi gerekenler (öneri yoksa)
		excludes   []string
	}{
		{name: "bilinen ad", given: lrName, names: names(codes.ArgLr, codes.ArgWeightDecay)},
		{name: "ASCII yazım", given: "ogrenme_orani", names: names(codes.ArgLr, codes.ArgWeightDecay), suggestion: lrName},
		{name: "yabancı ad", given: "lr", names: names(codes.ArgLr, codes.ArgWeightDecay), suggestion: lrName},
		{name: "yabancı ad büyük harfle", given: "LR", names: names(codes.ArgLr), suggestion: lrName},
		{name: "yabancı ad yazım yakınlığından önce", given: "eps", names: names(codes.ArgEps, codes.ArgDim), suggestion: epsilon},
		{name: "yazım hatası", given: "egsen", names: names(codes.ArgDim, codes.ArgSeed), suggestion: axisName},
		{name: "yabancı adın karşılığı bu fonksiyonda yok", given: "lr", names: names(codes.ArgDim),
			contains: []string{axisName}, excludes: []string{lrName}},
		{name: "adlı argüman almayan fonksiyon", given: "eksen", names: nil, contains: []string{"almaz"}},
	}
	for _, d := range cases {
		t.Run(d.name, func(t *testing.T) {
			value := map[string]interp.Value{d.given: &interp.IntVal{Val: 1}}
			err := CheckNamed("deneme", value, d.names...)
			if d.suggestion == "" && d.contains == nil {
				if err != nil {
					t.Fatalf("bilinen ad reddedildi: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%q kabul edildi; hata bekleniyordu", d.given)
			}
			m := err.Error()
			if !strings.Contains(m, "deneme") || !strings.Contains(m, "'"+d.given+"'") {
				t.Errorf("mesaj fonksiyonu ve verilen adı söylemeli: %s", m)
			}
			if d.suggestion != "" && !strings.Contains(m, "'"+d.suggestion+"'?") {
				t.Errorf("öneri %q bekleniyordu: %s", d.suggestion, m)
			}
			if d.suggestion == "" && strings.Contains(m, "?") {
				t.Errorf("öneri beklenmiyordu: %s", m)
			}
			for _, p := range d.contains {
				if !strings.Contains(m, p) {
					t.Errorf("mesajda %q yok: %s", p, m)
				}
			}
			for _, p := range d.excludes {
				if strings.Contains(m, p) {
					t.Errorf("mesajda %q olmamalı: %s", p, m)
				}
			}
		})
	}
}

func TestGetNamed(t *testing.T) {
	a := NamedArg(codes.ArgDim)
	v := &interp.IntVal{Val: 3}
	if g, ok := GetNamed(map[string]interp.Value{a.TR: v}, a); !ok || g != v {
		t.Fatalf("GetNamed = %v, %v", g, ok)
	}
	if _, ok := GetNamed(map[string]interp.Value{"başka": v}, a); ok {
		t.Fatal("verilmeyen adlı argüman bulundu")
	}
}

func TestIntAndBool(t *testing.T) {
	for _, d := range []struct {
		v  interp.Value
		n  int64
		ok bool
	}{
		{&interp.IntVal{Val: -7}, -7, true},
		{&interp.FloatVal{Val: 4}, 4, true},
		{&interp.FloatVal{Val: 4.5}, 0, false},
		{&interp.StringVal{Val: "4"}, 0, false},
	} {
		if n, ok := Int(d.v); n != d.n || ok != d.ok {
			t.Errorf("TamSayi(%s) = %d, %v; %d, %v olmalı", d.v.Inspect(), n, ok, d.n, d.ok)
		}
	}
	if b, err := Bool("f", "x", &interp.BoolVal{Val: true}); err != nil || !b {
		t.Errorf("Mantik(doğru) = %v, %v", b, err)
	}
	if _, err := Bool("f", "x", &interp.IntVal{Val: 1}); err == nil || !strings.Contains(err.Error(), "f: x") {
		t.Errorf("Mantik(1) hata vermeli: %v", err)
	}
}

// Kurtar ve Guvenli: Go paniği programı çökertmez, TR'nin yakalayabildiği bir hataya döner.
func TestPanicBecomesError(t *testing.T) {
	convert := func(r any) error { return fmt.Errorf("çevrildi: %v", r) }
	if s, err := Recover(func() int { return 5 }, convert); s != 5 || err != nil {
		t.Errorf("paniksiz Kurtar = %d, %v", s, err)
	}
	if s, err := Recover(func() int { panic("dizin taştı") }, convert); s != 0 || err == nil || err.Error() != "çevrildi: dizin taştı" {
		t.Errorf("panikli Kurtar = %d, %v", s, err)
	}

	want := errors.New("sıradan hata")
	if _, err := Safe("fn", func() (interp.Value, error) { return nil, want }); err != want {
		t.Errorf("Guvenli fonksiyonun hatasını olduğu gibi döndürmeli: %v", err)
	}
	if v, err := Safe("fn", func() (interp.Value, error) { return &interp.IntVal{Val: 2}, nil }); err != nil || v.Inspect() != "2" {
		t.Errorf("Guvenli = %v, %v", v, err)
	}
	_, err := Safe("fn", func() (interp.Value, error) {
		var s []int
		_ = s[3]
		return nil, nil
	})
	if !messages.Has(err, messages.H9000) || !strings.HasPrefix(err.Error(), "fn: ") {
		t.Errorf("Guvenli paniği iç hataya çevirmeli: %v", err)
	}
}

// Kesilme (Ctrl+C) bir hata değildir: Kurtar onu yutmaz, yığın çözülmeye devam eder ve
// görev kesilmiş olarak biter.
func TestRecoverKeepsInterrupt(t *testing.T) {
	m := interp.New()
	converted := false
	m.GlobalEnv.Define("dene_kurtar", &interp.BuiltinVal{Name: "dene_kurtar", Fn: func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		y.Interrupt()
		_, _ = Recover(func() int {
			y.Blocking(func() {}) // kilit geri alınınca kesilme paniği atılır
			return 1
		}, func(any) error { converted = true; return errors.New("yutuldu") })
		return interp.Null, nil
	}}, true)
	_, err := m.Run(parser.New(lexer.New("dene_kurtar()")).ParseProgram())
	if converted {
		t.Fatal("Kurtar kesilmeyi hataya çevirdi")
	}
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("görev kesilmiş olarak bitmeli: %v", err)
	}
}

// ResolvePath göreli yolu önce çalışma klasörüne, sonra programın klasörüne göre çözer; hiçbir
// yerde yoksa çalışma klasörüne göre yol döner (hata mesajı kullanıcının yazdığını göstersin).
func TestResolvePath(t *testing.T) {
	work, program := t.TempDir(), t.TempDir()
	t.Chdir(work)
	text := func(dir, name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text(work, "ikisinde.txt")
	text(program, "ikisinde.txt")
	text(program, "yalnız_programda.txt")
	m := interp.New()
	m.SourceFile = filepath.Join(program, "betik.tr")

	if g := ResolvePath(m, "ikisinde.txt"); g != "ikisinde.txt" {
		t.Errorf("çalışma klasörü önce gelmeli: %q", g)
	}
	if g := ResolvePath(m, "yalnız_programda.txt"); g != filepath.Join(program, "yalnız_programda.txt") {
		t.Errorf("programın klasörüne bakılmalı: %q", g)
	}
	g, tried := ResolvePathTried(m, "yok.txt")
	if g != "yok.txt" || len(tried) != 2 {
		t.Errorf("olmayan dosya çalışma klasörüne göre dönmeli ve iki yere bakılmalı: %q %v", g, tried)
	}
	if g := ResolvePath(nil, "yalnız_programda.txt"); g != "yalnız_programda.txt" {
		t.Errorf("VM yoksa yalnızca çalışma klasörüne bakılır: %q", g)
	}
	abs := filepath.Join(program, "yalnız_programda.txt")
	if g, b := ResolvePathTried(m, abs); g != abs || b != nil {
		t.Errorf("mutlak yol olduğu gibi kalmalı: %q %v", g, b)
	}

	_, err := os.Open(ResolvePath(m, "yok.txt"))
	if !IsNotExist(err) || !IsNotExist(fmt.Errorf("sarılı: %w", err)) || IsNotExist(fs.ErrPermission) {
		t.Errorf("YokMu yalnızca 'dosya yok' hatasını tanımalı: %v", err)
	}
}

// Kural (4D): kısa sayı ileti parametresidir; basamak sözcüğü (bin, thousand) ve ondalık ayırıcı
// iletinin dilindedir. Bayt miktarının kuralı tek yerdedir: tensor.ByteSize (gpu_test).
func TestShortNumber(t *testing.T) {
	for n, b := range map[int64][2]string{
		0: {"0", "0"}, 999: {"999", "999"}, 1000: {"1 bin", "1 thousand"}, 12_345: {"12,3 bin", "12.3 thousand"},
		2_500_000: {"2,5 milyon", "2.5 million"}, 56_900_000_000: {"56,9 milyar", "56.9 billion"},
		3_000_000_000_000: {"3 trilyon", "3 trillion"},
	} {
		if tr, en := ShortNumber(n).MessageText("tr"), ShortNumber(n).MessageText("en"); tr != b[0] || en != b[1] {
			t.Errorf("ShortNumber(%d) = %q / %q; %q / %q olmalı", n, tr, en, b[0], b[1])
		}
	}
}

// Kural: CodeMethod yöntemi Go paniğine karşı korur (panik, adlı bir H2000 hatasına döner);
// değerin adını kodun kanonik adı yapar, panik iletisinde interp.CallName(kd) kullanır ve
// noNamedArgs bayrağını değere yazar.
func TestMethodWrappersTurnPanicsIntoNamedErrors(t *testing.T) {
	boom := func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) { panic("patladı") }
	c := CodeMethod(codes.MemUpdate, true, boom)
	if c.Name != codes.Name(codes.MemUpdate) || !c.NoNamedArgs {
		t.Fatalf("CodeMethod: %+v", c)
	}
	if _, err := c.Fn(nil, nil, nil); !messages.Has(err, messages.H2000) || !strings.Contains(err.Error(), interp.CallName(codes.MemUpdate).String()) {
		t.Fatalf("CodeMethod panik hatası: %v", err)
	}
}

// Kural: OptionalBool verilmemiş adlı Mantık argümanında varsayılanı verir, verilmişte değeri okur,
// Mantık olmayan değerde argümanı adıyla anan hata verir; ad iletinin dilindedir (4D).
func TestOptionalBool(t *testing.T) {
	a := NamedArg(codes.ArgDim)
	for _, def := range []bool{false, true} {
		if b, err := OptionalBool("f", map[string]interp.Value{}, a, def); err != nil || b != def {
			t.Errorf("verilmemiş (varsayılan %v) = %v, %v", def, b, err)
		}
	}
	if b, err := OptionalBool("f", map[string]interp.Value{a.TR: &interp.BoolVal{Val: true}}, a, false); err != nil || !b {
		t.Errorf("doğru = %v, %v", b, err)
	}
	_, err := OptionalBool("f", map[string]interp.Value{a.TR: &interp.IntVal{Val: 1}}, a, false)
	if err == nil || !strings.Contains(err.Error(), "f: "+a.TR+" ") {
		t.Errorf("Mantık olmayan değer adıyla hata vermeli: %v", err)
	}
	en := codes.NameRef(codes.ArgDim).MessageText("en")
	if got := messages.ErrorIn(err, "en"); en == a.TR || !strings.Contains(got, "f: "+en+" must be") {
		t.Errorf("İngilizce iletide ad İngilizce olmalı (%s): %s", en, got)
	}
}

// Kural: SafeBuiltin yerleşiğin Go paniğini adlı iç hataya çevirir, sıradan sonucu ve hatayı olduğu gibi verir.
func TestSafeBuiltin(t *testing.T) {
	boom := func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) { panic("patladı") }
	if _, err := SafeBuiltin("ad", boom)(nil, nil, nil); !messages.Has(err, messages.H2000) || !strings.Contains(err.Error(), "ad") {
		t.Errorf("panik iç hataya dönmeli: %v", err)
	}
	want := errors.New("sıradan")
	fail := func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) { return nil, want }
	if _, err := SafeBuiltin("ad", fail)(nil, nil, nil); err != want {
		t.Errorf("hata olduğu gibi dönmeli: %v", err)
	}
	ok := func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
		return &interp.IntVal{Val: 2}, nil
	}
	if v, err := SafeBuiltin("ad", ok)(nil, nil, nil); err != nil || v.Inspect() != "2" {
		t.Errorf("sonuç: %v, %v", v, err)
	}
}

// Kural: SingleDictArg tam bir sözlük argümanı bekler; sayı ve tür hatalarında yöntemin koduyla
// ArgCountError/ArgTypeError verir.
func TestSingleDictArg(t *testing.T) {
	d := interp.NewDictVal()
	if got, err := SingleDictArg(codes.MemLoadState, []interp.Value{d}, "örnek"); err != nil || got != d {
		t.Errorf("sözlük = %v, %v", got, err)
	}
	if _, err := SingleDictArg(codes.MemLoadState, nil, "örnek"); err == nil || !strings.Contains(err.Error(), "örnek") {
		t.Errorf("argümansız çağrı sayı hatası vermeli (örnek anılmalı): %v", err)
	}
	if _, err := SingleDictArg(codes.MemLoadState, []interp.Value{&interp.IntVal{Val: 1}}, "örnek"); err == nil {
		t.Error("sözlük olmayan argüman hata vermeli")
	}
}

// Kural: IntList her tam sayı türünü (int, int32, int64) TR tam sayı listesine çevirir; boş dilim boş liste verir.
func TestIntList(t *testing.T) {
	if got := IntList([]int{1, 2}).Inspect(); got != "[1, 2]" {
		t.Errorf("int: %s", got)
	}
	if got := IntList([]int32{-3}).Inspect(); got != "[-3]" {
		t.Errorf("int32: %s", got)
	}
	if got := IntList([]int64{7, 8, 9}).Inspect(); got != "[7, 8, 9]" {
		t.Errorf("int64: %s", got)
	}
	if l := IntList[int](nil); l == nil || len(l.Elements) != 0 {
		t.Errorf("boş: %v", l)
	}
}
