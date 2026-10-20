package main

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/devtools/formatter"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
		case "--kontrol", "-k":
			kontrol = true
		case "--check":
			hataylaCik(messages.Text(messages.H8690))
		default:
			hedefler = append(hedefler, a)
		}
	}
	if len(hedefler) == 0 {
		hataylaCik(messages.Text(messages.H8691))
	}

	var dosyalar []string
	for _, h := range hedefler {
		d, err := trDosyalari(h)
		if err != nil {
			hataylaCik(err)
		}
		dosyalar = append(dosyalar, d...)
	}

	degisen, hatali := 0, 0
	for _, yol := range dosyalar {
		veri, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", yol, messages.H2100, err))
			hatali++
			continue
		}
		yeni, err := formatter.Format(string(veri))
		if err != nil {
			var anlam *formatter.MeaningChangedError
			if errors.As(err, &anlam) {
				fmt.Fprintln(os.Stderr, "✗ "+messages.Text(messages.H8692, yol, err))
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
			fmt.Println(messages.Text(messages.H8693, yol))
			continue
		}
		bilgi, _ := os.Stat(yol)
		izin := fs.FileMode(0644)
		if bilgi != nil {
			izin = bilgi.Mode().Perm()
		}
		if err := os.WriteFile(yol, []byte(yeni), izin); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", yol, messages.H2101, err))
			hatali++
			continue
		}
		fmt.Println(messages.Text(messages.H8694, yol))
	}

	switch {
	case kontrol && degisen > 0:
		fmt.Println(messages.Text(messages.H8695, len(dosyalar), degisen))
	case hatali > 0:
		fmt.Println(messages.Text(messages.H8696, len(dosyalar), hatali))
	case kontrol:
		fmt.Println("✅ " + messages.Text(messages.H8697, len(dosyalar)))
	default:
		fmt.Println(messages.Text(messages.H8698, len(dosyalar), degisen))
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
		return nil, turkish.FileError("", hedef, messages.H2108, err)
	}
	if !bilgi.IsDir() {
		return []string{hedef}, nil
	}
	var dosyalar []string
	err = filepath.WalkDir(hedef, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return turkish.FileError("", yol, messages.H2109, err)
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
