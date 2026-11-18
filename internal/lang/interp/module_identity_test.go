package interp

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// counterModule her çağrıda bir artan sayaçtır: iki aktarım aynı modülse a.artir(), b.artir()
// "1 2" verir, ayrı kopyalarsa "1 1".
const counterModule = source.Header + "\nTR_var sayi = 0\n\nTR_func artir() -> TR_Int {\n    sayi = sayi + 1\n    TR_return sayi\n}\n"

// twoImports aynı sayaca iki yazımla ulaşan ana programdır (başlıksız gövde, satır 1 ve 2).
func twoImports(first, second string) string {
	return "TR_import \"" + first + "\" TR_as a\nTR_import \"" + second + "\" TR_as b\nTR_print(a.artir())\nTR_print(b.artir())\n"
}

// linkOrSkip bağlantıyı kurar; kurulamazsa (Windows'ta ayrıcalık) alt test nedenini yazıp atlanır.
func linkOrSkip(t *testing.T, kind string, make func() error) {
	t.Helper()
	if err := make(); err != nil {
		t.Skipf("%s kurulamadı: %v", kind, err)
	}
}

// junctionOrSkip Windows'ta dizin kavşağı kurar (mklink /J yönetici istemez).
func junctionOrSkip(t *testing.T, target, name string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("dizin kavşağı yalnız Windows'ta")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", name, target).CombinedOutput(); err != nil {
		t.Skipf("dizin kavşağı kurulamadı: %v %s", err, out)
	}
}

// runMain root/ana.tr'yi (başlıksız gövde) çalıştırır; check doğruysa önce çalışmadan önceki
// denetimden geçer (1.3), değilse yükleyici tek başına sınanır.
func runMain(t *testing.T, root, main string, check bool) (string, error) {
	t.Helper()
	mainFile := testkit.WritePath(t, filepath.Join(root, "ana.tr"), main)
	p := parser.New(bodyLexer(main))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("ana.tr sözdizimi: %v", p.Errors())
	}
	if check {
		if err := CheckImportedModules(mainFile, prog); err != nil {
			return "", err
		}
	}
	var out bytes.Buffer
	m := New()
	m.Out, m.Err = &out, &out
	m.SourceFile, m.SourceCode = mainFile, main
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// counterRoot root/lib/sayac.tr'yi yazar ve kökü verir.
func counterRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCounter(t, filepath.Join(root, "lib", "sayac.tr"))
	return root
}

// writeCounter sayaç modülünü filePath'e yazar.
func writeCounter(t *testing.T, filePath string) {
	t.Helper()
	testkit.WritePath(t, filePath, counterModule)
}

