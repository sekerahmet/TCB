package stdlib

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Ad çakışması sınavı (madde 15, ad kuralı): bir yazım dilinde (F) yazılmış dosyada başka bir
// gömülü dilin (O) sözcüğü kullanıcının adı olarak geçince ya ad kuralı hatası verilir (sözcük
// bulunduğu yerin ad alanında O'nun bir sözcüğüyse; O'nun bağlamsal sözcükleri her yerde) ya da
// program, adı nötr bir adla değiştirilmiş hâliyle aynı sonucu verir. O'nun her sözcüğü dil
// dosyasından ad alanıyla okunur (değer adları program adı değildir; dil tanımı 17'nin tablosu),
// F'de aynı yazılanlar sınava girmez; programlar Türkçe görünümle yazılıp kodlu biçim üzerinden
// F'nin görünümüne çevrilir (kodların biçimini bilmez).
//
// Bugün kalan durumlar testdata/name_clash_known.txt'dedir ("F O yer sözcük"); liste yalnızca
// küçülür: listede olmayan durum kalırsa ya da listedeki durum geçerse test düşer. Artık geçen
// satırları silmek için: go test ./internal/stdlib -run TestNameClashExam -update (yeni kalan
// durum elle, nedeniyle eklenir).

var updateKnown = flag.Bool("update", false, "testdata/name_clash_known.txt'ten artık geçen satırları sil")

// clashSlot şablonlarda sınanan adın yeridir; nötr program bu adla koşar.
const clashSlot = "ad_1"

// clashPlace sınavın bir yeridir: main (ve varsa modül) Türkçe görünümde yazılır, clashSlot
// sınanan addır. namespaces, sözcüğün o yerde yasak olduğu ad alanlarıdır (tasarımın A tablosu).
type clashPlace struct {
	name       string
	namespaces []codes.Namespace
	main       string
	// module modülün Türkçe görünümüdür (m.tr); moduleInOther doğruysa modül O'da yazılır ve
	// sınanan adı o tanımlar (modülden gelen ad), değilse F'de ve nötrdür.
	module        string
	moduleInOther bool
	// neutralFails nötr programın da hata verdiğini söyler (tanımsız ad, tip, üye ya da adlı
	// argüman): serbest sözcük aynı ileti kodlarını vermelidir.
	neutralFails bool
}

var (
	global = []codes.Namespace{codes.NamespaceGlobal}
	value  = []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType} // değer olarak okunan ad tipe de bağlanır
	member = []codes.Namespace{codes.NamespaceMember}
)

var clashPlaces = []clashPlace{
	{name: "değişken", namespaces: global, main: "değişken ad_1 = 5\nyazdır(ad_1 + 1)\n"},
	{name: "fonksiyon", namespaces: global, main: "fonksiyon ad_1(x) {\n    sonuç x * 2\n}\nyazdır(ad_1(3))\n"},
	{name: "parametre", namespaces: global, main: "fonksiyon f(ad_1) {\n    sonuç ad_1 * 2\n}\nyazdır(f(3))\n"},
	{name: "blok_parametresi", namespaces: global, main: "yazdır([1, 2].haritala((ad_1) => ad_1 * 2))\n"},
	{name: "yakala_adı", namespaces: global, main: "dene {\n    fırlat \"x\"\n} yakala ad_1 {\n    yazdır(ad_1)\n}\n"},
	{name: "takma_ad", namespaces: global, module: "değişken ad_2 = 1\n", main: "aktar \"m\" olarak ad_1\nyazdır(ad_1.ad_2)\n"},
	{name: "değer", namespaces: value, main: "yazdır(ad_1)\n", neutralFails: true},
	{name: "yapı", namespaces: value, main: "yapı ad_1 {\n    a\n}\ndeğişken p: ad_1 = ad_1(a: 1)\nyazdır(p.a)\n"},
	{name: "tip_açıklaması", namespaces: []codes.Namespace{codes.NamespaceType}, main: "değişken x: ad_1 = 1\nyazdır(x)\n", neutralFails: true},
	{name: "alan", namespaces: member, main: "yapı P {\n    ad_1\n}\ndeğişken p = P(ad_1: 1)\nyazdır(p.ad_1)\n"},
	{name: "metot", namespaces: member, main: "yapı P {\n    a\n    fonksiyon ad_1() {\n        sonuç kendi.a + 1\n    }\n}\nyazdır(P(a: 1).ad_1())\n"},
	{name: "üye", namespaces: member, main: "yazdır([1, 2].ad_1)\n", neutralFails: true},
	{name: "sözlük_anahtarı", namespaces: member, main: "değişken d = {ad_1: 1}\nyazdır(d, d.ad_1)\n"},
	{name: "adlı_argüman", namespaces: []codes.Namespace{codes.NamespaceParam}, main: "yazdır(toplam(tensör([1.0, 2.0]), ad_1: 0))\n", neutralFails: true},
	// Kullanıcı fonksiyonuna verilen adlı argüman serbesttir; parametrenin tanımı genel ad alanındadır.
	{name: "kullanıcı_adlı_argümanı", namespaces: global, main: "fonksiyon f(a, ad_1 = 0) {\n    sonuç a + ad_1\n}\nyazdır(f(1, ad_1: 2))\n"},
	{name: "modül_adı", namespaces: value, module: "değişken ad_1 = 5\n", moduleInOther: true, main: "aktar \"m\"\nyazdır(ad_1)\n"},
	{name: "modül_üyesi", namespaces: member, module: "değişken ad_1 = 7\n", moduleInOther: true, main: "aktar \"m\" olarak m\nyazdır(m.ad_1)\n"},
}

