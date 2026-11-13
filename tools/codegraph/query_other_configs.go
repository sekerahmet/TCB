package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Bu dosya sorulan adın grafiğin tiplerle okumadığı (bu yapılandırmada derlenmeyen) dosyalarda geçtiği yerleri bulur
// (other_os.go o dosyaları grafiğe tipsiz okur).

// otherConfigMention sorulan adın, grafiğin tiplerle okumadığı (bu yapılandırmada derlenmeyen) bir dosyada geçtiği yerdir.
type otherConfigMention struct {
	file string
	line int
	in   *Node // geçtiği bildirim (dosyada ondan önce başlayan son bildirim); yoksa nil
}

// otherConfigMentions n'nin kısa adının derlenmeyen dosyalarda sözcük olarak geçtiği yerlerdir (metin olarak okunur,
// tiplerle değil). Grafik oradaki çağrıları ve tür kullanımlarını görmez; cevap bu yerleri ayrıca söyler (DENETİM
// doğrulaması Y4: dataload.mmapFile'ın Linux tanımı ve onu sınayan TestMmapTruncatedDoesNotCrash görünmüyordu, "25 files"
// notu ise ilgisiz boş cevaplara da düşüyordu).
func (ix *index) otherConfigMentions(n *Node) (config string, out []otherConfigMention) {
	if !strings.HasPrefix(n.ID, "go:") || n.Kind == kindFile || n.Kind == kindPackage || ix.root == "" {
		return "", nil // dosya ve paket adı bir bildirim adı değildir ("text.go"nun son parçası "go" olurdu)
	}
	name := n.Name[strings.LastIndex(n.Name, ".")+1:]
	for i := range ix.g.Nodes {
		f := &ix.g.Nodes[i]
		if f.Kind != kindFile || !strings.HasPrefix(f.Detail, otherConfigDetail) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(f.Name)))
		if err != nil || !strings.Contains(string(data), name) {
			continue
		}
		word, ok := ix.otherConfigWord(n, f.Name, data, name)
		if !ok {
			continue
		}
		config = strings.TrimPrefix(f.Detail, otherConfigDetail)
		type decl struct {
			line int
			node *Node
		}
		var decls []decl
		for _, e := range ix.out[f.ID] {
			if e.Kind == edgeContains && ix.byID[e.To] != nil {
				decls = append(decls, decl{e.Line, ix.byID[e.To]})
			}
		}
		sort.Slice(decls, func(a, b int) bool { return decls[a].line < decls[b].line })
		fw := foldText(word) // sözcük bir kez katlanır, her satır bir kez
		for no, line := range strings.Split(string(data), "\n") {
			if !foldText(line).hasWord(fw) || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue // yorum satırı kullanım değildir
			}
			m := otherConfigMention{file: f.Name, line: no + 1}
			for _, d := range decls {
				if d.line <= no+1 {
					m.in = d.node
				}
			}
			out = append(out, m)
		}
	}
	return config, out
}

// otherConfigWord n'nin derlenmeyen file dosyasında aranacak yazımıdır. Ad kendi paketinin dosyasında kısa adıyla geçer;
// başka paketin dosyasında yalnız dışa açık ad geçebilir ve yalnız dosya n'nin paketini içe aktarıyorsa: paket düzeyi
// bildirim niteleyicisiyle (`paket.Ad`, takma adla da), üye (alan, metot) kısa adıyla. ok false ise n o dosyada geçemez
// (DENETİM 358 Ş1: Linux'ta impact.load, sysmem'deki memState.load alanıyla eşleşiyordu).
func (ix *index) otherConfigWord(n *Node, file string, data []byte, name string) (word string, ok bool) {
	home := ix.filePackage(n.File)
	if n.File == "" || home == "" || ix.filePackage(file) == home {
		return name, true
	}
	if !token.IsExported(name) {
		return "", false
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, data, parser.ImportsOnly)
	if err != nil {
		return "", false
	}
	for local, p := range packageImports(f) {
		if p != home {
			continue
		}
		if local == "." || strings.Count(n.Name, ".") > 1 {
			return name, true // nokta ile içe aktarma ya da üye: niteleyici paket adı değildir
		}
		return local + "." + name, true
	}
	return "", false
}

// filePackage dosyanın Go paketinin yoludur (grafikte paketi dosyayı içerir); bilinmiyorsa boş.
func (ix *index) filePackage(file string) string {
	for _, e := range ix.in[fileID(file)] {
		if e.Kind == edgeContains && strings.HasPrefix(e.From, "go:pkg:") {
			return strings.TrimPrefix(e.From, "go:pkg:")
		}
	}
	return ""
}

// printOtherConfigs sorulan adın derlenmeyen dosyalarda geçtiği yerleri bildirime göre tek satırda yazar (testler
// işaretli); geçmiyorsa bir şey yazmaz.
func (ix *index) printOtherConfigs(w io.Writer, n *Node, limit int) {
	ix.printOtherConfigsExcept(w, n, limit, nil)
}

// printOtherConfigsExcept printOtherConfigs'tir; listed'deki testler (tests-for'un gruplarında yazılanlar) burada bir
// kez daha yazılmaz (bağımsız denetim grup 7: derlenmeyen dosyadaki test iki grupta çıkıyordu).
func (ix *index) printOtherConfigsExcept(w io.Writer, n *Node, limit int, listed []*Node) {
	config, ms := ix.otherConfigMentions(n)
	if len(ms) == 0 {
		return
	}
	var lines []string
	seen := map[string]bool{}
	for _, m := range ms {
		where := fmt.Sprintf("%s:%d", m.file, m.line)
		if m.in != nil && slices.Contains(listed, m.in) {
			continue
		}
		if m.in != nil {
			if seen[m.in.ID+m.file] {
				continue
			}
			seen[m.in.ID+m.file] = true
			where = fmt.Sprintf("%s [%s] at %s", m.in.Name, m.in.Kind, where)
			if isTestNode(&Node{Kind: m.in.Kind, Name: m.in.Name, File: m.file}) {
				where += "  [test]"
			}
		}
		lines = append(lines, where)
	}
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- in other build configurations (%d; files not built for %s, read as text, not type-checked):\n", len(lines), config)
	printLimited(w, lines, limit, "    ")
}
