package interp

import (
	"bytes"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Değerin metni işlemin yazıldığı dosyanın dilindedir (3, 17.1; sözcüksel kural): metin(),
// yazdır, metin yerleştirmesi ve tip() adları o dosyanın dilinde çıkar. Kitaplık (Türkçe)
// fonksiyonu kendi dilinde, geri çağırma kendi dosyasının dilinde çevirir. Inspect kanonik
// kalır (JSON yazıcısı ve günlük onu kullanır).

// runEnglish İngilizce yazılmış main metnini kodlayıp (dil=en) programın kendisi gibi çalıştırır;
// files çalışma klasörüne yazılan (Türkçe) modüllerdir.
func runEnglish(t *testing.T, main string, files map[string]string) string {
	t.Helper()
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	coded, err := source.EncodeIn(main, en)
	if err != nil {
		t.Fatalf("İngilizce metin kodlanamadı: %v", err)
	}
	root := t.TempDir()
	runInDir(t, root)
	for name, content := range files {
		testkit.WritePath(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	r := source.Prepare(coded, source.Options{})
	if r.Stage != "" || r.Language != "en" {
		t.Fatalf("hazırlık: dil %q, %v", r.Language, r.Errors)
	}
	var out bytes.Buffer
	m := New()
	m.Out, m.Err = &out, &out
	m.SourceFile, m.SourceCode, m.SourceLanguage = filepath.Join(root, "main.tr"), r.Text, r.Language
	if _, err := m.Run(r.Program); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

func TestValueTextFollowsTheFilesLanguage(t *testing.T) {
	out := runEnglish(t, `var b = true
print(b, none, type(1), [true, none, {k: false}], {false: 1})
print("{b} {none} {type(b)} {[type([])]}")
print(str(b) == "true", str(none) == "none", str(type(1.5)) == "Float")
`, nil)
	want := "true none Int [true, none, {\"k\": false}] {false: 1}\n" +
		"true none Bool [List]\n" +
		"true true true\n"
	if out != want {
		t.Errorf("İngilizce dosyada değerler İngilizce yazılmalı:\n%s\nbeklenen:\n%s", out, want)
	}
	// Anlam kanonik kalır: Inspect (veri biçimleri) Türkçedir.
	if True.Inspect() != "doğru" || Null.Inspect() != "yok" || TypeOfValue(NewInt(1)).Inspect() != "Tam" {
		t.Errorf("Inspect kanonik kalmalı: %s %s %s", True.Inspect(), Null.Inspect(), TypeOfValue(NewInt(1)).Inspect())
	}
}

// Türkçe modülün fonksiyonu İngilizce programdan çağrılınca metni Türkçedir (modülün dili);
// modüle verilen İngilizce geri çağırma, modülden çağrılınca İngilizce çevirir (kendi dosyası).
func TestValueTextIsLexical(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(
		"fonksiyon anlat(x) { sonuç \"{x} \" + metin(tip(x)) }\n" +
			"fonksiyon uygula(f) { yazdır(f(doğru)) }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
import "yerel" as y
print(k.anlat(true), y.anlat(none))
k.uygula((x) => "{x} " + str(x))
y.uygula((x) => str([x]))
`, map[string]string{"yerel.tr": "fonksiyon anlat(x) { sonuç metin(x) }\nfonksiyon uygula(f) { yazdır(f(yok)) }\n"})
	want := "doğru Mantık yok\ntrue true\n[none]\n"
	if out != want {
		t.Errorf("metin işlemin yazıldığı dosyanın dilinde olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}