// clashOutcome bir programın sonucudur: durduğu aşama ("" hatasız), hataları ve çıktısı.
type clashOutcome struct {
	stage string
	errs  []error
	out   string
}

// clashCodes hataların ileti kodlarıdır (sonucun metinden bağımsız karşılaştırması).
func (o clashOutcome) codes() string {
	var r []string
	for _, err := range o.errs {
		var m *messages.Error
		if errors.As(err, &m) {
			r = append(r, m.Code.String())
		} else {
			r = append(r, "?")
		}
	}
	return strings.Join(r, ",")
}

// namesWord sonucun durduğu aşama kodlama ya da denetimse ve hatalardan biri sözcüğü anıyorsa doğrudur
// (ad kuralının bugünkü yaklaşığı; ileti kodu gelince ona sıkılaşır).
func (o clashOutcome) namesWord(word string) bool {
	if o.stage == "" || o.stage == "run" {
		return false
	}
	for _, err := range o.errs {
		for _, lang := range codes.Languages() {
			if strings.Contains(messages.ErrorIn(err, lang), "'"+word+"'") {
				return true
			}
		}
	}
	return false
}

// clashCase F'de yazılmış programı (görünüm metni) kodlar, denetler ve çalıştırır; modül varsa
// dir/m.tr'ye yazılır.
func clashCase(t *testing.T, dir string, text string, f *codes.Language, module string) clashOutcome {
	t.Helper()
	modulePath := filepath.Join(dir, "m.tr")
	os.Remove(modulePath)
	if module != "" {
		if err := os.WriteFile(modulePath, []byte(module), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coded, err := source.EncodeIn(text, f)
	if err != nil {
		return clashOutcome{stage: "encode", errs: []error{err}}
	}
	r := source.Prepare(coded, source.Options{TypeCheck: true})
	if r.Stage != "" {
		return clashOutcome{stage: r.Stage, errs: r.Errors}
	}
	var out bytes.Buffer
	vm := interp.New()
	vm.Out, vm.Err = &out, &out
	vm.SourceFile, vm.SourceCode, vm.SourceLanguage = filepath.Join(dir, "main.tr"), r.Text, r.Language
	_, err = vm.Run(r.Program)
	vm.WaitAsync()
	if err != nil {
		return clashOutcome{stage: "run", errs: []error{err}, out: out.String()}
	}
	return clashOutcome{out: out.String()}
}

// viewIn Türkçe görünümde yazılmış metni lang'in görünümüne çevirir.
func viewIn(t *testing.T, turkish string, lang *codes.Language) string {
	t.Helper()
	text, err := source.Show(turkish, lang)
	if err != nil {
		t.Fatalf("şablon %s görünümüne çevrilemedi: %v\n%s", lang.Code, err, turkish)
	}
	return text
}

// clashWord O'nun bir sözcüğüdür: adı ve yasak olduğu ad alanları.
type clashWord struct {
	name       string
	namespaces []codes.Namespace
	contextual bool
}

// clashWords O'nun program adları olan sözcükleridir (genel, tip, üye, adlı argüman); F'de aynı
// yazılan sözcük girmez.
func clashWords(o, f *codes.Language) []clashWord {
	programNamespaces := []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType, codes.NamespaceMember, codes.NamespaceParam}
	byName := map[string]*clashWord{}
	var names []string
	for _, k := range o.Codes() {
		ns := k.Namespace()
		// Değer adları (inf, nan) genel ad alanında olsa da program adı değildir: kaynakta sözcük
		// olarak okunmaz, yalnız metinden okunur (dil tanımı 17'nin sınıf tablosu).
		if !slices.Contains(programNamespaces, ns) || k.Class() == codes.ClassValueName {
			continue
		}
		name := o.Name(k)
		w := byName[name]
		if w == nil {
			w = &clashWord{name: name}
			byName[name] = w
			names = append(names, name)
		}
		w.namespaces = append(w.namespaces, ns)
		w.contextual = w.contextual || k.Class() == codes.ClassContextual
	}
	var r []clashWord
	for _, name := range names {
		same := false
		for _, ns := range programNamespaces {
			if _, ok := f.Find(ns, name); ok {
				same = true
			}
		}
		if !same {
			r = append(r, *byName[name])
		}
	}
	slices.SortFunc(r, func(a, b clashWord) int { return strings.Compare(a.name, b.name) })
	return r
}

func TestNameClashExam(t *testing.T) {
	if testing.Short() {
		t.Skip("binlerce küçük program")
	}
	dir := t.TempDir()
	var failing []string
	for _, fCode := range codes.Languages() {
		f, err := codes.LoadLanguage(fCode)
		if err != nil {
			t.Fatal(err)
		}
		for _, oCode := range codes.Languages() {
			if oCode == fCode {
				continue
			}
			o, err := codes.LoadLanguage(oCode)
			if err != nil {
				t.Fatal(err)
			}
			words := clashWords(o, f)
			for _, p := range clashPlaces {
				main := viewIn(t, p.main, f)
				module, moduleLang := "", f
				if p.moduleInOther {
					moduleLang = o
				}
				if p.module != "" {
					module = viewIn(t, p.module, moduleLang)
				}
				neutral := clashCase(t, dir, main, f, encodeModule(module, moduleLang))
				if (neutral.stage != "") != p.neutralFails {
					t.Fatalf("%s/%s: nötr program %s aşamasında durdu: %v\n%s", fCode, p.name, neutral.stage, neutral.errs, main)
				}
				for _, w := range words {
					forbidden := w.contextual
					for _, ns := range w.namespaces {
						forbidden = forbidden || slices.Contains(p.namespaces, ns)
					}
					mod := ""
					if module != "" {
						mod = module
						if p.moduleInOther {
							mod = strings.ReplaceAll(module, clashSlot, w.name)
						}
						if mod = encodeModule(mod, moduleLang); mod == "" {
							continue // sözcük O'da ad olarak tanımlanamıyor (anahtar sözcük): yer yok
						}
					}
					got := clashCase(t, dir, strings.ReplaceAll(main, clashSlot, w.name), f, mod)
					ok := got.namesWord(w.name)
					if !forbidden {
						ok = got.stage == neutral.stage && got.codes() == neutral.codes() &&
							got.out == strings.ReplaceAll(neutral.out, clashSlot, w.name)
					}
					if !ok {
						failing = append(failing, fmt.Sprintf("%s %s %s %s", fCode, oCode, p.name, w.name))
					}
				}
			}
		}
	}
	header := "# Ad çakışması sınavının bugün kalan durumları (TestNameClashExam): F O yer sözcük.\n" +
		"# Liste yalnızca küçülür; -update ile yeniden yazılır.\n"
	testkit.CheckKnownList(t, "testdata/name_clash_known.txt", header, failing, *updateKnown)
}

// encodeModule modülün görünüm metnini lang'de kodlar; kodlanamıyorsa boş verir.
func encodeModule(text string, lang *codes.Language) string {
	if text == "" {
		return ""
	}
	coded, err := source.EncodeIn(text, lang)
	if err != nil {
		return ""
	}
	return coded
}
