package debugger

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

func TestDebuggerBreakpointToggleIsSafe(t *testing.T) {
	d := NewDebugger("t.tr", "a\nb", false)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			d.toggleBreakpoint(i % 7)
		}
		close(done)
	}()
	for i := 0; i < 2000; i++ {
		_ = d.hasBreakpoint(i % 7)
	}
	<-done
}

// runCLI programı terminal hata ayıklayıcısıyla, komutları input'tan okuyarak çalıştırır;
// hata ayıklayıcının ekrana yazdıklarını ve VM'i döner. Süre sınırı asılmayı yakalar.
func runCLI(t *testing.T, path, code, input string, setup func(*Debugger)) (string, *interp.VM) {
	t.Helper()
	p := parser.New(lexer.NewBody(code, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = code
	machine.Out = io.Discard
	d := NewDebugger(path, code, true)
	d.Reader = bufio.NewReader(strings.NewReader(input))
	if setup != nil {
		setup(d)
	}
	machine.Debugger = d

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	captured := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		captured <- string(b)
	}()
	done := make(chan error, 1)
	go func() {
		_, err := machine.Run(prog)
		done <- err
	}()
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		os.Stdout = stdout
		t.Fatal("program 10 saniyede bitmedi (asılı?)")
	}
	os.Stdout = stdout
	w.Close()
	out := <-captured
	if err != nil {
		t.Fatalf("program hatası: %v\n%s", err, out)
	}
	return out, machine
}

// screens hata ayıklayıcının kaç kez durduğunu (çizdiği ekran sayısını) sayar.
func screens(out string) int { return strings.Count(out, "TR HATA AYIKLAYICI") }

// İzleme koşulu bir kullanıcı fonksiyonunu çağırınca fonksiyonun deyimleri hata ayıklayıcıyı
// yeniden tetiklemez: koşul her deyimde yeniden hesaplanıp sonsuz özyinelemeye girmez.
func TestCLIWatchConditionCallsFunction(t *testing.T) {
	code := "TR_func g() {\n    TR_return 1\n}\nTR_var a = 0\na = 1\n"
	out, _ := runCLI(t, "t.tr", code, "continue\ncontinue\ncontinue\n", func(d *Debugger) {
		d.Mode = StepContinue
		d.Watchpoints = append(d.Watchpoints, &Watchpoint{ID: 1, Expression: "g() == 1", IsCond: true})
	})
	if strings.Contains(out, "YIĞIN_DERİNLİĞİ=9") || screens(out) != 1 {
		t.Fatalf("koşul bir kez sağlanıp bir kez durmalıydı (%d duruş):\n%s", screens(out), out)
	}
}

// Duraklamada 'hesapla' ya da 'değiştir' bir kullanıcı fonksiyonunu çağırınca fonksiyonun
// gövdesinde yeniden durulmaz.
func TestCLIEvaluateCallsFunctionQuietly(t *testing.T) {
	code := "TR_func g() {\n    TR_return 7\n}\nTR_var a = 0\nTR_print(a)\n"
	out, machine := runCLI(t, "t.tr", code, "step\nstep\neval g()\nset a = g()\ncontinue\n", nil)
	if screens(out) != 3 || !strings.Contains(out, "Sonuç (Tam): 7") {
		t.Fatalf("başta ve iki adım sonrası olmak üzere üç duruş bekleniyordu (%d):\n%s", screens(out), out)
	}
	if v, _ := machine.GlobalEnv.Get("a"); v == nil || v.Inspect() != "7" {
		t.Fatalf("set a = g() uygulanmalıydı: %v", v)
	}
}

// 'zıpla' düzenleyicideki gibi yalnızca duraklanan bloğun satırlarına izin verir; zıplanan
// satırlar çalışmaz, durum metni doğru ekle yazılır.
func TestCLIJump(t *testing.T) {
	code := "TR_var a = 0\n" + strings.Repeat("a = a + 1\n", 49)
	for line, want := range map[int]string{6: "Satır 6'ya zıplandı", 9: "Satır 9'a zıplandı", 3: "Satır 3'e zıplandı", 40: "Satır 40'a zıplandı"} {
		out, machine := runCLI(t, "t.tr", code, "step\njump "+strconv.Itoa(line)+"\ncontinue\n", nil)
		if !strings.Contains(out, want) {
			t.Errorf("%q bekleniyordu:\n%s", want, out)
		}
		// 2. satırda (çalışmadan) line'a zıplandı: line..50 çalışır.
		if v, _ := machine.GlobalEnv.Get("a"); v == nil || v.Inspect() != strconv.Itoa(51-line) {
			t.Errorf("jump %d: a = %d bekleniyordu, %v", line, 51-line, v)
		}
	}
}

