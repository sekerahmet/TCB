package codeindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// libraryProject kitaplık adları gerçek kodlar olan küçük bir proje kurar: Doğrusal (TR_type_Linear), ileri
// (TR_mem_forward), öğrenme_oranı_planı (TR_fn_lr_schedule) ve parametreleri; örnekte aynı adlı bir kullanıcı işlevi
// (ileri) ve aynı adlı bir yerleşik adlı argüman (özel_türev(ileri:), TR_arg_forward) da vardır.
func libraryProject(t testing.TB) *Reader {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"lib/layers.tr": `TR_struct TR_Linear {
    TR_weight = 1
    TR_func TR_forward(TR_x) {
        TR_return TR_x * TR_self.TR_weight
    }
}

TR_func TR_lr_schedule(TR_step, TR_num_steps, TR_warmup = TR_none) {
    TR_return TR_step / TR_num_steps
}
`,
		"lib/layers_test.tr": `TR_import "tr/layers"

TR_test "doğrusal" {
    TR_var d = TR_Linear()
    TR_assert(d.TR_forward(2) == 2)
    TR_assert(TR_lr_schedule(1, 2) == 0.5)
}
`,
		"examples/a.tr": `TR_import "tr/layers"

TR_func ileri(x) {
    TR_return x
}

TR_var d = TR_Linear()
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
		"struct:lib/layers.tr#TR_type_Linear":                        {"TR_Linear", "TR_type_Linear", "Doğrusal", "Linear", "layers.Linear", "layers.TR_Linear", "layers.TR_type_Linear"},
		"method:lib/layers.tr#TR_type_Linear.TR_mem_forward":         {"layers.Doğrusal.ileri", "layers.Linear.forward", "Linear.forward", "layers.TR_Linear.TR_forward", "layers.TR_type_Linear.TR_mem_forward"},
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
}

// TestHelperLibraryText bir test değil, alt süreçtir: projenin kitaplık adlarının tanımını okuyanın dilinde yazar.
func TestHelperLibraryText(t *testing.T) {
	out := os.Getenv("CODEINDEX_LIBRARY_OUT")
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	r := libraryProject(t)
	var b strings.Builder
	for _, q := range []string{"TR_Linear", "TR_lr_schedule", "TR_num_steps"} {
		b.WriteString(Text(r.Definition(q, QueryOptions{})))
	}
	if err := os.WriteFile(filepath.Clean(out), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Kural: çıktı okuyanın dilindedir (TR_LANG). Dizin kimlikleri tutar; kitaplık adı, nitelenmiş adın kodlu
// parçaları ve imzadaki anahtar sözcük ile parametre adları gösterim dilinde yazılır, kullanıcının yazdığı belge
// yorumu ve modül adı aynen kalır.
func TestALibraryNameIsShownInTheReadersLanguage(t *testing.T) {
	t.Setenv("TR_LANG", "en")
	en := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "en"))
	for _, part := range []string{"layers.Linear ", "layers.Linear.forward", "layers.Linear.weight", "func lr_schedule(step, num_steps, warmup = …)",
		"layers.lr_schedule.num_steps"} {
		if !strings.Contains(en, part) {
			t.Errorf("English output lacks %q:\n%s", part, en)
		}
	}
	for _, part := range []string{"Doğrusal", "ileri", "öğrenme_oranı_planı", "adım"} {
		if strings.Contains(en, part) {
			t.Errorf("English output shows the Turkish name %q:\n%s", part, en)
		}
	}
	t.Setenv("TR_LANG", "tr")
	tr := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "tr"))
	if !strings.Contains(tr, "layers.Doğrusal.ileri") || !strings.Contains(tr, "fonksiyon öğrenme_oranı_planı(adım, adım_sayısı, ısınma = …)") {
		t.Errorf("Turkish output:\n%s", tr)
	}
}
