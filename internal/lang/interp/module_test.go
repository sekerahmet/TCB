package interp

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// runWithModules klasör altına dosyaları yazar, çalışma klasörünü kok yapar ve ana.tr'yi çalıştırır.
// Modül bir dosyadır (2.1): başlıksız modül metnine başlık yazılır, satırları bir artar. ana.tr başlıksız
// kodlu gövde olarak okunur, satırları yazıldığı gibidir.
func runWithModules(t *testing.T, root string, files map[string]string) (string, error) {
	t.Helper()
	for name, content := range files {
		if name != "ana.tr" && strings.HasSuffix(name, ".tr") && source.HeaderLines(content) == 0 {
			content = source.Header + "\n" + content
		}
		testkit.WritePath(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	codeVal := files["ana.tr"]
	p := parser.New(bodyLexer(codeVal))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("ana.tr sözdizimi: %v", p.Errors())
	}
	var out bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &out
	m.SourceFile = filepath.Join(root, "ana.tr")
	m.SourceCode = codeVal
	// Çalışmadan önce aktarılan modüller denetlenir (1.3); hata varsa hiçbir satır çalışmaz.
	if err := CheckImportedModules(m.SourceFile, prog); err != nil {
		return out.String(), err
	}
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// Modülün üst düzeyindeki 'sonuç' aktaran programı sessizce bitiriyordu (çıkış kodu 0).
// Derleme hatasıdır: modülün hiçbir satırı çalışmaz, aktar hata verir.
func TestModuleTopLevelReturnError(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
		"m1.tr":  "TR_print(\"modül çalıştı\")\nTR_return 5\n",
		"ana.tr": "TR_import \"m1\"\nTR_print(\"aktardıktan sonra\")\n",
	})
	if err == nil || !strings.Contains(err.Error(), "'TR_return' modülün üst düzeyinde kullanılamaz") {
		t.Fatalf("modül hatası bekleniyordu: %v\n%s", err, out)
	}
	if !strings.Contains(err.Error(), "m1.tr, Satır 3") || strings.Contains(out, "modül çalıştı") || strings.Contains(out, "aktardıktan") {
		t.Fatalf("hata m1.tr'nin satırını göstermeli, modül çalışmamalı: %v\n%s", err, out)
	}
}

// Modüller tip denetiminden geçmeden çalışıyordu (docs/spec/01_program.md §1 ile çelişki).
func TestModulePassesTypeCheck(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	out, err := runWithModules(t, root, map[string]string{
		"m5.tr":  "TR_var t: TR_Int = \"metin\"\nTR_func f(a: TR_Int) -> TR_Int { TR_return \"yanlış tip\" }\nTR_func g() {\n    TR_print(\"m5 çalıştı\")\n}\n",
		"ana.tr": "TR_import \"m5\"\n",
	})
	if err == nil {
		t.Fatalf("tip hatalı modül çalıştı:\n%s", out)
	}
	message := err.Error()
	for _, wantSub := range []string{"Satır 1:", "\"m5\" modülü tip denetimi hataları nedeniyle yüklenemedi", "m5.tr, Satır 2, Sütun 1: Tip uyuşmazlığı", "m5.tr, Satır 3, Sütun 34: Dönüş tipi uyuşmazlığı"} {
		if !strings.Contains(message, wantSub) {
			t.Errorf("hatada %q yok: %s", wantSub, message)
		}
	}
	if strings.Contains(out, "m5 çalıştı") {
		t.Error("tip hatalı modülün hiçbir satırı çalışmamalı")
	}
	// Maddeler: aktar satırı ana dosyadan, her tip hatası modülün kaynağından çerçevelenir.
	entries := ErrorItems(err, "TR_import \"m5\"\n")
	if len(entries) != 3 || !strings.Contains(entries[0].Frame, "1 | TR_import \"m5\"") ||
		!strings.Contains(entries[1].Frame, "2 | TR_var t: TR_Int = \"metin\"") ||
		!strings.Contains(entries[2].Frame, "3 | TR_func f(a: TR_Int)") {
		t.Fatalf("maddeler yanlış: %+v", entries)
	}
}

