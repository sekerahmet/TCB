package codeindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// libraryProject kitaplık adları gerçek kodlar olan küçük bir proje kurar: Doğrusal (TR_type_Linear), ileri
// (TR_mem_forward), öğrenme_oranı_planı (TR_fn_lr_schedule) ve parametreleri; örnekte aynı adlı bir kullanıcı işlevi
// (ileri) ve aynı adlı bir yerleşik adlı argüman (özel_türev(ileri:), TR_arg_forward) da vardır.
func libraryProject(t testing.TB) *Reader {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"lib/layers.tr": `TR_enum TR_Role {
    TR_System
    TR_User
}

TR_struct TR_Linear {
    TR_weight: TR_Int = 1
    TR_func TR_forward(TR_x: TR_Int) -> TR_Int {
        TR_return TR_x * TR_self.TR_weight
    }
}

TR_func TR_lr_schedule(TR_step: TR_Int, TR_num_steps: TR_Int, TR_warmup: TR_Int? = TR_none) -> TR_Float {
    TR_return TR_step / TR_num_steps
}
`,
		"lib/layers_test.tr": `TR_import "tr/layers"

TR_test "doğrusal" {
    TR_var d = TR_Linear()
    TR_assert(d.TR_forward(2) == 2)
    TR_assert(TR_lr_schedule(1, 2) == 0.5)
    TR_assert(TR_Role.TR_User == TR_Role.TR_User)
}
`,
		"examples/a.tr": `TR_import "tr/layers"
TR_import "tr/layers" TR_as katman

TR_struct Model {
    a: TR_Int = 1
}

TR_struct A {
    n: TR_Int = 1
}

TR_func ileri(x: TR_Int) -> TR_Int {
    TR_return x
}

TR_var d = TR_Linear()
TR_var r = TR_Role.TR_User
TR_var l = [1, 2].TR_map((x) => x)
l.TR_append(3)
TR_var sh = TR_Tensor([1.0]).TR_shape
TR_print(d.TR_forward(ileri(3)), TR_lr_schedule(1, 4))
TR_var f = TR_custom_grad(TR_forward: (x) => { TR_return [x, []] }, TR_backward: (s, g) => { TR_return [g] })
`,
	} {
		testkit.WriteFile(t, dir, name, string(trText(name, text)))
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx)
}