func TestCLIJumpOutsideBlockRejected(t *testing.T) {
	code := "TR_func f() {\n    TR_var x = 1\n    TR_return x\n}\nTR_var a = f()\na = 5\n"
	// f'nin içinde (2. satır) durup gövdenin dışına (6. satır) zıplamak reddedilir.
	out, machine := runCLI(t, "t.tr", code, "step\nstep\njump 6\ncontinue\n", nil)
	if !strings.Contains(out, "çalışan bloğun dışında") || strings.Contains(out, "SATIRA ZIPLANDI") {
		t.Fatalf("blok dışına zıplama reddedilmeliydi:\n%s", out)
	}
	if v, _ := machine.GlobalEnv.Get("a"); v == nil || v.Inspect() != "5" {
		t.Fatalf("program olduğu gibi sürmeliydi: a = %v", v)
	}
}

// Ana dosyanın kesme noktası 'aktar' ile yüklenen modülün aynı numaralı satırında durmaz;
// modülün içine adımlanınca modülün satırları gösterilir.
func TestCLIModuleLines(t *testing.T) {
	dir := t.TempDir()
	module := "TR_func f(x) {\n    TR_var modül_satırı = x\n    TR_return modül_satırı\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(withHeader(module)), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "program.tr")
	code := "TR_import \"m.tr\" TR_as m\nTR_var ana_satırı = 1\nTR_var b = m.f(ana_satırı)\n"
	out, _ := runCLI(t, path, code, "break 2\ncontinue\ncontinue\n", nil)
	if screens(out) != 2 {
		t.Fatalf("başta ve 2. satırda olmak üzere iki duruş bekleniyordu (%d):\n%s", screens(out), out)
	}
	out, _ = runCLI(t, path, code, "step\nstep\nf11\n\ncontinue\n", nil)
	if !strings.Contains(out, "m.tr") || !strings.Contains(out, "===>   3 |     TR_var modül_satırı = x") {
		t.Fatalf("modülün içinde modülün satırları gösterilmeliydi:\n%s", out)
	}
}

// İzleme noktası boş metinden bir değere geçişi ve tip değişimini (Tam 123 → Metin "123")
// değişiklik sayar.
func TestWatchpointObserve(t *testing.T) {
	wp := &Watchpoint{}
	if _, changed := wp.observe(&interp.StringVal{Val: ""}); changed {
		t.Fatal("ilk görülen değer değişiklik sayılmamalı")
	}
	if old, changed := wp.observe(&interp.StringVal{Val: "a"}); !changed || old != "" {
		t.Fatalf("boş metinden \"a\"ya geçiş değişiklik sayılmalı (%v, %q)", changed, old)
	}
	wp = &Watchpoint{}
	wp.observe(&interp.IntVal{Val: 123})
	if _, changed := wp.observe(&interp.StringVal{Val: "123"}); !changed {
		t.Fatal("Tam 123'ten Metin \"123\"e geçiş değişiklik sayılmalı")
	}
	if _, changed := wp.observe(&interp.StringVal{Val: "123"}); changed {
		t.Fatal("aynı değer değişiklik sayılmamalı")
	}
}

// Hakem mimari 25: terminal hata ayıklayıcısında f5 adım, f8 devam idi; düzenleyicide F5
// devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık. Artık tuş adları iki yerde de
// aynı anlamdadır.
func TestTerminalKeysMatchVSCode(t *testing.T) {
	oldOutput := os.Stdout
	empty, _ := os.Open(os.DevNull)
	os.Stdout = empty
	defer func() { os.Stdout = oldOutput; empty.Close() }()
	for input, await := range map[string]StepMode{
		"f5":        StepContinue,
		"F5":        StepContinue,
		"continue":  StepContinue,
		"f10":       StepOver,
		"next":      StepOver,
		"f11":       StepInto,
		"step":      StepInto,
		"":          StepInto,
		"shift+f11": StepOut,
		"out":       StepOut,
	} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		d.Reader = bufio.NewReader(strings.NewReader(input + "\n"))
		d.promptCLI(interp.New(), 1, interp.NewEnvironment(nil))
		if d.Mode != await {
			t.Errorf("%q: kip %d bekleniyordu, %d", input, await, d.Mode)
		}
	}
}

