package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Planlama sınavı (testdata/planning_questions.txt) soruları metin olarak taşır; sınavı koşan test andığı işlevleri
// çağırmadığı için çağrı zinciri sınava ulaşmaz. Bu dosya sınavda anılan düğümlere examTest'ten names_in_exam kenarı
// kurar ve yeniden adlandırma/silme farklarında silinen satırlardaki sınav sözcüklerini yakalar.
const examFile, examTest, examSkip = "tools/codegraph/testdata/planning_questions.txt", "TestPlanningQuestionsAreAnswered", " codegraph trlang index contains: not: exit: first-item: group: first-of: top: before: "

var (
	examLineRef = regexp.MustCompile(`@\{([^#}]+)#[^}]+\}`)
	examSymRe   = regexp.MustCompile(`^(?:TR_|[HKGA]\d+$)|\.|_`)
	examIdent   = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*(?:\.[\p{L}_][\p{L}\p{N}_]*)*`)
	examRepl    = strings.NewReplacer(" | ", " ", " || ", " ", " => ", " ")
	examKinds   = map[string]bool{kindFunc: true, kindMethod: true, kindType: true, kindConst: true, kindVar: true, kindField: true, kindMessage: true, kindName: true}
)

// examNodeKeys düğümün sınav sözcükleriyle eşleşen ad ya da yol anahtarlarıdır (başlık, paket ve bölüm numarası hariç).
func examNodeKeys(n *Node) []string {
	if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
		return []string{n.File}
	}
	if n.Name == "" || (!examKinds[n.Kind] && (!strings.HasPrefix(n.Kind, "tr_") || n.Kind == kindTRTest)) {
		return nil
	}
	keys := []string{n.Name}
	for _, s, ok := strings.Cut(n.Name, "."); ok && strings.Contains(s, "."); _, s, ok = strings.Cut(s, ".") {
		keys = append(keys, s)
	}
	return keys
}

// parseExamWords sınav dosyasının her satırındaki komut ve beklenti sözcüklerini okur.
func parseExamWords(root string) (out [][]string) {
	data, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(examFile)))
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		var row []string
		if line, _, _ = strings.Cut(strings.TrimSpace(line), " | src: "); line != "" && !strings.HasPrefix(line, "#") {
			for _, w := range strings.Fields(examRepl.Replace(examLineRef.ReplaceAllString(line, " $1 "))) {
				if t := strings.Trim(w, ",:;()[]`\""); !strings.Contains(examSkip, " "+w+" ") {
					row = append(row, slices.Compact([]string{w, t})...)
				}
			}
		}
		out = append(out, row)
	}
	return out
}

// linkExam sınavın her sorusunda adı geçen düğüme yalnız examTest'ten names_in_exam kenarı çeker.
func linkExam(root string, b *builder) {
	if test := "go:func:" + b.module + "/tools/codegraph." + examTest; b.nodes[test] != nil {
		byKey := map[string][]string{}
		for id, n := range b.nodes {
			for _, k := range examNodeKeys(n) {
				byKey[k] = append(byKey[k], id)
			}
		}
		for i, words := range parseExamWords(root) {
			for _, w := range words {
				for _, id := range byKey[w] {
					if id != test {
						b.addEdge(test, id, edgeNamesInExam, inferred, examFile, i+1)
					}
				}
			}
		}
	}
}

// examEdge verilen düğümlerdeki doğrudan listelenmemiş sınav kenarlarını tek kenarda birleştirir.
func (ix *index) examEdge(listed []*Node, all map[string]int, nodes ...*Node) (out *Edge) {
	for _, n := range nodes {
		for _, e := range ix.in[n.ID] {
			if _, ok := all[e.From]; e.Kind == edgeNamesInExam && !ok && !slices.Contains(listed, ix.byID[e.From]) {
				if out == nil {
					out = &Edge{From: e.From, To: e.To, Kind: e.Kind, Confidence: e.Confidence, File: e.File, Line: e.Line, Count: e.Count}
				} else {
					out.Count, out.Line = out.Count+max(e.Count, 1), min(out.Line, e.Line)
				}
			}
		}
	}
	return out
}