// Kural (dil tanımı 17.3): kitaplığın adı bir koddur; kodla, her yazım dilindeki adıyla ve nitelenmiş adın her
// yazımıyla sorulan ad aynı bildirime çözülür ve tanım, kullananlar, sınayanlar aynı cevabı verir. Kullanıcının adı
// metindir: aynı Türkçe yazımlı kullanıcı işlevi İngilizce adla bulunmaz.
func TestALibraryNameIsFoundByItsCodeAndByItsNameInEveryLanguage(t *testing.T) {
	r := libraryProject(t)
	for want, spellings := range map[string][]string{
		"struct:lib/layers.tr#TR_type_Linear":                        {"TR_Linear", "TR_type_Linear", "Doğrusal", "Linear", "layers.Linear", "layers.TR_Linear", "layers.TR_type_Linear", "katman.TR_Linear"},
		"method:lib/layers.tr#TR_type_Linear.TR_mem_forward":         {"TR_forward", "x.TR_forward", "l.TR_forward", "uzun_alici.TR_forward", "TR_self.TR_forward", ".TR_forward", "TR_Linear.TR_forward", "tr_linear.TR_forward", "Linear.TR_forward", "Doğrusal.TR_forward", "doğrusal.TR_forward", "layers.Doğrusal.ileri", "layers.Linear.forward", "Linear.forward", "layers.TR_Linear.TR_forward", "layers.TR_type_Linear.TR_mem_forward"},
		"field:lib/layers.tr#TR_type_Role." + "TR_mem_User":          {"TR_User", "TR_mem_User", "Kullanıcı", "User", "TR_Role.TR_User", "Rol." + "TR_User", "Role." + "TR_User", "Rol.Kullanıcı", "Role.User"},
		"function:lib/layers.tr#TR_fn_lr_schedule":                   {"TR_lr_schedule", "TR_fn_lr_schedule", "öğrenme_oranı_planı", "lr_schedule", "LR_SCHEDULE"},
		"parameter:lib/layers.tr#TR_fn_lr_schedule.TR_arg_num_steps": {"lr_schedule.num_steps", "layers.öğrenme_oranı_planı.adım_sayısı"},
	} {
		var first []string
		for _, q := range spellings {
			var got []string
			for _, res := range []*Result{r.Definition(q, QueryOptions{}), r.References(q, QueryOptions{}), r.TestedBy(q, QueryOptions{})} {
				if res.Status != StatusOK || res.Target == nil || res.Target.ID != want {
					t.Errorf("%s %q: status %s, target %+v, want %s", res.Command, q, res.Status, res.Target, want)
				}
				got = append(got, Text(res))
			}
			if first == nil {
				first = got
			} else if !slices.Equal(got, first) {
				t.Errorf("%q answers differently from %q:\n%s\nvs\n%s", q, spellings[0], strings.Join(got, "\n"), strings.Join(first, "\n"))
			}
		}
		if want == "struct:lib/layers.tr#TR_type_Linear" && !strings.Contains(first[1], "examples/a.tr") || want == "function:lib/layers.tr#TR_fn_lr_schedule" && !strings.Contains(first[2], "layers_test") {
			t.Errorf("%s: the use in the example and the test block are listed:\n%s", want, strings.Join(first, "\n"))
		}
	}
	// Aynı adı taşıyan bir yerleşik (ileri/forward: TR_arg_forward) varken de iki yazım aynı adayları verir; kullanıcının
	// aynı Türkçe yazımlı işlevi yalnız kendi adıyla bulunur.
	ids := func(q string) []string {
		var out []string
		for _, n := range r.Resolve(q, "") {
			out = append(out, n.ID)
		}
		slices.Sort(out)
		return out
	}
	turkish, english := ids("ileri"), ids("forward")
	if !slices.Contains(turkish, "function:examples/a.tr#ileri") {
		t.Errorf("ileri -> %v: the user function is found by its own name", turkish)
	}
	turkish = slices.DeleteFunc(turkish, func(id string) bool { return id == "function:examples/a.tr#ileri" })
	if !slices.Equal(turkish, english) || !slices.Contains(english, "method:lib/layers.tr#TR_type_Linear.TR_mem_forward") || !slices.Contains(english, "builtin:TR_arg_forward") {
		t.Errorf("ileri -> %v, forward -> %v: the library method and the builtin are candidates under both names", turkish, english)
	}
	for _, q := range []string{"l.TR_map", ".TR_map", "tr_linear.TR_map"} {
		if res := r.Definition(q, QueryOptions{}); res.Status != StatusOK || res.Target == nil || res.Target.ID != "builtin:TR_mem_map" {
			t.Errorf("Definition(%q) = %+v, want builtin:TR_mem_map", q, res)
		}
	}
	if res := r.Definition("model.TR_append", QueryOptions{}); res.Status != StatusOK || res.Target == nil || res.Target.ID != "builtin:TR_mem_append" {
		t.Errorf("Definition(\"model.TR_append\") = %+v, want builtin:TR_mem_append", res)
	}
	for _, q := range []string{"TR_Linear.TR_map", "Model.TR_GPT", "foo.TR_Linear", "model.TR_Linear", "TR_self.TR_Linear", ".TR_Linear", "x.TR_Linear"} {
		if res := r.Definition(q, QueryOptions{}); res.Status != StatusNotFound {
			t.Errorf("Definition(%q) = %s (%+v), want not_found", q, res.Status, res.Target)
		}
	}
	for _, tc := range []struct {
		q     string
		owner messages.Code
	}{
		{"A.TR_shape", messages.H8877},
		{"Model.TR_append", messages.H8877},
		{"Linear.TR_map", messages.H8877},
		{"Doğrusal.TR_map", messages.H8877},
		{"layers.TR_map", messages.H8879},
	} {
		recv, member, _ := strings.Cut(tc.q, ".")
		res := r.Definition(tc.q, QueryOptions{})
		out := Text(res)
		wantOwner, wantVal := messages.Text(tc.owner, recv), messages.Text(messages.H8878, recv, member)
		hasOwner := slices.ContainsFunc(res.Candidates, func(n Node) bool { return n.NoteCode == tc.owner.String() })
		hasVal := slices.ContainsFunc(res.Candidates, func(n Node) bool { return n.NoteCode == messages.H8878.String() })
		if res.Status != StatusAmbiguous || !hasOwner || !hasVal || !strings.Contains(out, wantOwner) || !strings.Contains(out, wantVal) {
			t.Errorf("Definition(%q) = %s (%d candidates):\n%s", tc.q, res.Status, len(res.Candidates), out)
		}
	}
}

