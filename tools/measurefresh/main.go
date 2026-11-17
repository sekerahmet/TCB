// measurefresh ölçüler klasörünün (docs/measurements/) satırlarının tazeliğini söyler: her satırın
// commit'inden bu yana origin/main'e giren ve satırın yollarına (README'nin kümeleri, ölçülen .tr
// programları ve özyinelemeli aktardıkları) dokunan commit'leri sayar, depo dışı sürümleri
// README'nin tablosuyla ve go.mod'un toolchain satırıyla karşılaştırır, "Yerini alan"ı doluysa
// yenisini söyler, boşsa aynı dosyada, bölümde, modelde ve programda daha yeni bir satırı olası
// yerini alan olarak önerir.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/measurefresh [kimlik ...] [--refs dosya]
//
// Kimlik verilmezse bütün satırlar; --refs dosyadaki kimlik sözlerini de alır ve klasörde
// olmayanları satırıyla yazar. Çıkış kodu bulunamayan kimlikte ve kırık göndermede 1, kullanım
// ya da git hatasında 2; eski satır ve olası yerini alan uyarıdır (0). İçe aktarmalar origin/main'de
// çözülür; satırlar çalışılan ağaçtan okunur. CI'da koşmaz: biçim ve gönderme testleri
// (check_test.go) yalnız çalışılan ağaca bakar.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

func main() {
	os.Exit(run(os.Args[1:], reporoot.Root(), os.Stdout, os.Stderr))
}

// run komutun kendisidir; root depo köküdür.
func run(args []string, root string, stdout, stderr io.Writer) int {
	var ids []string
	refs := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--refs" && i+1 < len(args) && refs == "":
			refs = args[i+1]
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(stderr, raw("usage"))
			return 2
		default:
			ids = append(ids, a)
		}
	}
	f, err := Load(root)
	if err != nil {
		fmt.Fprintln(stderr, "measurefresh:", err)
		return 2
	}
	ids, code, err := wanted(f, ids, refs, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "measurefresh:", err)
		return 2
	}
	g := repo{root}
	head, err := g.git("log", "-1", "--format=%h %cs", compareRef)
	if err != nil {
		fmt.Fprintln(stderr, "measurefresh:", err)
		return 2
	}
	hash, day, _ := strings.Cut(head, " ")
	fmt.Fprintln(stdout, text("outCompared", compareRef, hash, day))
	goVer, err := g.goVersion()
	if err != nil {
		fmt.Fprintln(stderr, "measurefresh:", err)
		return 2
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		r, ok := f.rowByID(id)
		if !ok {
			fmt.Fprintln(stdout, text("outNotInFolder", id))
			code = 1
			continue
		}
		line, err := freshness(f, g, goVer, r)
		if err != nil {
			fmt.Fprintln(stderr, "measurefresh:", err)
			return 2
		}
		fmt.Fprintln(stdout, line)
		if s := possibleReplacement(f, r); s != "" {
			fmt.Fprintln(stdout, text("outPossible", id, s))
		}
	}
	return code
}

// wanted bakılacak kimliklerdir: verilenler ve refs dosyasının klasörde olan kimlik sözleri;
// ikisi de yoksa bütün satırlar. Klasörde olmayan gönderme satırıyla yazılır ve kodu 1 yapar.
func wanted(f *Folder, ids []string, refs string, stdout io.Writer) ([]string, int, error) {
	code := 0
	if refs == "" {
		if len(ids) == 0 {
			for _, r := range f.Rows {
				ids = append(ids, r.ID())
			}
		}
		return ids, code, nil
	}
	data, err := os.ReadFile(refs)
	if err != nil {
		return nil, 0, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		for _, id := range idMention.FindAllString(line, -1) {
			if _, ok := f.rowByID(id); !ok {
				fmt.Fprintln(stdout, text("outBrokenRef", refs, i+1, id))
				code = 1
			} else {
				ids = append(ids, id)
			}
		}
	}
	return ids, code, nil
}
