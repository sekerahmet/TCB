package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// goneCode geçmişte sözlükte bulunup bugün olmayan bir koddur.
type goneCode struct {
	code    string
	removed commitInfo // silindiği (en son silen) commit
	added   commitInfo // ilk eklendiği commit (bilinmiyorsa boş)
}

// codeFamily bir sözlük dosyaları kümesi ve adıdır.
type codeFamily struct {
	name  string
	files []string
}

var families = []codeFamily{{"messages", messageFiles}, {"names", nameFiles}}

// runDeleted `deleted` komutudur: grafik gerekmez, git geçmişi ve bugünkü sözlük dosyaları yeter.
func runDeleted(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deleted", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "repository root (default: found from the current directory)")
	retired := fs.Bool("retired-json", false, "print the deleted codes of --family as retired.json content")
	family := fs.String("family", "messages", "family for --retired-json: messages or names")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	abs, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	if *retired {
		return printRetired(abs, *family, stdout, stderr)
	}
	for _, f := range families {
		gone, redefined, err := deletedCodes(abs, f)
		if err != nil {
			fmt.Fprintln(stderr, "codegraph:", err)
			return exitFail
		}
		fmt.Fprintf(stdout, "%s: %d deleted codes\n", f.name, len(gone))
		for _, g := range gone {
			fmt.Fprintf(stdout, "  %s  removed %s  (added %s)\n", g.code, g.removed, orUnknown(g.added))
		}
		if len(redefined) > 0 {
			fmt.Fprintf(stdout, "%s: %d codes defined again after being deleted (a number must not be reused):\n", f.name, len(redefined))
			for _, g := range redefined {
				fmt.Fprintf(stdout, "  %s  removed %s\n", g.code, g.removed)
			}
		}
	}
	return exitOK
}

func orUnknown(c commitInfo) string {
	if c.hash == "" {
		return "before the visible history"
	}
	return c.String()
}

// printRetired deleted kodları retired.json biçiminde yazar: numara -> {commit, reason}.
func printRetired(root, family string, stdout, stderr io.Writer) int {
	for _, f := range families {
		if f.name != family {
			continue
		}
		gone, _, err := deletedCodes(root, f)
		if err != nil {
			fmt.Fprintln(stderr, "codegraph:", err)
			return exitFail
		}
		type entry struct {
			Commit string `json:"commit"`
			Reason string `json:"reason"`
		}
		out := map[string]entry{}
		for _, g := range gone {
			out[g.code] = entry{g.removed.hash, "deleted: " + g.removed.subject}
		}
		data, _ := json.MarshalIndent(out, "", " ")
		fmt.Fprintf(stdout, "%s\n", data)
		return exitOK
	}
	fmt.Fprintf(stderr, "unknown family %q (messages or names)\n", family)
	return exitUsage
}

// currentCodes sözlük dosyalarının bugünkü kodlarını verir (yeni sözlük dosyası yoksa boş).
func currentCodes(root string, files []string) map[string]bool {
	cur := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) == nil {
			for k := range m {
				cur[k] = true
			}
		}
	}
	return cur
}

// deletedCodes ailenin geçmişte silinmiş ve bugün hiçbir dosyada olmayan kodlarını (kod
// sırasıyla), ayrıca silindikten sonra yeniden tanımlanmış kodları verir. Yeniden adlandırılan
// kod silinmiş değildir: geçmişi bugünkü anahtarında birleşir (renames.go).
func deletedCodes(root string, f codeFamily) (gone, redefined []goneCode, err error) {
	events, err := scanCodeHistory(root, f.files)
	if err != nil {
		return nil, nil, err
	}
	events = followRenames(events, readRenames(root))
	cur := currentCodes(root, f.files)
	codes := make([]string, 0, len(events))
	for c := range events {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		list := events[c] // yeniden eskiye
		var g goneCode
		g.code = c
		for _, e := range list {
			if e.removed && g.removed.hash == "" {
				g.removed = e.commit
			}
			if e.added {
				g.added = e.commit // sonuncusu en eskisidir
			}
		}
		switch {
		case g.removed.hash == "":
		case !cur[c]:
			gone = append(gone, g)
		case redefinedAfterRemoval(list):
			redefined = append(redefined, g)
		}
	}
	return gone, redefined, nil
}

// redefinedAfterRemoval aynı dosyada bir silmeden SONRA yeniden eklenme var mı (liste yeniden
// eskiye: daha eskideki silme, daha yenideki eklemeden sonra gelir).
func redefinedAfterRemoval(list []codeEvent) bool {
	addedAfter := map[string]bool{}
	for _, e := range list {
		switch {
		case e.added:
			addedAfter[e.file] = true
		case e.removed && addedAfter[e.file]:
			return true
		}
	}
	return false
}
