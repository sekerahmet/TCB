package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Tanımsız ad denetimi (docs/spec/05_deyimler.md §5.14) dinamik dilde yanlış alarm vermemeli:
// depodaki bütün .tr dosyaları (testler, örnekler, kıyas programları, kitaplık) hatasız
// denetlenir. Bilerek hatalı fikstürler (tests/negative/) hariçtir. Başka dosyaların
// aktardığı modüller, yorumlayıcıdaki gibi aktaranın genel adlarıyla denetlenir (11.1).
func TestDepodakiTRDosyalariDenetimdenGecer(t *testing.T) {
	kok := reporoot.Root()
	kodlar := map[string]string{}
	err := filepath.WalkDir(kok, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "negative", "node_modules", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(yol, ".tr") {
			veri, err := os.ReadFile(yol)
			if err != nil {
				return err
			}
			kodlar[yol] = string(veri)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kodlar) < 20 {
		t.Fatalf("yalnızca %d .tr dosyası bulundu", len(kodlar))
	}

	// Modül → aktaranların genel adları (birleşimi); aktaranlardan biri dinamikse dinamik.
	type genel struct {
		adlar   map[string]bool
		dinamik bool
	}
	moduller := map[string]*genel{}
	for yol, kod := range kodlar {
		adlar, dinamik := checker.TopLevelNames(source.Prepare(kod, source.Options{}).Program)
		for _, y := range aktarYerleri(kod) {
			cozulen, _, ok := modulYolunuCoz(y.yol, yol, yol)
			if !ok {
				continue
			}
			mutlak, _ := filepath.Abs(cozulen)
			g := moduller[mutlak]
			if g == nil {
				g = &genel{adlar: map[string]bool{}}
				moduller[mutlak] = g
			}
			for _, a := range adlar {
				g.adlar[a] = true
			}
			g.dinamik = g.dinamik || dinamik
		}
	}

	for yol, kod := range kodlar {
		goreli, _ := filepath.Rel(kok, yol)
		mutlak, _ := filepath.Abs(yol)
		var hatalar []error
		if g := moduller[mutlak]; g != nil {
			var adlar []string
			for a := range g.adlar {
				adlar = append(adlar, a)
			}
			hatalar = interp.CheckModule(kod, adlar, g.dinamik).Errors
		} else {
			hatalar = source.Prepare(kod, source.Options{TypeCheck: true}).Errors
		}
		for _, h := range hatalar {
			t.Errorf("%s: %s", filepath.ToSlash(goreli), h)
		}
	}
}
