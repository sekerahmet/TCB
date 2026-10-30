package checker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Depodaki programlar diskteki biçimleriyle (kodlu dosya, dili başlığında) okunup denetlenir:
// source.Prepare dosyayı programın okuduğu gibi kimliklere çevirir ve denetleyiciye dosyanın
// dilini verir. Dış test paketidir: source denetleyiciyi içe aktarır.

// prepareFile dosyayı okuyup ayrıştırır ve denetler; okunamayan dosya testi düşürür.
func prepareFile(t *testing.T, file string) *source.Result {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return source.Prepare(string(data), source.Options{TypeCheck: true})
}

// Kural (yanlış alarm vermez, 2.3b): örnek programlar hatasız denetlenir; depodaki programlar
// (örnekler, dil tanımı testleri ve kıyas programları) uyarısızdır: yerleşik adları gölgeleyen
// ya da yazılış kuralına uymayan kullanıcı adı yoktur.
func TestExamplesCheckWithoutErrors(t *testing.T) {
	examples := testkit.GlobFiles(t, "examples/tr/*.tr")
	if len(examples) == 0 {
		t.Fatal("examples/tr/*.tr bulunamadı")
	}
	for _, file := range examples {
		t.Run(filepath.Base(file), func(t *testing.T) {
			if r := prepareFile(t, file); r.Stage != "" {
				t.Errorf("hata beklenmiyordu (%s): %v", r.Stage, r.Errors)
			}
		})
	}
	for _, file := range testkit.GlobFiles(t, "examples/tr/*.tr", "examples/tr/helpers/*.tr", "tests/*.tr", "tools/bench/tr/*.tr") {
		r := prepareFile(t, file)
		if r.Stage == source.StageSyntax {
			t.Errorf("%s: %v", file, r.Errors)
			continue
		}
		if len(r.Warnings) > 0 {
			t.Errorf("%s uyarısız denetlenmeli: %v", file, r.Warnings)
		}
	}
}

// Kural (2.3b): kitaplık ve GPU araçları da yazılış kuralına uyar (kitaplıkta gölgeleme bilinçli
// olabilir: adlı parametreler girdi, taban gibi yerleşik adları taşır).
func TestLibraryFollowsNameStyle(t *testing.T) {
	files := testkit.GlobFiles(t, "lib/*.tr", "tools/gpu/*.tr")
	if len(files) == 0 {
		t.Fatal("lib/*.tr bulunamadı")
	}
	for _, file := range files {
		r := prepareFile(t, file)
		if r.Stage == source.StageSyntax {
			t.Errorf("%s: %v", file, r.Errors)
			continue
		}
		for _, w := range r.Warnings {
			if messages.Has(w, messages.H1012) || messages.Has(w, messages.H1013) || messages.Has(w, messages.H1014) || messages.Has(w, messages.H1015) {
				t.Errorf("%s yazılış kuralına (2.3b) uymalı: %v", file, w)
			}
		}
	}
}

// Kural (6.1, 7.1): depodaki programlarda tipsiz parametre, alan ve dönüş yoktur (kapının hatası
// çıkmaz); gömülü kitaplığın dosyaları kendi adlarıyla denetlenir, 4c'yi bekleyen yerleri istisnadır.
// Olumsuz fikstürler (tests/negative) sayılmaz: hatayı onlar bekler.
func TestRepoProgramsAreTyped(t *testing.T) {
	gate := []messages.Code{messages.H1109, messages.H1110, messages.H1111, messages.H1112, messages.H1113}
	files := testkit.GlobFiles(t, "lib/*.tr", "tests/*.tr", "examples/tr/*.tr", "examples/tr/helpers/*.tr", "tools/gpu/*.tr", "tools/bench/tr/*.tr")
	if len(files) < 60 {
		t.Fatalf("yalnız %d dosya bulundu; depo düzeni değişmiş olabilir", len(files))
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		library := ""
		if filepath.Base(filepath.Dir(file)) == "lib" {
			library = filepath.Base(file)
		}
		r := source.Prepare(string(data), source.Options{TypeCheck: true, Library: library})
		for _, err := range append(r.Errors, r.Warnings...) {
			for _, code := range gate {
				if messages.Has(err, code) {
					t.Errorf("%s: %v", file, err)
				}
			}
		}
	}
}
