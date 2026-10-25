package main

import (
	"path"
	"path/filepath"
	"sort"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Bu dosya izlenen her dosyayı ve veri dosyalarını (JSON) düğüm yapar. Gömülen ya da okunan bir veri dosyası (ad ve ileti listeleri,
// tanım tabloları) kodun parçası gibi sınanır: bütün listeyi gezen testler her girdiyi korur. Dosyanın düğümü
// olmazsa `tests-for tr.json` "not found" der; düğüm olunca gömen değişkenden (`embeds`), okuyan işlevden
// (`reads_file`) ve onları çağıran testlerden (`tests`) gelen kenarlar ona bağlanır (test_paths.go). Dil
// dosyalarındaki her ad ve ileti kodu dosyasının düğümünden `contains` kenarı alır; böylece `tests-for TR_fn_default_device`
// kodun tanımlandığı dosyayı geçen testleri de gösterir.

// extractDataFiles git'in izlediği ve başka bir çıkarıcının düğüm yapmadığı her dosyayı (betik, .gitignore, JS, YAML,
// testdata fikstürü, JSON; JSON'un ayrıntısı "data") `file` düğümü yapar ve ad/ileti dil dosyalarındaki kodların
// düğümlerini dosyaya bağlar. İzlenen bir dosyanın yolu böylece hep o dosyaya çözülür: düğümü olmayan yol ad aramasına
// düşüp yolu metninde anan bildirimleri "belirsiz" diye veriyordu (`tests-for tools/full_check.sh`; TCB 2026-10-25,
// 1. ve 4. madde). Kural: TestEveryTrackedFileIsANode.
func extractDataFiles(root string, b *builder) {
	has := map[string]bool{}
	for _, n := range b.nodes {
		if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
			has[n.File] = true
		}
	}
	rels, ok := codeindex.GitTracked(root)
	if !ok {
		rels = walkFiles(root, "")
	}
	for _, rel := range rels {
		if !has[rel] {
			detail := ""
			if path.Ext(rel) == ".json" {
				detail = "data"
			}
			b.addNode(Node{ID: "file:" + rel, Kind: kindFile, Name: rel, File: rel, Line: 1, Detail: detail})
		}
	}
	for _, dir := range []struct{ rel, prefix string }{{codesDir, "name:"}, {messagesDir, "msg:"}} {
		files, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir.rel), "*.json"))
		sort.Strings(files)
		for _, f := range files {
			rel := dir.rel + "/" + path.Base(filepath.ToSlash(f))
			lines := jsonKeyLines(f)
			for code, line := range lines {
				b.addEdge("file:"+rel, dir.prefix+code, edgeContains, extracted, rel, line)
			}
		}
	}
}