// Modüldeki çalışma hatası ana programın aynı numaralı satırını gösteriyordu. Modülün en dışı
// çalışmaz (1.1): hata modülün fonksiyonunda, modülün değişkeniyle.
func TestModuleRuntimeErrorWithOwnFile(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{
		"m2.tr":  "TR_var x = 1\n\nTR_func z() -> TR_Float {\n    TR_return x / 0\n}\n",
		"ana.tr": "TR_import \"m2\"\n\nz()\n",
	})
	var mh *ModuleError
	if err == nil || !errors.As(err, &mh) || !strings.HasPrefix(err.Error(), "m2.tr, Satır 5: Sıfıra bölme hatası") {
		t.Fatalf("m2.tr'nin satırı bekleniyordu: %v", err)
	}
	m := ErrorItems(err, "TR_import \"m2\"\n\nz()\n")
	if len(m) != 1 || !strings.Contains(m[0].Frame, "5 |     TR_return x / 0") {
		t.Fatalf("çerçeve modülden çizilmeli: %+v", m)
	}

	// Modülde tanımlı fonksiyonun sonradan (ana programdan) çağrısındaki hata da öyle.
	root2 := t.TempDir()
	runInDir(t, root2)
	_, err = runWithModules(t, root2, map[string]string{
		"lib.tr": "TR_func böl(a: TR_Int) -> TR_Float {\n    TR_return a / 0\n}\n",
		"ana.tr": "TR_import \"lib\"\nTR_print(\"x\")\nböl(1)\n",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "lib.tr, Satır 3: Sıfıra bölme hatası") {
		t.Fatalf("lib.tr'nin satırı bekleniyordu: %v", err)
	}
	// Modülün geri çağırdığı ana program fonksiyonunun hatası ana dosyaya ait kalır.
	_, err = runWithModules(t, root2, map[string]string{
		"uygula.tr": "TR_func uygula(f: () -> TR_Float) -> TR_Float {\n    TR_return f()\n}\n",
		"ana.tr":    "TR_import \"uygula\"\nTR_func g() -> TR_Float {\n    TR_return 1 / 0\n}\nuygula(g)\n",
	})
	if err == nil || !strings.HasSuffix(strings.SplitN(err.Error(), ",", 2)[0], "ana.tr") || !strings.Contains(err.Error(), "Satır 3: Sıfıra bölme") {
		t.Fatalf("hata ana.tr'nin 3. satırına ait olmalı: %v", err)
	}
}

// Kural (1.1, iletinin önerisi çalışır): modülün en dışındaki deyim (H1107) ve sabit olmayan ilk
// değer (H1108) aktaranın denetiminde hatadır; iletinin önerdiği yazım modüle kopyalanınca modül
// denetimden geçer ve aktaran program öneriyi çağırarak aynı sonucu alır.
func TestModuleRuleSuggestionRuns(t *testing.T) {
	for _, c := range []struct {
		name, wrong, call, want string
		code                    messages.Code
	}{
		{"deyim", "TR_print(ad)", "kur()", "x\n", messages.H1107},
		{"ilk değer", "TR_const KOPYA = ad + \"y\"", "TR_print(kopya())", "xy\n", messages.H1108},
	} {
		module := "TR_var ad = \"x\"\n" + c.wrong + "\n"
		_, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\"\n"})
		var mh *ModuleError
		if !errors.As(err, &mh) || len(mh.Errors) != 1 || !messages.Has(mh.Errors[0], c.code) {
			t.Fatalf("%s: modülün %v hatası bekleniyordu: %v", c.name, c.code, err)
		}
		message := mh.Errors[0].Error()
		if !strings.Contains(message, "m.tr") && !strings.Contains(err.Error(), "\"m\"") {
			t.Errorf("%s: aktarılan modül iletide anılmalı: %v", c.name, err)
		}
		_, fix, ok := strings.Cut(message, ":\n")
		if !ok {
			t.Fatalf("%s: iletide önerilen yazım yok: %s", c.name, message)
		}
		fixed := strings.Replace(module, c.wrong, fix, 1)
		out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": fixed, "ana.tr": "TR_import \"m\"\n" + c.call + "\n"})
		if err != nil || out != c.want {
			t.Errorf("%s: önerilen yazım çalışmalı:\n%s\nçıktı %q, hata %v", c.name, fixed, out, err)
		}
	}
}

// Kural (1.1): paylaşılan durumun önerisi (H1114) kopyalanınca çalışır: değişken yoklukla başlar,
// kurma fonksiyonu değeri bir kez kurar, sonraki çağrı fonksiyonların atadığını değiştirmez.
func TestModuleSharedStateSuggestionRuns(t *testing.T) {
	wrong := "TR_var yol = taban()"
	module := "TR_func taban() -> TR_String {\n    TR_return \"k\"\n}\n" + wrong + "\nTR_func yaz(s: TR_String) {\n    yol = s\n}\nTR_func oku() -> TR_String {\n    TR_return yol ?? \"\"\n}\n"
	_, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\"\n"})
	var mh *ModuleError
	if !errors.As(err, &mh) || len(mh.Errors) != 1 || !messages.Has(mh.Errors[0], messages.H1114) {
		t.Fatalf("modülün H1114 hatası bekleniyordu: %v", err)
	}
	_, fix, ok := strings.Cut(mh.Errors[0].Error(), ":\n")
	if !ok {
		t.Fatalf("iletide önerilen yazım yok: %s", mh.Errors[0])
	}
	fixed := strings.Replace(module, wrong, fix, 1)
	call := "yol_kur()\nTR_print(oku())\nyaz(\"a\")\nyol_kur()\nTR_print(oku())\n"
	out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": fixed, "ana.tr": "TR_import \"m\"\n" + call})
	if err != nil || out != "k\na\n" {
		t.Errorf("önerilen yazım çalışmalı:\n%s\nçıktı %q, hata %v", fixed, out, err)
	}
}