// Kural (11.3): aynı dosya aynı modüldür. Başka bir yazımla (bağlantı, dizin kavşağı, sabit bağ,
// sürücü harfinin büyüklüğü) ulaşılan dosya bir kez yüklenir, durumu paylaşılır ("1 2"); denetim
// de onu bir kez denetler (testleri bir kez koşar, 12.1). Neden: anahtar mutlak yoldu, ikinci
// yazım sessizce ikinci bir kopya kuruyordu ("1 1").
func TestAModuleThroughAnotherPathLoadsOnce(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string) string // ikinci yazım
	}{
		{"dizin bağlantısı", func(t *testing.T, root string) string {
			linkOrSkip(t, "dizin bağlantısı", func() error { return os.Symlink(filepath.Join(root, "lib"), filepath.Join(root, "bag")) })
			return "bag/sayac"
		}},
		{"aynı klasördeki dosya bağlantısı", func(t *testing.T, root string) string {
			linkOrSkip(t, "dosya bağlantısı", func() error {
				return os.Symlink(filepath.Join(root, "lib", "sayac.tr"), filepath.Join(root, "lib", "sayac_bag.tr"))
			})
			return "lib/sayac_bag"
		}},
		{"aynı klasördeki sabit bağ", func(t *testing.T, root string) string {
			linkOrSkip(t, "sabit bağ", func() error {
				return os.Link(filepath.Join(root, "lib", "sayac.tr"), filepath.Join(root, "lib", "sayac_sabit.tr"))
			})
			return "lib/sayac_sabit"
		}},
		{"dizin kavşağı", func(t *testing.T, root string) string {
			junctionOrSkip(t, filepath.Join(root, "lib"), filepath.Join(root, "kavsak"))
			return "kavsak/sayac"
		}},
		{"sürücü harfi", func(t *testing.T, root string) string {
			abs := filepath.ToSlash(filepath.Join(root, "lib", "sayac"))
			if runtime.GOOS != "windows" || len(filepath.VolumeName(abs)) != 2 {
				t.Skip("sürücü harfi yalnız Windows'ta")
			}
			drive := abs[:1]
			if strings.ToUpper(drive) == drive {
				return strings.ToLower(drive) + abs[1:]
			}
			return strings.ToUpper(drive) + abs[1:]
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := counterRoot(t)
			second := c.setup(t, root)
			main := twoImports("lib/sayac", second)
			out, err := runMain(t, root, main, true)
			if err != nil || out != "1\n2\n" {
				t.Fatalf("%q ile %q aynı modül olmalı: çıktı %q, hata %v", "lib/sayac", second, out, err)
			}
			if out, err := runMain(t, root, main, false); err != nil || out != "1\n2\n" {
				t.Fatalf("yükleyici tek başına da aynı modülü vermeli: çıktı %q, hata %v", out, err)
			}
			p := parser.New(bodyLexer(main))
			files, err := ImportedModuleFiles(filepath.Join(root, "ana.tr"), p.ParseProgram())
			if err != nil || len(files) != 1 {
				t.Fatalf("denetim modülü bir kez görmeli: %v, hata %v", files, err)
			}
		})
	}
}

// Kural (11.3): aynı dosyaya klasör yolu (klasör ve üst klasörleri) farklı iki yazımla ulaşmak
// çalışmadan önceki denetimin hatasıdır (H2726), yükleyicinin de; hiçbir satır çalışmaz. İleti
// ilk aktarımın dosyasını aktaran dosyanın klasörüne göre önerir; öneri kopyalanınca iki aktarım
// aynı modüldür ("1 2"). Neden: modülün göreli yolu sözcükseldir (`..`); klasör yolu farklı
// yazımda modülün aktardığı dosya hangi aktarımın önce geldiğine bağlanırdı.
func TestAModuleThroughAnotherFolderIsAnError(t *testing.T) {
	otherFolderLink := func(t *testing.T, root string) {
		linkOrSkip(t, "dosya bağlantısı", func() error {
			return os.Symlink(filepath.Join(root, "lib", "sayac.tr"), filepath.Join(root, "baska", "sayac.tr"))
		})
	}
	otherFolderHardLink := func(t *testing.T, root string) {
		linkOrSkip(t, "sabit bağ", func() error {
			return os.Link(filepath.Join(root, "lib", "sayac.tr"), filepath.Join(root, "baska", "sayac.tr"))
		})
	}
	otherParentLink := func(t *testing.T, root string) {
		linkOrSkip(t, "dizin bağlantısı", func() error {
			return os.Symlink(filepath.Join(root, "lib"), filepath.Join(root, "alt", "bag2"))
		})
	}
	cases := []struct {
		name          string
		setup         func(t *testing.T, root string)
		first, second string
	}{
		{"başka klasördeki dosya bağlantısı", otherFolderLink, "lib/sayac", "baska/sayac"},
		{"başka klasördeki sabit bağ", otherFolderHardLink, "lib/sayac", "baska/sayac"},
		{"başka üst klasördeki dizin bağlantısı", otherParentLink, "lib/sayac", "alt/bag2/sayac"},
		{"başka üst klasördeki dizin bağlantısı, öbür sıra", otherParentLink, "alt/bag2/sayac", "lib/sayac"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := counterRoot(t)
			for _, dir := range []string{"baska", "alt"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			c.setup(t, root)
			main := twoImports(c.first, c.second)
			for _, check := range []bool{true, false} {
				out, err := runMain(t, root, main, check)
				if !messages.Has(err, messages.H2726) || out != "" {
					t.Fatalf("denetim=%v: H2726 bekleniyordu, hiçbir satır çalışmamalı: çıktı %q, hata %v", check, out, err)
				}
				if !strings.Contains(err.Error(), "ana.tr") || !strings.Contains(err.Error(), c.second) {
					t.Errorf("ileti ikinci yazımı ve ilk aktarımın dosyasını söylemeli: %v", err)
				}
				suggest := suggestedPath(t, err)
				if suggest != c.first {
					t.Errorf("öneri ilk yazım olmalı: %q, beklenen %q", suggest, c.first)
				}
				if out, err := runMain(t, root, twoImports(c.first, suggest), check); err != nil || out != "1\n2\n" {
					t.Errorf("öneri kopyalanınca aynı modül olmalı: çıktı %q, hata %v", out, err)
				}
			}
		})
	}
}