// Komutlar ad kodudur (17.9): yalnız kimliğin adı çalışır (yukarıda); gösterim dilindeki adlar,
// kısaltmalar ve Türkçe karaktersiz biçimler çalışmaz, kimlik adını söyleyen bir ipucu verir ve komut
// beklemeye devam edilir.
func TestCommandsSingleSpelling(t *testing.T) {
	oldOutput := os.Stdout
	defer func() { os.Stdout = oldOutput }()
	for old, actual := range map[string]string{"adım": "step", "adim": "step", "s": "step", "geç": "next", "gec": "next", "o": "out",
		"devam": "continue", "c": "continue", "zıpla": "jump", "goto": "jump", "b": "break", "izle": "watch", "w": "watch",
		"yığın": "stack", "yigin": "stack", "bt": "stack", "p": "eval", "yardım": "help", "h": "help", "çıkış": "quit", "q": "quit", "cikis": "quit"} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		r, w, _ := os.Pipe()
		os.Stdout = w
		d.Reader = bufio.NewReader(strings.NewReader(old + "\ncontinue\n"))
		d.promptCLI(interp.New(), 1, interp.NewEnvironment(nil))
		w.Close()
		data, _ := io.ReadAll(r)
		os.Stdout = oldOutput
		if want := "'" + old + "' hata ayıklayıcıda bir komut değil; '" + actual + "' yazın"; d.Mode != StepContinue || !strings.Contains(string(data), want) {
			t.Errorf("%q: %q ipucu ve devam bekleniyordu, kip %d: %q", old, want, d.Mode, data)
		}
	}
}

// Kural (B8): komut olmayan ama ifade olarak ayrışan girdi 'hesapla' gibi değerlendirilir (pdb gibi);
// tanımsız bir ada 'değiştir' yeni değişken açmaz, hata verir. Önceden ifade "Bilinmeyen komut"
// alıyor, tanımsız ada değiştir sessizce değişken tanımlıyordu.
func TestCLIExpressionAndUndefinedAssign(t *testing.T) {
	code := "TR_var a = 2\nTR_print(a)\n"
	out, machine := runCLI(t, "t.tr", code, "step\na * 3\nset yok_boyle = 5\ncontinue\n", nil)
	if !strings.Contains(out, "Sonuç (Tam): 6") {
		t.Errorf("ifade değerlendirilmeli:\n%s", out)
	}
	if strings.Contains(out, "CANLI DEĞİŞTİRİLDİ") || !strings.Contains(out, "'yok_boyle'") {
		t.Errorf("tanımsız ada değiştir hata vermeli:\n%s", out)
	}
	if _, ok := machine.GlobalEnv.Get("yok_boyle"); ok {
		t.Error("tanımsız ad tanımlanmamalı")
	}
}

// Kural (17.7): hata ayıklayıcıya yazılan ifade koddur, kimlik yazımıyla okunur: TR_print(a) çalışır,
// Türkçe adıyla yazılan yazdır(a) kullanıcının adıdır ve tanımsız ad hatası verir. Neden: kod yalnız
// kimliklerle yazılır, görünüm koda geri çevrilmez; kabuk ve dosya da öyle okunur.
func TestCLIExpressionIsWrittenWithIdentities(t *testing.T) {
	code := "TR_var a = 2\nTR_print(a)\n"
	out, _ := runCLI(t, "t.tr", code, "step\neval TR_String(a * 3)\neval yazdır(a)\ncontinue\n", nil)
	if !strings.Contains(out, "Sonuç (Metin): 6") {
		t.Errorf("kimlik yazımıyla ifade hesaplanmalı:\n%s", out)
	}
	if !strings.Contains(out, "'yazdır'") || strings.Contains(out, "Sonuç (Yok)") {
		t.Errorf("Türkçe adıyla yazılan ifade tanımsız ad hatası vermeli:\n%s", out)
	}
}

// Kural (B9): başlık kutusunun her satırı kenarlıkla aynı genişliktedir (sağ kenar hizalı), iletinin
// dili ve dosya yolunun uzunluğu ne olursa olsun. Önceden dolgu yalnız yola uygulanıyordu.
func TestScreenBoxRowsAlign(t *testing.T) {
	if displayWidth("🛠️") != 2 || displayWidth("📌") != 2 || displayWidth("şğü") != 3 {
		t.Fatalf("görüntü genişliği: %d %d %d", displayWidth("🛠️"), displayWidth("📌"), displayWidth("şğü"))
	}
	for _, path := range []string{"t.tr", "çok/uzun/bir/klasör/yolu/içindeki/program_dosyası.tr", strings.Repeat("a", 100) + ".tr"} {
		out, _ := runCLI(t, path, "TR_var a = 1\n", "continue\n", nil)
		want := -1
		for _, line := range strings.Split(out, "\n") {
			if !strings.HasPrefix(line, "╔") && !strings.HasPrefix(line, "║") && !strings.HasPrefix(line, "╠") {
				continue
			}
			if want < 0 {
				want = displayWidth(line)
			}
			if w := displayWidth(line); w != want {
				t.Errorf("%s: satır genişliği %d, kenarlık %d: %q", path, w, want, line)
			}
		}
	}
}
