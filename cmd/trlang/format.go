package main

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/devtools/formatter"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"os"
	"strings"
)

// formatCommand 'trlang format [--check] <dosya.tr | klasör>...' komutudur. Dosyaları standart
// biçime getirip yerine yazar; --check ile yalnızca biçimlenmesi gereken dosyaları listeler
// ve böyle bir dosya varsa 1 koduyla çıkar. Klasör verilirse içindeki (alt klasörler dahil)
// tüm .tr dosyaları işlenir.
func formatCommand(args []string) {
	check := false
	targets := fileArguments(codes.NameRef(codes.CmdFormat), args, []codes.Code{codes.OptCheck}, func(a string) bool {
		check = check || a == "-k" || isOption(a, codes.OptCheck)
		return a == "-k" || isOption(a, codes.OptCheck)
	})
	if len(targets) == 0 {
		exitUsage(messages.Text(messages.H8691, codes.NameRef(codes.CmdFormat), codes.OptionFlag(codes.OptCheck)))
	}

	files, err := collectTrFiles(targets, messages.H2108)
	if err != nil {
		exitWithError(err)
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
			reason, line := formatDifference(string(content), updated)
			fmt.Println(messages.Text(messages.H8693, filePath, reason, line))
			continue
		}
		if err := writeInPlace(filePath, updated); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", err)
			faulty++
			continue
		}
		fmt.Println(messages.Text(messages.H8694, filePath))
	}

	switch {
	case check && changed > 0:
		fmt.Println(messages.Text(messages.H8695, len(files), changed, codes.NameRef(codes.CmdFormat)))
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
// klasörler hariç) sıralı olarak döndürür; açılamayan hedef komutun failed iletisiyle (H2108
// biçimlenemedi, H2115 kodlanamadı) bildirilir.
func trFiles(target string, failed messages.Code) ([]string, error) {
	return source.WalkFiles(target, ".tr", func(target string, err error) error {
		return turkish.FileError("", target, failed, err)
	}, func(path string, err error) error {
		return turkish.FileError("", path, messages.H2109, err)
	})
}

// formatDifference --check'in neden biçimlenmesi gerektiğini ve satırını söyler (ajan nedeni tahmin
// etmesin): CRLF satır sonu ("crlf", ilk CRLF'li satır), son satırın sonunda satır sonu yok ("eol",
// son satır), değilse biçimin ilk farklı satırı.
func formatDifference(content, formatted string) (reason string, line int) {
	if i := strings.Index(content, "\r\n"); i >= 0 {
		return "crlf", strings.Count(content[:i], "\n") + 1
	}
	if content+"\n" == formatted {
		return "eol", strings.Count(content, "\n") + 1
	}
	a, b := strings.Split(content, "\n"), strings.Split(formatted, "\n")
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return "line", i + 1
		}
	}
	return "line", min(len(a), len(b)) + 1
}
