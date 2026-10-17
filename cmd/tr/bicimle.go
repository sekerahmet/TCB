package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"tr_dili/pkg/bicim"
)

// bicimleKomutu 'tr biçimle [--kontrol] <dosya.tr | klasör>...' komutudur. Dosyaları standart
// biçime getirip yerine yazar; --kontrol ile yalnızca biçimlenmesi gereken dosyaları listeler
// ve böyle bir dosya varsa 1 koduyla çıkar. Klasör verilirse içindeki (alt klasörler dahil)
// tüm .tr dosyaları işlenir.
func bicimleKomutu(args []string) {
	kontrol := false
	var hedefler []string
	for _, a := range args {
		switch a {
		case "--kontrol", "-k", "--check":
			kontrol = true
		default:
			hedefler = append(hedefler, a)
		}
	}
	if len(hedefler) == 0 {
		fmt.Fprintln(os.Stderr, "Hata: Lütfen bir .tr dosyası ya da klasör belirtin. Örn: tr biçimle ornek.tr veya tr biçimle --kontrol .")
		os.Exit(1)
	}

	var dosyalar []string
	for _, h := range hedefler {
		d, err := trDosyalari(h)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		dosyalar = append(dosyalar, d...)
	}

	degisen, hatali := 0, 0
	for _, yol := range dosyalar {
		veri, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: okunamadı: %v\n", yol, err)
			hatali++
			continue
		}
		yeni, err := bicim.Bicimle(string(veri))
		if err != nil {
			var anlam *bicim.AnlamDegistiHatasi
			if errors.As(err, &anlam) {
				fmt.Fprintf(os.Stderr, "✗ %s: %v — lütfen bu dosyayı bir hata kaydıyla bildirin\n", yol, err)
			} else {
				fmt.Fprintf(os.Stderr, "✗ %s: %v\n", yol, err)
			}
			hatali++
			continue
		}
		if yeni == string(veri) {
			continue
		}
		degisen++
		if kontrol {
			fmt.Printf("biçimlenmeli: %s\n", yol)
			continue
		}
		bilgi, _ := os.Stat(yol)
		izin := fs.FileMode(0644)
		if bilgi != nil {
			izin = bilgi.Mode().Perm()
		}
		if err := os.WriteFile(yol, []byte(yeni), izin); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: yazılamadı: %v\n", yol, err)
			hatali++
			continue
		}
		fmt.Printf("biçimlendi: %s\n", yol)
	}

	switch {
	case kontrol && degisen > 0:
		fmt.Printf("%d dosyadan %d tanesi biçimlenmeli. Düzeltmek için: tr biçimle <dosya|klasör>\n", len(dosyalar), degisen)
	case kontrol:
		fmt.Printf("✅ %d dosyanın hepsi standart biçimde.\n", len(dosyalar))
	default:
		fmt.Printf("%d dosya incelendi, %d dosya biçimlendi.\n", len(dosyalar), degisen)
	}
	if hatali > 0 || (kontrol && degisen > 0) {
		os.Exit(1)
	}
}

// trDosyalari hedef bir dosyaysa onu, klasörse altındaki tüm .tr dosyalarını (gizli
// klasörler hariç) sıralı olarak döndürür.
func trDosyalari(hedef string) ([]string, error) {
	bilgi, err := os.Stat(hedef)
	if err != nil {
		return nil, fmt.Errorf("%s bulunamadı", hedef)
	}
	if !bilgi.IsDir() {
		return []string{hedef}, nil
	}
	var dosyalar []string
	err = filepath.WalkDir(hedef, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if yol != hedef && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tr") {
			dosyalar = append(dosyalar, yol)
		}
		return nil
	})
	sort.Strings(dosyalar)
	return dosyalar, err
}
