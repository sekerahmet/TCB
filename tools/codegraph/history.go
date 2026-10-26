package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// runHistory bir düğümün git geçmişini yazar (geçmiş graph.json'a yazılmaz, git'ten hesaplanır):
//   - işlev, metot, tip: `git log -L` ile o gövdeyi değiştiren commit'ler;
//   - ileti ve ad kodu: sözlük dosyalarında pickaxe; eklendiği ve silindiği commit işaretli;
//   - dosya ve belge: o dosyayı değiştiren commit'ler.
func (ix *index) runHistory(root string, n *Node, limit int, stdout, stderr io.Writer) int {
	var err error
	switch n.Kind {
	case kindFunc, kindMethod, kindType:
		err = historyBody(root, n, limit, stdout)
	case kindMessage, kindName:
		err = historyCode(root, n, limit, stdout)
	case kindFile, kindDoc:
		err = historyFile(root, n, limit, stdout)
	default:
		fmt.Fprintf(stderr, "history is not available for a %s node (functions, methods, types, message and name codes, files, docs)\n", n.Kind)
		return exitUsage
	}
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	return exitOK
}

const historyFormat = "--format=%h%x09%as%x09%s"

func printCommits(w io.Writer, n *Node, out []byte, limit int) {
	fmt.Fprintln(w, describe(n))
	lines := nonEmptyLines(string(out))
	if len(lines) == 0 {
		fmt.Fprintln(w, "  no commits found")
		return
	}
	for _, l := range lines {
		f := strings.SplitN(l, "\t", 3)
		if len(f) == 3 {
			fmt.Fprintf(w, "  %s  %s  %s\n", f[0], f[1], f[2])
		}
	}
	if limit > 0 && len(lines) >= limit {
		fmt.Fprintf(w, "  ... limited to %d commits (--all lists every one)\n", limit)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, "\r"))
		}
	}
	return out
}

func limitArg(limit int) []string {
	if limit > 0 {
		return []string{"-n", strconv.Itoa(limit)}
	}
	return nil
}

// historyBody `git log -L` ile gövdenin geçmişini verir. Bildirim satırı "func"/"type" ile
// başlıyorsa ada göre (satır numaraları geçmişte kayar), değilse (arayüz metodu, gruplu tip)
// bugünkü satır aralığına göre izlenir.
func historyBody(root string, n *Node, limit int, w io.Writer) error {
	spec := fmt.Sprintf("%d,%d:%s", n.Line, max(n.End, n.Line), n.File)
	short := n.Name[strings.LastIndex(n.Name, ".")+1:]
	decl := sourceLine(root, n.File, n.Line)
	switch {
	case n.Kind == kindType && strings.HasPrefix(decl, "type "+short+" "):
		spec = ":^type " + short + " :" + n.File
	case n.Kind == kindFunc && strings.HasPrefix(decl, "func "+short+"("):
		spec = ":^func " + short + "(:" + n.File
	case n.Kind == kindMethod && strings.HasPrefix(decl, "func ("):
		spec = ":^func (.*) " + short + "(:" + n.File
	}
	args := append([]string{"log", "-L", spec, "-s", historyFormat}, limitArg(limit)...)
	out, err := gitOut(root, args...)
	if err != nil {
		return err
	}
	printCommits(w, n, out, limit)
	return nil
}

// sourceLine dosyanın verilen satırını (1'den) verir; okunamazsa "".
func sourceLine(root, file string, line int) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[line-1], "\r")
}

func historyFile(root string, n *Node, limit int, w io.Writer) error {
	args := append([]string{"log", "--follow", historyFormat}, limitArg(limit)...)
	out, err := gitOut(root, append(args, "--", n.File)...)
	if err != nil {
		return err
	}
	printCommits(w, n, out, limit)
	return nil
}

// historyCode kodun sözlük dosyalarındaki geçmişini verir; eklendiği, silindiği ve eski
// anahtarından yeniden adlandırıldığı commit "added" / "removed" / "renamed" ile işaretlidir.
// Geçmiş eski anahtarlarıyla sürer (renames.go).
func historyCode(root string, n *Node, limit int, w io.Writer) error {
	files := nameFiles
	if n.Kind == kindMessage {
		files = messageFiles
	}
	renames := readRenames(root)
	events, err := scanCodeHistory(root, files, append([]string{n.Name}, formerKeys(renames)[n.Name]...)...)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, describe(n))
	list := followRenames(events, renames)[n.Name]
	if len(list) == 0 {
		fmt.Fprintln(w, "  no commits found")
		return nil
	}
	printLimited(w, formatEvents(list), limit, "  ")
	return nil
}
