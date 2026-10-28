package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// trText bir test projesine yazılan dosyanın içeriğidir: .tr dosyası başlıksızsa başına `# TR 4` başlığı konur (dil tanımı
// 2.1: başlıksız dosya okunmaz), öbür dosyalar olduğu gibi. Satırlar başlıktan sonra bir kayar: beklenen konumlar
// başlıkla birlikte sayılır.
func trText(path, text string) []byte {
	if strings.HasSuffix(path, ".tr") && !strings.HasPrefix(text, source.Header) {
		text = source.Header + "\n" + text
	}
	return []byte(text)
}

// srcAt src'nin dosyaya yazılmış hâlinde (trText: başlıkla) line'ı içeren ilk satırda word'ün n. (1'den) geçişinin
// "satır:sütun"udur: beklenen konum kaynaktan hesaplanır, elle sayı yazılmaz.
func srcAt(t *testing.T, src, line, word string, n int) string {
	t.Helper()
	for i, l := range strings.Split(string(trText("m.tr", src)), "\n") {
		if !strings.Contains(l, line) {
			continue
		}
		at := 0
		for k := 0; k < n; k++ {
			j := strings.Index(l[at:], word)
			if j < 0 {
				t.Fatalf("%q has no %d. %q", l, n, word)
			}
			at += j + len(word)
		}
		return fmt.Sprintf("%d:%d", i+1, len([]rune(l[:at-len(word)]))+1)
	}
	t.Fatalf("no line with %q", line)
	return ""
}

// fixtureLines fikstür dosyasının satırlarıdır (1. satır başlık).
func fixtureLines(t *testing.T, rel string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}

// fixtureAt fikstür dosyasında metnin ilk geçtiği satır ve sütundur (1'den; sütun Unicode karakteri). Beklenen konum
// dosyadan hesaplanır, elle sayı yazılmaz: yazım değişince (başlık, kimlik yazımı) satırlar ve sütunlar kayar.
func fixtureAt(t *testing.T, rel, text string) (line, col int) {
	t.Helper()
	for i, l := range fixtureLines(t, rel) {
		if j := strings.Index(l, text); j >= 0 {
			return i + 1, len([]rune(l[:j])) + 1
		}
	}
	t.Fatalf("%s lacks %q", rel, text)
	return 0, 0
}

// fixtureLine fikstür dosyasında metnin ilk geçtiği satırdır (fixtureAt).
func fixtureLine(t *testing.T, rel, text string) int {
	t.Helper()
	line, _ := fixtureAt(t, rel, text)
	return line
}

// fixtureClose from. satırdan sonra satırın tamamı close olan ilk satırdır: bir bloğun kapanışı (`    }`).
func fixtureClose(t *testing.T, rel string, from int, close string) int {
	t.Helper()
	lines := fixtureLines(t, rel)
	for i := from; i < len(lines); i++ {
		if lines[i] == close {
			return i + 1
		}
	}
	t.Fatalf("%s: no line %q after line %d", rel, close, from)
	return 0
}

// fixtureTest fikstürdeki test bloğunun kimliğidir: düğümün adı bloğun satırıdır (`test_block:lib/geo_test.tr#4`).
func fixtureTest(t *testing.T, rel, title string) string {
	t.Helper()
	return NodeID(KindTest, rel, strconv.Itoa(fixtureLine(t, rel, "TR_test \""+title+"\"")))
}

// declaredIDs bir dosyanın üst düzey ifadelerinden dizinde olması gereken kimlikleri ast'yi
// doğrudan sayarak çıkarır (dizinin kendi kodundan bağımsız bir döküm).
func declaredIDs(rel string, p *parsed) []string {
	var ids []string
	for _, s := range p.program.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				ids = append(ids, NodeID(KindFunction, rel, fl.Name))
			}
		case *ast.VarStatement:
			kind := KindVariable
			if s.IsConst {
				kind = KindConstant
			}
			ids = append(ids, NodeID(kind, rel, s.Name.Value))
		case *ast.StructStatement:
			ids = append(ids, NodeID(KindStruct, rel, s.Name))
			for _, f := range s.Fields {
				ids = append(ids, NodeID(KindField, rel, s.Name+"."+f.Name))
			}
			for name := range s.Methods {
				ids = append(ids, NodeID(KindMethod, rel, s.Name+"."+name))
			}
		case *ast.TestStatement:
			ids = append(ids, NodeID(KindTest, rel, strconv.Itoa(s.Token.Line)))
		}
	}
	return ids
}