// printExamEdge tests-for çıktısında sınav grubunu ayrı başlıkla yazar.
func (ix *index) printExamEdge(w io.Writer, e *Edge) {
	if e != nil {
		fmt.Fprintf(w, "  <- exam questions that name it (1; questions are plain text in %s):\n    %s\n", examFile, ix.edgeLine(e, e.From, true))
	}
}

// printGoFileExams bir Go dosyasının ve bildirimlerinin sınav kenarını ayrı başlıkla yazar.
func (ix *index) printGoFileExams(w io.Writer, file *Node, ts *touchSet, s fileTestSet) {
	ix.printExamEdge(w, ix.examEdge(s.own, s.all, append(slices.Collect(maps.Values(ts.nodes)), file)...))
}

// examTestsFor dokunulan düğümde, dosyada ya da silinen satırda sınavın andığı ad geçiyorsa sınav testini verir.
func (ix *index) examTestsFor(ts *touchSet) []*Node {
	exam := ix.byID["go:func:"+ix.g.Module+"/tools/codegraph."+examTest]
	if exam != nil && (slices.ContainsFunc(ix.out[exam.ID], func(e *Edge) bool {
		n := ix.byID[e.To]
		return e.Kind == edgeNamesInExam && (ts.nodes[e.To] != nil || n != nil && ts.files[n.File] && (n.Kind == kindFile || n.Kind == kindDoc))
	}) || len(ts.hunks) > 0 && ix.deletedHunkNamesExam(ts)) {
		return []*Node{exam}
	}
	return nil
}

// mixedCaseTail sözcüğün ilk harfinden sonrasında büyük harf var mı; ilk harf rune olarak kesilir (`çok`'un ilk baytı
// kesilince kalan geçersiz UTF-8 ToLower'da değişiyor, sözcük karışık harfli sayılıyordu).
func mixedCaseTail(tok string) bool {
	_, n := utf8.DecodeRuneInString(tok)
	return tok[n:] != strings.ToLower(tok[n:])
}

// deletedHunkNamesExam farkın silinen satırlarında ya da eski dosya yolunda grafikte olmayan bir sınav sözcüğü arar.
func (ix *index) deletedHunkNamesExam(ts *touchSet) bool {
	words := map[string]bool{}
	for _, ws := range parseExamWords(ix.root) {
		for _, w := range ws {
			words[w] = w != "" && !strings.Contains(w, "..")
		}
	}
	for i := range ix.g.Nodes {
		for _, k := range examNodeKeys(&ix.g.Nodes[i]) {
			delete(words, k)
		}
	}
	types := slices.Collect(maps.Values(ts.nodes))
	for _, h := range ts.hunks {
		if words[h.oldFile] {
			return true
		}
		pkg := map[bool]string{false: path.Base(path.Dir(h.file)), true: strings.TrimSuffix(path.Base(h.file), ".tr")}[strings.HasSuffix(h.file, ".tr")]
		for _, l := range h.changedLines {
			if strings.HasPrefix(l, "-") && !strings.HasSuffix(h.file, ".md") {
				toks := examIdent.FindAllString(l[1:], -1)
				for i, tok := range toks {
					if (examSymRe.MatchString(tok) || len(tok) >= 3 && mixedCaseTail(tok)) && words[tok] ||
						words[pkg+"."+tok] ||
						slices.ContainsFunc(toks[i+1:], func(b string) bool { return words[tok+"."+b] || words[pkg+"."+tok+"."+b] }) ||
						slices.ContainsFunc(types, func(n *Node) bool {
							return n.File == h.file && (n.Kind == kindType || n.Kind == kindTRStruct) && words[n.Name+"."+tok]
						}) {
						return true
					}
				}
			}
		}
	}
	return false
}
