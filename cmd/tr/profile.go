package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/profiler"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// 'tr profil' ve 'tr izle' komutlarının ortak parçaları ve 'tr profil'. Profilin çekirdeği
// internal/devtools/profiler'dedir; burada yalnızca argümanlar, çıktı yerleri ve çıkış vardır.

// profileOptions komutların program dosyasından önce yazılan seçenekleridir.
type profileOptions struct {
	json      string // K103 (--json): raporun JSON yolu (klasörse içine speed.json)
	goProfile string // K104 (--go-profili): yorumlayıcının Go CPU profili (geliştiriciler için)
	output    string // K105 (--çıktı): tr izle'de programın çıktısının dosyası
}

// profileArguments seçenekleri, program dosyasını ve programın argümanlarını ayırır.
// allowed, komutun kabul ettiği seçeneklerdir. Hata varsa yazıp 1 ile çıkar.
func profileArguments(command string, args []string, allowed ...codes.Code) (profileOptions, string, []string) {
	var s profileOptions
	target := map[codes.Code]*string{codes.K103: &s.json, codes.K104: &s.goProfile, codes.K105: &s.output}
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		name, value, equal := strings.Cut(args[0], "=")
		k, ok := optionCode(name)
		if !ok || !slices.Contains(allowed, k) {
			names := make([]string, len(allowed))
			for i, a := range allowed {
				names[i] = codes.OptionFlag(a)
			}
			exitWithError(messages.Text(messages.H8674, command, name, strings.Join(names, ", ")))
		}
		p := target[k]
		args = args[1:]
		if !equal {
			if len(args) == 0 {
				exitWithError(messages.Text(messages.H8675, codes.OptionFlag(k), command))
			}
			value, args = args[0], args[1:]
		}
		*p = value
	}
	if len(args) == 0 {
		exitWithError(messages.Text(messages.H8605, command, "egit.tr"))
	}
	return s, args[0], args[1:]
}

// prepareProgram dosyayı h hazırlığıyla hazırlar ve bir VM kurar (çalıştırma, profil, izleme ve
// hata ayıklama aynı adımları kullanır).
func prepareProgram(path string, arguments []string, h preparation) (*interp.VM, *ast.Program, string) {
	_, s := prepareFile(path, h)
	machine := interp.New()
	machine.SourceFile = path
	machine.SourceCode = s.Text
	machine.SetArgs(arguments)
	return machine, s.Program, s.Text
}

// warningWriter profil uyarılarını programın çıktı noktasından (canlı satırla eşgüdümlü)
// standart hataya yazar.
func warningWriter(machine *interp.VM) func(string) {
	return func(m string) { machine.WriteOutput(os.Stderr, m+"\n") }
}

// startMeasuredProfile tr izle ve tr profil'in profilini başlatır: satır örneklemesi ve GPU
// çekirdek ölçümü açık; arka plan işiyse durum dosyası da yazılır. GPU'nun son ölçümleri
// motorlar kapanmadan alınır (beforeClose).
func startMeasuredProfile(machine *interp.VM, path, code string, prog *ast.Program) *profiler.Profile {
	p := profiler.Start(machine, profiler.Options{
		Program: path, Source: code, SampleLines: true, GPUMeasure: true,
		StatusFile: profiler.JobStatusFile(), Warning: warningWriter(machine),
	})
	beforeClose = append(beforeClose, p.BeforeClose)
	return p
}

// profileCommand 'tr profil [--json yol] [--go-profili dosya] program.tr [argümanlar]':
// programı satır örneklemesiyle çalıştırır, bitince (hata ya da kesmeyle bitse de) metin
// raporu standart çıktıya, istenmişse JSON raporu dosyaya yazar.
func profileCommand(args []string) {
	s, path, arguments := profileArguments(codes.DisplayName(codes.K013), args, codes.K103, codes.K104)
	machine, prog, code := prepareProgram(path, arguments, runPreparation)
	if s.goProfile != "" {
		startGoProfile(s.goProfile)
	}
	p := startMeasuredProfile(machine, path, code, prog)
	exitCleanup = append(exitCleanup, func() {
		end := p.Finish(exitCode)
		fmt.Print("\n" + profiler.ReportText(end))
		if s.json != "" {
			writeJSONReport(s.json, end)
		}
	})
	runProgram(machine, prog, code)
	cleanup()
}

// writeJSONReport raporu yola (klasörse içine speed.json) yazar; yazılamazsa bildirir.
func writeJSONReport(filePath string, g *profiler.Snapshot) {
	if b, err := os.Stat(filePath); (err == nil && b.IsDir()) || strings.HasSuffix(filePath, "/") || strings.HasSuffix(filePath, string(os.PathSeparator)) {
		if err := os.MkdirAll(filePath, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "⚠️ "+messages.Text(messages.H8219, messages.New(messages.H8676, filePath, err)))
			return
		}
		filePath = filepath.Join(filePath, profiler.ReportFileName)
	}
	if err := profiler.WriteFile(filePath, g, func(m string) { fmt.Fprintln(os.Stderr, m) }); err != nil {
		fmt.Fprintln(os.Stderr, "⚠️ "+messages.Text(messages.H8219, messages.New(messages.H8677, err)))
		return
	}
	fmt.Fprintln(os.Stderr, messages.Text(messages.H8678, filePath))
}

// startGoProfile yorumlayıcının kendi Go CPU profilini yazmaya başlar (geliştiriciler
// için; 'go tool pprof' ile okunur).
func startGoProfile(filePath string) {
	f, err := os.Create(filePath)
	if err != nil {
		exitWithError(messages.Text(messages.H8679, err))
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		exitWithError(messages.Text(messages.H8680, err))
	}
	exitCleanup = append(exitCleanup, func() {
		pprof.StopCPUProfile()
		f.Close()
	})
}

// startJobProfile program bir arka plan işi olarak çalışıyorsa (TR_IS_KLASORU) iş klasörüne
// saniyede bir profil.json yazar (yalnızca her zaman açık sayaçlar; satır örneklemesi
// yok: ek maliyet saniyede bir dosya yazmasıdır). İş değilse hiçbir şey yapmaz.
func startJobProfile(machine *interp.VM, path, code string, prog *ast.Program) {
	filePath := profiler.JobStatusFile()
	if filePath == "" {
		return
	}
	p := profiler.Start(machine, profiler.Options{
		Program: path, Source: code, StatusFile: filePath, Warning: warningWriter(machine),
	})
	exitCleanup = append(exitCleanup, func() { p.Finish(exitCode) })
}
