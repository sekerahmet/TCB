package profiler

import (
	"os"
	"path/filepath"
	"testing"
)

// 'aktar' ile yüklenen modülün satırları (fonksiyonları ve üst düzey kodu) modülün dosyasıyla
// ve metniyle bildirilir: ana dosyada aynı adlı bir fonksiyon olsa da ana dosyaya yazılmaz.
func TestProfileModuleLines(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "yardımcı.tr")
	code := "değişken ısınma = 0\ndöngü j içinde 0..300000 {\n    ısınma += j % 3\n}\nfonksiyon ileri(x) {\n    değişken s = 0\n    döngü i içinde 0..300000 {\n        s += i % 7\n    }\n    sonuç s + x\n}\n"
	if err := os.WriteFile(module, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	main := "aktar \"" + filepath.ToSlash(module) + "\" olarak m\nfonksiyon ileri(x) {\n    sonuç x\n}\nyazdır(m.ileri(ileri(1)))\n"
	g, _ := run(t, main, Options{SampleLines: true})
	want := map[int]string{3: "ısınma += j % 3", 8: "s += i % 7"}
	found := 0
	for _, s := range g.Sampling.Lines {
		text, ok := want[s.Line]
		if !ok || s.Share < 0.05 {
			continue
		}
		found++
		if s.File == "egit.tr" || s.Text != text || filepath.Base(filepath.FromSlash(s.File)) != "yardımcı.tr" {
			t.Errorf("modülün %d. satırı modülün dosyası ve metniyle bildirilmeli: %+v", s.Line, s)
		}
	}
	if found == 0 {
		t.Fatalf("modülün sıcak satırları örneklenmedi: %+v", g.Sampling.Lines)
	}
}
