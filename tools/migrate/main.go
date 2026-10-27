// migrate çevirir: depodaki TR metnini, dil tanımının 2. bölümü (sözcük yapısı) yeniden yazılmadan
// önceki biçimden yenisine. Eski biçim: ilk satır "//! tr kodlu 3 dil=xx", sınıflı kimlikler
// (TR_fn_print), '//' ve '/* */' yorumu, '{x}' yerleştirmesi, tek tırnaklı metin, çıplak ileti kodu,
// TR_kw_while ve TR_kw_default. Yeni biçim: ilk satır "# TR 4", TR_print, '#' yorumu, '\(x)', yalnız
// çift tırnak (docs/spec/02_sozcuk_yapisi.md).
//
// Girdiler (izlenen dosyalar): .tr dosyaları; .md belgelerindeki ```tr blokları; Go testlerindeki TR
// metni (go_strings.go); internal/lang/codes/tr.json ve en.json'daki "example" alanları (gösterim
// metni, yalnız sözdizimi). Başlıksız metin Türkçe gösterimdir: önce yazımı çevrilir, sonra bugünkü
// kodlama yoluyla (internal/lang/source) kimliğe çevrilir. Yeni biçimdeki girdiye dokunulmaz: iki kez
// koşmak bir kez koşmakla aynıdır. Çevrilemeyen her yer (dosya:satır, neden) listelenir.
//
// Usage (from the repository root):
//
//	go run ./tools/migrate                 // the plan: files that would change, places it cannot convert
//	go run ./tools/migrate --apply         // write the files
//	go run ./tools/migrate --apply a.tr b  // only these paths (files or folders)
//
// Layer: tools (tools); not shipped to users.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/tools/internal/repodir"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// skipped çevrilmeyen yollardır: dil tanımı ve durum belgeleri ana oturumundur (orada çevirici
// ayrıca koşulur), arşiv geçmiştir, codes.md tr.json'dan üretilir.
var skipped = []string{"docs/spec/", "docs/status.md", "docs/roadmap.md", "docs/archive/", "docs/codes.md"}

// run komutu çalıştırır; çıkış kodu 0 başarı, 1 çevrilemeyen yer ya da hata, 2 yanlış kullanım.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apply := fs.Bool("apply", false, "write the converted files (without it only the plan is printed)")
	rootFlag := fs.String("root", "", "repository root (default: found from the current directory)")
	outDir := fs.String("out", "", "write the converted files under this folder instead of in place (for review)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, err := repodir.Resolve(*rootFlag)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	files, err := trackedFiles(root, fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	changed, failed := 0, 0
	for _, rel := range files {
		res, ok := convertFile(root, rel)
		if !ok {
			continue
		}
		for _, p := range res.problems {
			fmt.Fprintf(stdout, "cannot convert %s:%d: %s\n", rel, p.Line, p.Reason)
			failed++
		}
		if res.out == res.in {
			continue
		}
		changed++
		fmt.Fprintf(stdout, "convert %s\n", rel)
		target := ""
		switch {
		case *outDir != "":
			target = filepath.Join(*outDir, rel)
			err = os.MkdirAll(filepath.Dir(target), 0o755)
		case *apply:
			target = filepath.Join(root, rel)
		}
		if target != "" && err == nil {
			err = os.WriteFile(target, []byte(res.out), 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "%d files to convert, %d places it cannot convert\n", changed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// result bir dosyanın çevirisidir.
type result struct {
	in, out  string
	problems []problem
}

// convertFile dosyayı türüne göre çevirir; ok, dosyanın çevirinin bir girdisi olduğudur.
func convertFile(root, rel string) (result, bool) {
	slash := filepath.ToSlash(rel)
	for _, s := range skipped {
		if strings.HasPrefix(slash, s) {
			return result{}, false
		}
	}
	base := filepath.Base(slash)
	var conv func(string, string) (string, []problem)
	switch {
	case strings.HasSuffix(base, ".tr"):
		conv = convertTRFile
	case strings.HasSuffix(base, ".md"):
		conv = convertMarkdown
	case strings.HasSuffix(base, "_test.go"):
		conv = convertGoTest
	case slash == "internal/lang/codes/tr.json" || slash == "internal/lang/codes/en.json":
		conv = convertExamples
	default:
		return result{}, false
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return result{problems: []problem{{Reason: err.Error()}}}, true
	}
	in := string(data)
	out, probs := conv(slash, in)
	return result{in: in, out: out, problems: probs}, true
}

// trackedFiles git'in izlediği dosyalardır (paths verilirse yalnız onların altındakiler), sıralı.
func trackedFiles(root string, paths []string) ([]string, error) {
	cmd := exec.Command("git", append([]string{"ls-files", "-z", "--"}, paths...)...)
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %v", err)
	}
	var files []string
	for _, f := range strings.Split(string(b), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}
