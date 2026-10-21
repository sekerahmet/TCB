package main

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/devtools/formatter"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// formatCommand 'tr biçimle [--kontrol] <dosya.tr | klasör>...' komutudur. Dosyaları standart
// biçime getirip yerine yazar; --kontrol ile yalnızca biçimlenmesi gereken dosyaları listeler
// ve böyle bir dosya varsa 1 koduyla çıkar. Klasör verilirse içindeki (alt klasörler dahil)
// tüm .tr dosyaları işlenir.
func formatCommand(args []string) {
	check := false
	var targets []string
	for _, a := range args {
		if a == "-k" || isOption(a, codes.K101) {
			check = true
		} else {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		exitWithError(messages.Text(messages.H8691, codes.DisplayName(codes.K008), optionName(codes.K101)))
	}

	var files []string
	for _, h := range targets {
		d, err := trFiles(h)
		if err != nil {
			exitWithError(err)
		}
		files = append(files, d...)
	}

	changed, faulty := 0, 0
	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", filePath, messages.H2100, err))
			faulty++
			continue
		}
		updated, err := formatter.Format(string(content))
		if err != nil {
			var meaning *formatter.MeaningChangedError
			if errors.As(err, &meaning) {
				fmt.Fprintln(os.Stderr, "✗ "+messages.Text(messages.H8692, filePath, err))
			} else {
				fmt.Fprintf(os.Stderr, "✗ %s: %v\n", filePath, err)
			}
			faulty++
			continue
		}
		if updated == string(content) {
			continue
		}
		changed++
		if check {
			fmt.Println(messages.Text(messages.H8693, filePath))
			continue
		}
		info, _ := os.Stat(filePath)
		perm := fs.FileMode(0644)
		if info != nil {
			perm = info.Mode().Perm()
		}
		if err := os.WriteFile(filePath, []byte(updated), perm); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", turkish.FileError("", filePath, messages.H2101, err))
			faulty++
			continue
		}
		fmt.Println(messages.Text(messages.H8694, filePath))
	}

	switch {
	case check && changed > 0:
		fmt.Println(messages.Text(messages.H8695, len(files), changed, codes.DisplayName(codes.K008)))
	case faulty > 0:
		fmt.Println(messages.Text(messages.H8696, len(files), faulty))
	case check:
		fmt.Println("✅ " + messages.Text(messages.H8697, len(files)))
	default:
		fmt.Println(messages.Text(messages.H8698, len(files), changed))
	}
	if faulty > 0 || (check && changed > 0) {
		os.Exit(1)
	}
}

// trFiles hedef bir dosyaysa onu, klasörse altındaki tüm .tr dosyalarını (gizli
// klasörler hariç) sıralı olarak döndürür.
func trFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, turkish.FileError("", target, messages.H2108, err)
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	var files []string
	err = filepath.WalkDir(target, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return turkish.FileError("", filePath, messages.H2109, err)
		}
		if d.IsDir() {
			if filePath != target && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tr") {
			files = append(files, filePath)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