// suggestedPath iletinin son tırnaklı yoludur (H2726'nın önerisi).
func suggestedPath(t *testing.T, err error) string {
	t.Helper()
	text := err.Error()
	end := strings.LastIndex(text, `"`)
	start := strings.LastIndex(text[:max(end, 0)], `"`)
	if start < 0 || end <= start {
		t.Fatalf("iletide önerilen yol yok: %s", text)
	}
	return text[start+1 : end]
}

// Kural (11.3, 11.4): döngü dosyanın kimliğiyle yakalanır; ana program da kümededir. Kendini
// gösteren bağlantıyla (d/ic → d) kendini aktaran modül ve bağlantıyla ana programa dönen modül
// ilk tekrarda H2481 alır; denetim (`trlang check`'in yolu) aynı dosyaya ikinci kez inmez,
// sonlanır. Neden: yol bağlantıda 64 kat uzayıp "ic klasörü yok" diyordu, ana programa dönen
// modül ana programı ikinci kez çalıştırıyordu.
func TestAnImportCycleThroughALinkIsAnError(t *testing.T) {
	t.Run("kendini gösteren bağlantı", func(t *testing.T) {
		root := t.TempDir()
		testkit.WritePath(t, filepath.Join(root, "d", "m.tr"), source.Header+"\nTR_import \"ic/m\" TR_as m\n")
		linkOrSkip(t, "dizin bağlantısı", func() error { return os.Symlink(filepath.Join(root, "d"), filepath.Join(root, "d", "ic")) })
		assertCycle(t, root, "TR_import \"d/m\"\n")
	})
	t.Run("ana programa dönen bağlantı", func(t *testing.T) {
		top := t.TempDir()
		root := filepath.Join(top, "a")
		testkit.WritePath(t, filepath.Join(root, "lib.tr"), source.Header+"\nTR_import \"../b/ana\" TR_as ana\n")
		linkOrSkip(t, "dizin bağlantısı", func() error { return os.Symlink(root, filepath.Join(top, "b")) })
		assertCycle(t, root, "TR_import \"lib\"\nTR_print(1)\n")
	})
}

// assertCycle denetimin hatasız sonlandığını, çalışmanın H2481 verdiğini ve hiçbir satırın
// çalışmadığını sınar.
func assertCycle(t *testing.T, root, main string) {
	t.Helper()
	p := parser.New(bodyLexer(main))
	prog := p.ParseProgram()
	testkit.WritePath(t, filepath.Join(root, "ana.tr"), main)
	if err := CheckImportedModules(filepath.Join(root, "ana.tr"), prog); err != nil {
		t.Fatalf("denetim döngüde sonlanmalı (döngüyü çalışma bildirir): %v", err)
	}
	out, err := runMain(t, root, main, false)
	var mh *ModuleError
	if errors.As(err, &mh) && mh.Err != nil {
		err = mh.Err
	}
	if !messages.Has(err, messages.H2481) || out != "" {
		t.Fatalf("H2481 bekleniyordu: çıktı %q, hata %v", out, err)
	}
}
