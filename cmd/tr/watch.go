package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// watchCommand 'tr izle [--çıktı dosya] program.tr [argümanlar]': programı çalıştırır ve hızını
// canlı gösterir.
//
// Standart hata bir terminalse tablo ekranın üstünde saniyede bir yeniden çizilir;
// programın kendi çıktısı (yazdır, ilerleme çubuğu, hatalar) tabloyla karışmasın diye bir
// dosyaya yakalanır, son satırları tablonun altında görünür ve program bitince yeniden
// yazılır. Terminal değilse program çıktısı olduğu gibi akar, 10 saniyede bir düz özet
// satırı eklenir. Her iki durumda program bitince (hata ya da kesmeyle bitse de) son rapor
// standart hataya yazılır.
func watchCommand(args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") && !strings.EqualFold(filepath.Ext(args[0]), ".tr") {
		if _, err := os.Stat(args[0]); err != nil {
			exitWithError(messages.Text(messages.H8670, args[0], codes.DisplayName(codes.K012)))
		}
	}
	s, path, arguments := profileArguments(codes.DisplayName(codes.K012), args, codes.K105)
	machine, prog, code := prepareProgram(path, arguments)
	realExit, realError := os.Stdout, os.Stderr
	live := profiler.NewLive(realError, logging.IsTerminal(realError), nil)
	var catch *profiler.Capture
	if live.Terminal() {
		filePath := s.output
		if filePath == "" {
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			filePath = filepath.Join(os.TempDir(), "tr-izle-"+name+"-"+strconv.Itoa(os.Getpid())+".txt")
		}
		var err error
		if catch, err = profiler.StartCapture(filePath); err != nil {
			live.Close()
			exitWithError(messages.Text(messages.H8671, err))
		}
		live.Capture(catch)
		os.Stdout, os.Stderr = catch.Writer(), catch.Writer()
		machine.Out, machine.Err = catch.Writer(), catch.Writer()
	}
	p := startMeasuredProfile(machine, path, code, prog)
	p.Listen(live.Show)
	exitCleanup = append(exitCleanup, func() {
		end := p.Finish(exitCode)
		live.Close()
		if catch != nil {
			os.Stdout, os.Stderr = realExit, realError
			catch.Close()
			fmt.Fprintln(realError, "\n"+messages.Text(messages.H8672, catch.Path()))
			if err := catch.WriteError(); err != nil {
				fmt.Fprintln(realError, "  ⚠️ "+messages.Text(messages.H8673, err))
			}
			for _, l := range catch.LastLines(15) {
				fmt.Fprintln(realError, "  │ "+l)
			}
		}
		fmt.Fprint(realError, "\n"+profiler.ReportText(end))
	})
	runProgram(machine, prog, code)
	cleanup()
}
