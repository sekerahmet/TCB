package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Bu dosya tests-for'un alt süreç testlerini ve test edilen ikiliye bağlanan paketleri yazar (subprocess.go'nun
// builds kenarlarından; bağımsız denetim Y5).

// subprocessTests n'nin paketini derleyip ikilisini koşan testlerdir, paket klasörüne göre gruplu. Derleyen TestMain ise
// onun paketindeki bütün testler (ikiliyi hepsi paylaşır); başka bir işlevse onu çağıran ilk testler (zincir kuralı).
func (ix *index) subprocessTests(n *Node) map[string][]*Node {
	return ix.subprocessTestsOf(ix.goPackageOf(n))
}

// subprocessTestsOf subprocessTests'tir, paket kimliğiyle.
func (ix *index) subprocessTestsOf(pkg string) map[string][]*Node {
	var builders []*Node
	for _, e := range ix.in[pkg] {
		if e.Kind == edgeBuilds && ix.byID[e.From] != nil {
			builders = append(builders, ix.byID[e.From])
		}
	}
	out := map[string][]*Node{}
	seen := map[string]bool{}
	add := func(t *Node) {
		if !seen[t.ID] && !isMeasurement(t) {
			seen[t.ID] = true
			out[path.Dir(t.File)] = append(out[path.Dir(t.File)], t)
		}
	}
	for _, b := range builders {
		if strings.HasSuffix(b.Name, ".TestMain") {
			for _, t := range ix.testsInDir(path.Dir(b.File)) {
				if t.ID != b.ID {
					add(t)
				}
			}
			continue
		}
		walkCallers(b.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
			if t := ix.byID[r.ID]; isTestNode(t) {
				add(t)
				return false
			}
			return true
		})
	}
	return out
}

// goPackageOf düğümün dosyasını içeren Go paketi düğümünün kimliğidir (paket → dosya `contains` kenarı); yoksa "".
func (ix *index) goPackageOf(n *Node) string {
	for _, e := range ix.in[fileID(n.File)] {
		if e.Kind == edgeContains && strings.HasPrefix(e.From, "go:pkg:") {
			return e.From
		}
	}
	return ""
}

// testsInDir klasördeki Go testleridir.
func (ix *index) testsInDir(dir string) []*Node {
	var out []*Node
	for i := range ix.g.Nodes {
		if t := &ix.g.Nodes[i]; path.Dir(t.File) == dir && isTestNode(t) {
			out = append(out, t)
		}
	}
	return out
}

// printSubprocessTests alt süreç testlerini klasör başına tek satırda sayar.
func (ix *index) printSubprocessTests(w io.Writer, byDir map[string][]*Node) {
	if len(byDir) == 0 {
		return
	}
	dirs := make([]string, 0, len(byDir))
	total := 0
	for d, ts := range byDir {
		dirs = append(dirs, fmt.Sprintf("%s (%d)", d, len(ts)))
		total += len(ts)
	}
	sort.Strings(dirs)
	fmt.Fprintf(w, "  <- covered by subprocess tests (%d; they build this package and run the binary, not traced per function): %s\n", total, strings.Join(dirs, ", "))
}

// printLinkedBinaries n'nin paketi (geçişli içe aktarmayla) bir testin derleyip koştuğu ana pakete bağlıysa bunu zayıf
// kanıt olarak yazar ve true verir: ikiliyi koşan testler bu koda varabilir, ama hangisinin vardığı izlenmez (DENETİM
// doğrulaması Y5: codeindex.LoadCached, e2e'deki --no-index testine rağmen "no tests found" diyordu). Paketin kendisi
// derleniyorsa satır zaten "covered by subprocess tests"tir; burada yalnızca bağlanan paketler.
func (ix *index) printLinkedBinaries(w io.Writer, n *Node) bool {
	pkg := ix.goPackageOf(n)
	if pkg == "" {
		return false
	}
	var parts []string
	total := 0
	for _, bin := range ix.builtPackages() {
		if bin == pkg || !ix.importsTransitively(bin, pkg) {
			continue
		}
		byDir := ix.subprocessTestsOf(bin)
		count := 0
		for _, ts := range byDir {
			count += len(ts)
		}
		total += count
		parts = append(parts, fmt.Sprintf("%s (%d tests run it)", ix.byID[bin].Name, count))
	}
	if len(parts) == 0 {
		return false
	}
	sort.Strings(parts)
	fmt.Fprintln(w, "  no tests traced to it")
	fmt.Fprintf(w, "  <- weak evidence (%d): linked into the binary of %s; a subprocess test may reach this code, which one is not traced\n", total, strings.Join(parts, ", "))
	return true
}

// builtPackages bir testin `go build` ile derlediği paketlerdir (builds kenarının hedefi), sıralı.
func (ix *index) builtPackages() []string {
	seen := map[string]bool{}
	var out []string
	for i := range ix.g.Edges {
		if e := &ix.g.Edges[i]; e.Kind == edgeBuilds && !seen[e.To] {
			seen[e.To] = true
			out = append(out, e.To)
		}
	}
	sort.Strings(out)
	return out
}

// importsTransitively from paketi to paketini (doğrudan ya da dolaylı) içe aktarıyor mu.
func (ix *index) importsTransitively(from, to string) bool {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range ix.out[cur] {
			if e.Kind != edgeImports || seen[e.To] {
				continue
			}
			if e.To == to {
				return true
			}
			seen[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return false
}
