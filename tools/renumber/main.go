// renumber dil tanımı maddelerini yeniden numaralar: eski→yeni eşleme dosyasından önce bir plan (değişecek
// her anma, dosya:satır) yazar, `--apply` ile uygular. Tanım satırları (`**15.31** ...`), açık atıflar
// (`**N.M**`, `dil tanımı N.M`, `§N.M`, `[N.M](…spec/…)`) ve spec içi parantez anmaları değişir; numaranın
// ardındaki Türkçe ek (`15.9'daki`) yeni okunuşa uydurulur; "15.31–15.38" gibi aralıklar yeni numaralar
// ardışık değilse parçalanır ("15.11–15.15, 15.20–15.22"). Her düzenleme özgün konumdan hesaplanır:
// zincirleme olmaz, 15.0 gibi madde olmayan yazılara dokunulmaz.
//
// Usage (from the repository root):
//
//	go run ./tools/renumber --map moves.txt            // the plan only
//	go run ./tools/renumber --map moves.txt --apply    // write the files
//	go run ./tools/renumber --suffixes [--apply]       // only fix suffixes that break vowel harmony
//
// The map file has one "old new" pair per line (`#` starts a comment; `->` and `→` are accepted).
// codegraph stays read-only; this is the separate tool that writes (decision: kod-grafigi-21 note).
//
// Layer: tools (tools); not shipped to users.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sekerahmet/TCB/tools/internal/repodir"
	"github.com/sekerahmet/TCB/tools/internal/specref"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run komutu çalıştırır; çıkış kodu 0 başarı, 1 hata, 2 yanlış kullanım.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("renumber", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mapPath := fs.String("map", "", "file with one \"old new\" item pair per line")
	root := fs.String("root", "", "repository root (default: found from the current directory)")
	apply := fs.Bool("apply", false, "write the changes (without it only the plan is printed)")
	suffixes := fs.Bool("suffixes", false, "also fix every suffix that breaks vowel harmony, not only the renumbered ones")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mapPath == "" && !*suffixes || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: renumber (--map <file> | --suffixes) [--root <dir>] [--apply]")
		return 2
	}
	dir := *root
	if dir == "" {
		dir = repodir.Find()
	}
	mapping := map[string]string{}
	if *mapPath != "" {
		var err error
		if mapping, err = specref.ParseMapping(*mapPath); err != nil {
			fmt.Fprintln(stderr, "renumber:", err)
			return 1
		}
	}
	files, err := specref.Collect(dir)
	if err != nil {
		fmt.Fprintln(stderr, "renumber:", err)
		return 1
	}
	ix := specref.BuildIndex(files)
	if err := specref.Validate(mapping, ix); err != nil {
		fmt.Fprintln(stderr, "renumber:", err)
		return 1
	}
	edits, manual := specref.Plan(specref.Scan(files, ix), files, ix, mapping, *suffixes)
	for _, e := range edits {
		fmt.Fprintf(stdout, "%s:%d  %s → %s  [%s]\n", e.File, e.Line, e.Old, e.New, e.Why)
	}
	for _, m := range manual {
		fmt.Fprintf(stderr, "%s:%d  needs a manual edit (range that cannot be mapped automatically)\n", m.File, m.Line)
	}
	fmt.Fprintf(stdout, "%d edit(s), %d manual\n", len(edits), len(manual))
	if len(manual) > 0 {
		return 1
	}
	if !*apply {
		return 0
	}
	if err := specref.Apply(dir, edits); err != nil {
		fmt.Fprintln(stderr, "renumber:", err)
		return 1
	}
	fmt.Fprintln(stdout, "applied")
	return 0
}