// TestHelperLibraryText bir test değil, alt süreçtir: projenin kitaplık adlarının tanımını okuyanın dilinde yazar.
func TestHelperLibraryText(t *testing.T) {
	out := os.Getenv("CODEINDEX_LIBRARY_OUT")
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	r := libraryProject(t)
	var b strings.Builder
	for _, q := range []string{"TR_Linear", "TR_lr_schedule", "TR_num_steps", "A.TR_shape"} {
		b.WriteString(Text(r.Definition(q, QueryOptions{})))
	}
	if err := os.WriteFile(filepath.Clean(out), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Kural: çıktı okuyanın dilindedir (TR_LANG). Dizin kimlikleri tutar; kitaplık adı, nitelenmiş adın kodlu
// parçaları, imzadaki anahtar sözcük ile parametre adları ve belirsiz üye sorgusunun aday etiketleri (H8877, H8878)
// gösterim dilinde yazılır, kullanıcının yazdığı belge yorumu ve modül adı aynen kalır.
func TestALibraryNameIsShownInTheReadersLanguage(t *testing.T) {
	t.Setenv("TR_LANG", "en")
	en := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "en"))
	for _, part := range []string{"layers.Linear ", "layers.Linear.forward", "layers.Linear.weight", "func lr_schedule(step, num_steps, warmup = …)",
		"layers.lr_schedule.num_steps", "[if `A` is a type: it has no such member]", "[if `A` is a value: `x.TR_shape`]"} {
		if !strings.Contains(en, part) {
			t.Errorf("English output lacks %q:\n%s", part, en)
		}
	}
	for _, part := range []string{"Doğrusal", "ileri", "öğrenme_oranı_planı", "adım", "bir tipse", "bir değerse"} {
		if strings.Contains(en, part) {
			t.Errorf("English output shows the Turkish text %q:\n%s", part, en)
		}
	}
	t.Setenv("TR_LANG", "tr")
	tr := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "tr"))
	for _, part := range []string{"layers.Doğrusal.ileri", "fonksiyon öğrenme_oranı_planı(adım, adım_sayısı, ısınma = …)",
		"[`A` bir tipse: bu üyesi yok]", "[`A` bir değerse: `x.TR_shape`]"} {
		if !strings.Contains(tr, part) {
			t.Errorf("Turkish output lacks %q:\n%s", part, tr)
		}
	}
	for _, part := range []string{"is a type", "is a value"} {
		if strings.Contains(tr, part) {
			t.Errorf("Turkish output shows the English text %q:\n%s", part, tr)
		}
	}
}

// Kural (DENETİM istek 66 K2): <takma ad>.TR_<ad> yalnız o takma adın aktardığı modülde bildirilmişse bulunur;
// başka kitaplık modülünün ya da kullanıcı modülünün takma adı öbür modüllerin bildirimlerine inmez.
func TestImportAliasResolvesOnlyDeclarationsInItsTargetModule(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"lib/layers.tr": `TR_struct TR_Linear {
    TR_weight: TR_Int = 1
}
`,
		"lib/optimizers.tr": `TR_const TR_NS_A = 3.4445
`,
		"examples/helpers/geometri.tr": `TR_struct Kare {
    kenar: TR_Int = 1
    TR_func alan() -> TR_Int {
        TR_return TR_self.kenar
    }
}
`,
		"tests/negative/kod_uye.tr": `TR_struct K {
    TR_func TR_length() -> TR_Int {
        TR_return 0
    }
}
`,
		"examples/a.tr": "TR_import \"tr/layers\" TR_as katman\n" +
			"TR_import \"tr/optimizers\" TR_as opt\n" +
			"TR_import \"helpers/geometri\" TR_as geo\n\n" +
			"TR_var d = " + "katman." + "TR_Linear()\n" +
			"TR_print(" + "opt." + "TR_NS_A, geo.Kare(), [1]." + "TR_length)\n",
	} {
		testkit.WriteFile(t, dir, name, string(trText(name, text)))
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for q, want := range map[string]string{
		"katman." + "TR_Linear": "struct:lib/layers.tr#TR_type_Linear",
		"opt." + "TR_NS_A":      "constant:lib/optimizers.tr#TR_const_NS_A",
		"k." + "TR_length":      "builtin:TR_mem_length",
		"K." + "TR_length":      "method:tests/negative/kod_uye.tr#K." + "TR_mem_length",
		"kare.alan":             "method:examples/helpers/geometri.tr#Kare.alan",
	} {
		if res := r.Definition(q, QueryOptions{}); res.Status != StatusOK || res.Target == nil || res.Target.ID != want {
			t.Errorf("Definition(%q) = %s (%+v), want %s", q, res.Status, res.Target, want)
		}
	}
	for _, q := range []string{"opt." + "TR_Linear", "katman." + "TR_NS_A", "geo." + "TR_Linear", "geo." + "TR_NS_A"} {
		if res := r.Definition(q, QueryOptions{}); res.Status != StatusNotFound {
			t.Errorf("Definition(%q) = %s (%+v), want not_found", q, res.Status, res.Target)
		}
	}
}
