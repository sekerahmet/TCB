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
		"lib/katman.tr": `yapı Doğrusal {
    ağırlık = 1
    fonksiyon ileri(x) {
        sonuç x * kendi.ağırlık
    }
}

fonksiyon öğrenme_oranı_planı(adım, adım_sayısı, ısınma = yok) {
    sonuç adım / adım_sayısı
}
`,
		"lib/katman_test.tr": `aktar "tr/katman"

test "doğrusal" {
    değişken d = Doğrusal()
    doğrula(d.ileri(2) == 2)
    doğrula(öğrenme_oranı_planı(1, 2) == 0.5)
}
`,
		"examples/a.tr": `aktar "tr/katman"

fonksiyon ileri(x) {
    sonuç x
}

değişken d = Doğrusal()
yazdır(d.ileri(ileri(3)), öğrenme_oranı_planı(1, 4))
değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x, []] }, geri: fonksiyon(s, g) { sonuç [g] })
`,
	} {
		testkit.WriteFile(t, dir, name, text)
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
		"struct:lib/katman.tr#Doğrusal":                           {"TR_type_Linear", "Doğrusal", "Linear", "katman.Linear", "katman.TR_type_Linear"},
		"method:lib/katman.tr#Doğrusal.ileri":                     {"katman.Doğrusal.ileri", "katman.Linear.forward", "Linear.forward", "katman.TR_type_Linear.TR_mem_forward"},
		"function:lib/katman.tr#öğrenme_oranı_planı":              {"TR_fn_lr_schedule", "öğrenme_oranı_planı", "lr_schedule", "LR_SCHEDULE"},
		"parameter:lib/katman.tr#öğrenme_oranı_planı.adım_sayısı": {"lr_schedule.num_steps", "katman.öğrenme_oranı_planı.adım_sayısı"},
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
		if want == "struct:lib/katman.tr#Doğrusal" && !strings.Contains(first[1], "examples/a.tr") || want == "function:lib/katman.tr#öğrenme_oranı_planı" && !strings.Contains(first[2], "katman_test") {
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
	if !slices.Equal(turkish, english) || !slices.Contains(english, "method:lib/katman.tr#Doğrusal.ileri") || !slices.Contains(english, "builtin:TR_arg_forward") {
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
	for _, q := range []string{"TR_type_Linear", "TR_fn_lr_schedule", "TR_arg_num_steps"} {
		b.WriteString(Text(r.Definition(q, QueryOptions{})))
	}
	if err := os.WriteFile(filepath.Clean(out), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Kural: çıktı okuyanın dilindedir (TR_DIL). Dizin Türkçe yazımı tutar; kitaplık adı, nitelenmiş adın kodlu
// parçaları ve imzadaki anahtar sözcük ile parametre adları gösterim dilinde yazılır, kullanıcının yazdığı belge
// yorumu ve modül adı aynen kalır.
func TestALibraryNameIsShownInTheReadersLanguage(t *testing.T) {
	en := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "en"))
	for _, part := range []string{"katman.Linear ", "katman.Linear.forward", "katman.Linear.weight", "func lr_schedule(step, num_steps, warmup = …)",
		"katman.lr_schedule.num_steps"} {
		if !strings.Contains(en, part) {
			t.Errorf("English output lacks %q:\n%s", part, en)
		}
	}
	for _, part := range []string{"Doğrusal", "ileri", "öğrenme_oranı_planı", "adım"} {
		if strings.Contains(en, part) {
			t.Errorf("English output shows the Turkish name %q:\n%s", part, en)
		}
	}
	tr := string(testkit.RunWithLanguageFile(t, "TestHelperLibraryText", "CODEINDEX_LIBRARY_OUT", "tr"))
	if !strings.Contains(tr, "katman.Doğrusal.ileri") || !strings.Contains(tr, "fonksiyon öğrenme_oranı_planı(adım, adım_sayısı, ısınma = …)") {
		t.Errorf("Turkish output:\n%s", tr)
	}
}
