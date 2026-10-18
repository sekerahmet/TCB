package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/kaynak"
	"tr_dili/internal/dil/yurut"
)

// Tanımsız ad denetimi (docs/dil/05_deyimler.md §5.14) dinamik dilde yanlış alarm vermemeli:
// depodaki bütün .tr dosyaları (testler, örnekler, kıyas programları, kitaplık) hatasız
// denetlenir. Bilerek hatalı fikstürler (testler/olumsuz/) hariçtir. Başka dosyaların
// aktardığı modüller, yorumlayıcıdaki gibi aktaranın genel adlarıyla denetlenir (11.1).
func TestDepodakiTRDosyalariDenetimdenGecer(t *testing.T) {
	kok := depo.Kok()
	kodlar := map[string]string{}
	err := filepath.WalkDir(kok, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "olumsuz", "node_modules", ".claude":
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
		adlar, dinamik := denetci.UstDuzeyAdlari(kaynak.Hazirla(kod, kaynak.Secenekler{}).Program)
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
		var hatalar []string
		if g := moduller[mutlak]; g != nil {
			var adlar []string
			for a := range g.adlar {
				adlar = append(adlar, a)
			}
			hatalar = yurut.ModulDenetle(kod, adlar, g.dinamik).Hatalar
		} else {
			hatalar = kaynak.Hazirla(kod, kaynak.Secenekler{TipDenetimi: true}).Hatalar
		}
		for _, h := range hatalar {
			t.Errorf("%s: %s", filepath.ToSlash(goreli), h)
		}
	}
}
