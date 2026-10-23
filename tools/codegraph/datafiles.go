package main

import (
	"path"
	"path/filepath"
	"sort"
)

// Bu dosya veri dosyalarını (JSON) düğüm yapar. Gömülen ya da okunan bir veri dosyası (ad ve ileti listeleri,
// tanım tabloları) kodun parçası gibi sınanır: bütün listeyi gezen testler her girdiyi korur. Dosyanın düğümü
// olmazsa `tests-for tr.json` "not found" der; düğüm olunca gömen değişkenden (`embeds`), okuyan işlevden
// (`reads_file`) ve onları çağıran testlerden (`tests`) gelen kenarlar ona bağlanır (test_paths.go). Dil
// dosyalarındaki her ad ve ileti kodu dosyasının düğümünden `contains` kenarı alır; böylece `tests-for T720`
// kodun tanımlandığı dosyayı geçen testleri de gösterir.

// extractDataFiles root altındaki JSON dosyalarını (testdata hariç) `file` düğümü yapar ve ad/ileti dil
// dosyalarındaki kodların düğümlerini dosyaya bağlar.
func extractDataFiles(root string, b *builder) {
	for _, rel := range walkFiles(root, ".json") {
		b.addNode(Node{ID: "file:" + rel, Kind: kindFile, Name: rel, File: rel, Line: 1, Detail: "data"})
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
