package interp

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// runWithModules klasör altına dosyaları yazar, çalışma klasörünü kok yapar ve ana.tr'yi çalıştırır.
// Modül bir dosyadır (2.1): başlıksız modül metnine başlık yazılır, satırları bir artar. ana.tr metin
// olarak okunur (source.TextLexer), satırları yazıldığı gibidir.
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
		"m5.tr":  "TR_var t: TR_Int = \"metin\"\nTR_func f(a: TR_Int) -> TR_Int { TR_return \"yanlış tip\" }\nTR_print(\"m5 çalıştı\")\n",
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

// Modüldeki çalışma hatası ana programın aynı numaralı satırını gösteriyordu.
func TestModuleRuntimeErrorWithOwnFile(t *testing.T) {
	root := t.TempDir()
	runInDir(t, root)
	_, err := runWithModules(t, root, map[string]string{
		"m2.tr":  "TR_var x = 1\n\nTR_var z = x / 0\n",
		"ana.tr": "TR_import \"m2\"\n\nTR_print(1)\n",
	})
	var mh *ModuleError
	if err == nil || !errors.As(err, &mh) || !strings.HasPrefix(err.Error(), "m2.tr, Satır 4: Sıfıra bölme hatası") {
		t.Fatalf("m2.tr'nin satırı bekleniyordu: %v", err)
	}
	m := ErrorItems(err, "TR_import \"m2\"\n\nTR_print(1)\n")
	if len(m) != 1 || !strings.Contains(m[0].Frame, "4 | TR_var z = x / 0") {
		t.Fatalf("çerçeve modülden çizilmeli: %+v", m)
	}

	// Modülde tanımlı fonksiyonun sonradan (ana programdan) çağrısındaki hata da öyle.
	root2 := t.TempDir()
	runInDir(t, root2)
	_, err = runWithModules(t, root2, map[string]string{
		"lib.tr": "TR_func böl(a) {\n    TR_return a / 0\n}\n",
		"ana.tr": "TR_import \"lib\"\nTR_print(\"x\")\nböl(1)\n",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "lib.tr, Satır 3: Sıfıra bölme hatası") {
		t.Fatalf("lib.tr'nin satırı bekleniyordu: %v", err)
	}
	// Modülün geri çağırdığı ana program fonksiyonunun hatası ana dosyaya ait kalır.
	_, err = runWithModules(t, root2, map[string]string{
		"uygula.tr": "TR_func uygula(f) {\n    TR_return f()\n}\n",
		"ana.tr":    "TR_import \"uygula\"\nTR_func g() {\n    TR_return 1 / 0\n}\nuygula(g)\n",
	})
	if err == nil || !strings.HasSuffix(strings.SplitN(err.Error(), ",", 2)[0], "ana.tr") || !strings.Contains(err.Error(), "Satır 3: Sıfıra bölme") {
		t.Fatalf("hata ana.tr'nin 3. satırına ait olmalı: %v", err)
	}
}
