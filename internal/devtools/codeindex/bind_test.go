package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// bindSource üye erişimleri, parametreler, adlı argümanlar ve komut_satırı anahtarları için bir proje.
const bindSource = `TR_struct Dikkat {
    baş: TR_Int
    kv_baş: TR_Int?
    TR_func TR_init(boyut: TR_Int, baş: TR_Int, kv_baş = TR_none) {
        TR_self.baş = baş
        TR_self.kv_baş = kv_baş
    }
    TR_func say() -> TR_Int {
        TR_return TR_self.baş
    }
}

TR_func yap(baş: TR_Int) -> TR_Int {
    TR_var d = Dikkat(8, baş)
    TR_return d.baş
}

TR_func gez(x) -> TR_Int {
    TR_return x.baş
}

TR_func gölge(baş: TR_Int) -> TR_Int {
    TR_var baş = 3
    TR_return baş
}

TR_var a = Dikkat(8, 2, kv_baş: 1)
TR_var e = yap(baş: 4)
TR_var ayar = TR_command_line({"baş": 8, "step": 1}, TR_descriptions: {"baş": "baş sayısı"})
`

// uses bir düğümün başvurularını "rol@satır:sütun/güven/dayanak" olarak sıralı verir.
func uses(r *Reader, id string) []string {
	var out []string
	for _, o := range r.Occurrences(id) {
		s := fmt.Sprintf("%s@%d:%d/%s", o.Role, o.Pos.Line, o.Pos.Col, o.Confidence)
		if o.Basis != "" {
			s += "/" + string(o.Basis)
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func wantUses(t *testing.T, r *Reader, id string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := uses(r, id); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s\n got  %v\n want %v", id, got, want)
	}
}

func TestMemberAccessBindsToTheField(t *testing.T) {
	r := buildSource(t, bindSource)
	if len(r.Index.Skipped) != 0 {
		t.Fatalf("the fixture must parse: %v", r.Index.Skipped)
	}
	at := func(line, word string, n int) string { return srcAt(t, bindSource, line, word, n) }
	wantUses(t, r, "field:m.tr#Dikkat.baş",
		"definition@"+at("    baş", "baş", 1)+"/EXTRACTED",
		"write@"+at("TR_self.baş = baş", "baş", 1)+"/EXTRACTED/self",    // TR_self.baş = baş
		"read@"+at("TR_return TR_self.baş", "baş", 1)+"/EXTRACTED/self", // TR_return TR_self.baş
		"read@"+at("TR_return d.baş", "baş", 1)+"/INFERRED/local_type",  // d.baş: yapıcıdan tek atamalı yerel
		"read@"+at("TR_return x.baş", "baş", 1)+"/INFERRED/name_only")   // x.baş: alıcı bilinmiyor, yalnızca ad tutuyor
}

func TestParameterAndNamedArgumentOccurrences(t *testing.T) {
	r := buildSource(t, bindSource)
	if n := r.Node("parameter:m.tr#Dikkat.TR_ctx_init.kv_baş"); n == nil || n.Signature != "kv_baş = TR_none" {
		t.Errorf("the default value text is part of the signature: %+v", n)
	}
	at := func(line, word string, n int) string { return srcAt(t, bindSource, line, word, n) }
	wantUses(t, r, "parameter:m.tr#Dikkat.TR_ctx_init.baş",
		"definition@"+at("TR_func TR_init(", "baş", 1)+"/EXTRACTED", "read@"+at("TR_self.baş = baş", "baş", 2)+"/EXTRACTED")
	wantUses(t, r, "parameter:m.tr#Dikkat.TR_ctx_init.kv_baş",
		"definition@"+at("TR_func TR_init(", "kv_baş", 1)+"/EXTRACTED", "read@"+at("TR_self.kv_baş = kv_baş", "kv_baş", 2)+"/EXTRACTED",
		"argument_name@"+at("Dikkat(8, 2, kv_baş: 1)", "kv_baş", 1)+"/EXTRACTED")
	wantUses(t, r, "parameter:m.tr#yap.baş",
		"definition@"+at("TR_func yap(", "baş", 1)+"/EXTRACTED", "read@"+at("Dikkat(8, baş)", "baş", 1)+"/EXTRACTED",
		"argument_name@"+at("yap(baş: 4)", "baş", 1)+"/EXTRACTED")
}

func TestShadowedParameterIsInferred(t *testing.T) {
	r := buildSource(t, bindSource)
	for _, u := range uses(r, "parameter:m.tr#gölge.baş") {
		if strings.HasPrefix(u, "read") && !strings.Contains(u, "INFERRED") {
			t.Errorf("a parameter rebound in the body is only a guess: %s", u)
		}
	}
}

func TestCommandLineOptionKeys(t *testing.T) {
	r := buildSource(t, bindSource)
	at := func(word string, n int) string { return srcAt(t, bindSource, "TR_command_line(", word, n) }
	wantUses(t, r, "option:m.tr#baş", "option_key@"+at("baş", 1)+"/EXTRACTED", "option_key@"+at("baş", 2)+"/EXTRACTED")
	// Seçeneğin adı metin anahtarıdır (3.7): kimlik taşımaz, çevrilmez.
	if n := r.Node("option:m.tr#step"); n == nil || n.Kind != KindOption || n.Code != "" {
		t.Errorf("each key is an option by its text: %+v", n)
	}
}

func TestNameOnlyGuessesStayOutOfReferences(t *testing.T) {
	r := buildSource(t, bindSource)
	res := r.References("Dikkat.baş", QueryOptions{})
	guess := srcAt(t, bindSource, "TR_return x.baş", "baş", 1)
	for _, u := range res.Users {
		if fmt.Sprintf("%d:%d", u.Pos.Line, u.Pos.Col) == guess {
			t.Errorf("a name-only guess is not a reference: %+v", u)
		}
	}
	if res.Counts.Total != 3 {
		t.Errorf("self write, self read and the typed local: %d", res.Counts.Total)
	}
}

// Kural: komut satırı seçeneğinin adı çalışma anındaki adıdır: metin anahtarının kendisi (3.7; `{"step": 100}` →
// `--step`), hiçbir dilde çevrilmez. Düğüm `option:x.tr#step`'tir, kimlik taşımaz; bir dildeki ad (`adım`) ayrı bir
// seçenektir. Konum tırnağın içindeki adın yeridir.
func TestAnOptionIsItsTextKey(t *testing.T) {
	dir := t.TempDir()
	src := "TR_var c = TR_command_line({\"step\": 100, \"adım\": 1, \"run\": \"trial\"}, TR_descriptions: {\"step\": \"steps\"})\n"
	if err := os.WriteFile(filepath.Join(dir, "x.tr"), trText("x.tr", src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	r := NewReader(idx)
	for _, id := range []string{"option:x.tr#step", "option:x.tr#run", "option:x.tr#adım"} {
		if n := r.Node(id); n == nil || n.Kind != KindOption || n.Code != "" {
			t.Errorf("%s is an option by its text key: %+v", id, n)
		}
	}
	if n := r.Node("option:x.tr#TR_mem_step"); n != nil {
		t.Errorf("an option carries no identity: %+v", n)
	}
	at := func(word string, n int) string { return srcAt(t, src, "TR_command_line(", word, n) }
	wantUses(t, r, "option:x.tr#adım", "option_key@"+at("adım", 1)+"/EXTRACTED")
	wantUses(t, r, "option:x.tr#step", "option_key@"+at("step", 1)+"/EXTRACTED", "option_key@"+at("step", 2)+"/EXTRACTED")
}

// memberGuessFiles alıcısı bilinmeyen üye erişimleri için bir proje: katman.tr'de aynı adlı beş metot (`ileri`: aday
// sınırı 4 aşılır) ve beş alan (`ağırlık`), iki metot (`geri`: sınır içi); kullan.tr katmanı aktarır, başka.tr aktarmaz.
var memberGuessFiles = map[string]string{
	"katman.tr": func() string {
		var b strings.Builder
		b.WriteString(source.Header + "\n")
		for i := 1; i <= 5; i++ {
			fmt.Fprintf(&b, "TR_struct Katman%d {\n    ağırlık = %d\n    TR_func ileri() {\n        TR_return TR_self.ağırlık\n    }\n", i, i)
			if i <= 2 {
				b.WriteString("    TR_func geri() -> TR_Int {\n        TR_return 0\n    }\n")
			}
			b.WriteString("}\n")
		}
		return b.String()
	}(),
	"kullan.tr": source.Header + "\nTR_import \"katman\"\n\nTR_func çalış(p) {\n    TR_print(p.ileri())\n    p.ağırlık = 2\n    TR_print(p.geri())\n}\n",
	"başka.tr":  source.Header + "\nTR_func başka(p) {\n    TR_print(p.ileri())\n    TR_print(p.ağırlık)\n}\n",
}

// Kural: alıcısı bilinmeyen üye erişimi (`p.ileri()`, `p.ağırlık = 2`) aynı adlı ve erişimin dosyasından görünen her
// alana, ve aynı adlı görünen metot 4'ten çoksa her metoda "olası" başvurudur: kullananlar göstermez, yeniden adlandırma
// "olası"da rolüyle (çağrı, yazma, okuma) listeler; metodun dosyasını görmeyen dosyadaki aynı adlı erişim hiç sayılmaz.
// Sınır içindeki metotlar (`geri`, 2) ada göre eşleşen adaydır (kenarı vardır). Bu davranış, tahminin kullanım yeri
// başına bir kez saklanıp sorguda çözülmesiyle değişmez (TestIndexSizeGrowsLinearlyWithSameNamedMembers).
func TestMemberGuessesAreLikelyOnlyWhereTheMemberIsVisible(t *testing.T) {
	dir := t.TempDir()
	for name, text := range memberGuessFiles {
		if err := os.WriteFile(filepath.Join(dir, name), trText(filepath.Join(dir, name), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	r := NewReader(idx)
	likely := func(q string) []string {
		var out []string
		for _, c := range r.RenameImpact(q, "yeni", RenameOptions{NoText: true}).Likely {
			out = append(out, fmt.Sprintf("%s:%d %s %s", c.Pos.File, c.Pos.Line, c.Role, c.Basis))
		}
		sort.Strings(out)
		return out
	}
	for q, want := range map[string]string{
		"Katman3.ileri":   "kullan.tr:5 call name_only",
		"Katman3.ağırlık": "kullan.tr:6 write name_only",
		"Katman1.geri":    "kullan.tr:7 call name_match",
	} {
		if got := strings.Join(likely(q), "; "); got != want {
			t.Errorf("rename %s: likely %q, want %q (başka.tr does not import katman.tr)", q, got, want)
		}
	}
	for _, q := range []string{"Katman3.ileri", "Katman3.ağırlık"} {
		for _, u := range r.References(q, QueryOptions{}).Users {
			if u.Pos.File != "katman.tr" {
				t.Errorf("references %s: a name-only guess is not a reference: %+v", q, u)
			}
		}
	}
}

// Demetin öğe okuması (t.0, t.0.1; 3.6a) bir üyenin kullanımı değildir: dizine üye tahmini girmez,
// dizin kurulurken düşmez; zincirin sonundaki gerçek üye (t.0.a) yine bir tahmin olarak dizinde kalır.
func TestTupleElementIsNotAMemberUse(t *testing.T) {
	const src = `TR_struct K {
    a: TR_Int
}

TR_func f() -> TR_Int {
    TR_var t = ((K(1), 2), 3)
    TR_return t.0.0.a + t.0.1 + t.1
}
`
	r := buildSource(t, src)
	if len(r.Index.Skipped) != 0 {
		t.Fatalf("the fixture must parse: %v", r.Index.Skipped)
	}
	var members []string
	for _, g := range r.Index.MemberGuesses {
		members = append(members, g.Member)
	}
	if strings.Join(members, " ") != "a" {
		t.Errorf("member guesses: got %v, want only [a]", members)
	}
	at := srcAt(t, src, "    TR_return t.0.0.a", "a", 1)
	wantUses(t, r, "field:m.tr#K.a", "definition@"+srcAt(t, src, "    a", "a", 1)+"/EXTRACTED", "read@"+at+"/INFERRED/name_only")
}