// Kural (11.8): modülün en dış adına aktaran dosyadan atama H2901'dir; tipi bilinen yol da
// (m.PI = 7), denetimin göremediği yol da (listeden okunan modül değeri, bileşik atama) çalışma
// anında yakalanır ve modülün değeri değişmez. Değerin içi (alan, listeye ekleme) ve modülün kendi
// fonksiyonunun kendi değişkenini değiştirmesi geçerlidir.
func TestModuleNameIsNotReboundFromTheImporter(t *testing.T) {
	module := "TR_struct Nokta { x: TR_Int }\nTR_const PI = 3\nTR_var nokta = Nokta(x: 0)\nTR_var kayit = []\nTR_var sayac = 0\nTR_func say() -> TR_Int {\n    sayac += 1\n    TR_return sayac\n}\n"
	for _, wrong := range []string{
		"m.PI = 7",
		"m.sayac += 1",
		"TR_var modüller = [m]\nmodüller[0].PI = 7",
	} {
		out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\" TR_as m\n" + wrong + "\n"})
		if !messages.Has(err, messages.H2901) {
			t.Errorf("%s: H2901 beklenirdi, gelen %v (%s)", wrong, err, out)
		}
	}
	valid := "TR_import \"m\" TR_as m\nTR_import \"m\" TR_as n\nm.nokta.x = 1\nm.kayit.TR_append(5)\nTR_print(n.nokta.x, n.kayit, m.say(), n.say(), n.PI)\n"
	out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": valid})
	if err != nil || out != "1 [5] 1 2 3\n" {
		t.Errorf("geçerli yollar: çıktı %q, hata %v", out, err)
	}
}

// Kural (11.2): modül değeri bir sözlük değildir: köşeli ayraçla okunması da ona köşeli ayraçla atama
// da H2904'tür, modülün değeri değişmez. İletinin önerdiği nokta yazımı kopyalanınca çalışır.
func TestModuleValueIsNotIndexed(t *testing.T) {
	module := "TR_const PI = 3\n"
	for _, wrong := range []string{"TR_print(m[\"PI\"])", "m[\"PI\"] = 9", "TR_var modüller = [m]\nTR_print(modüller[0][\"PI\"])"} {
		out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\" TR_as m\n" + wrong + "\n"})
		if !messages.Has(err, messages.H2904) {
			t.Errorf("%s: H2904 beklenirdi, gelen %v (%s)", wrong, err, out)
		}
	}
	_, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\" TR_as m\nTR_print(m[\"PI\"])\n"})
	if text := messages.ErrorIn(err, "en"); !strings.Contains(text, "`m.PI`") {
		t.Fatalf("ileti m.PI'yi önermeli: %q", text)
	}
	out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": "TR_import \"m\" TR_as m\nTR_print(m.PI)\n"})
	if err != nil || out != "3\n" {
		t.Errorf("önerilen yazım çalışmalı: çıktı %q, hata %v", out, err)
	}
}

// Kural (11.3): modül değerinin adı modülün değişkeninin bugünkü değerini verir: modülün kendi
// fonksiyonu değişkenini değiştirince her takma ad yeni değeri görür (eskiden yüklemedeki kopyayı
// veriyordu: m.say() iki kez çağrılınca m.sayac 0'dı). H2901'in önerdiği yol böyle doğru sonuç verir.
func TestModuleValueReadsTheModulesVariable(t *testing.T) {
	module := "TR_var sayac = 0\nTR_func say() {\n    sayac += 1\n}\nTR_func oku() -> TR_Int {\n    TR_return sayac\n}\n"
	main := "TR_import \"m\" TR_as m\nTR_import \"m\" TR_as n\nm.say()\nm.say()\nTR_print(m.sayac, n.sayac, m.oku())\n"
	out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": module, "ana.tr": main})
	if err != nil || out != "2 2 2\n" {
		t.Errorf("çıktı %q, hata %v; \"2 2 2\" beklenirdi", out, err)
	}
}
