package checker

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Sayaç (1.2, 3.13): yerel değişkenin tipi ilk değerinden çıkar. Depodaki programların (kitaplık,
// dil tanımı testleri, örnekler, GPU araçları; olumsuz fikstürler hariç) TR_var ve TR_const ilk
// değerlerinden (iç içe olanlar da) kaçının tipi denetleyicide bilinmiyor (TR_Any)? Düşmez: çıkarımın
// ilerleyişini yazar (go test -run TestRepoFirstValueTypes -v); bilinmeyenler ifade türüne göre de
// sayılır. Dosya kodlu biçimiyle okunur: başlık satırı bir yorumdur (iç test paketi source'u aktaramaz).
func TestRepoFirstValueTypes(t *testing.T) {
	files := testkit.GlobFiles(t, "lib/*.tr", "tests/*.tr", "examples/tr/*.tr", "tools/gpu/*.tr")
	total, unknown := 0, 0
	byKind := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewBody(strings.TrimPrefix(string(data), "\uFEFF"), true))
		prog := p.ParseProgram()
		if len(p.Errs()) > 0 {
			continue
		}
		c := New()
		c.CheckProgram(prog)
		for value, typ := range c.valueTypes {
			total++
			if typ == typeAny {
				unknown++
				byKind[strings.TrimPrefix(fmt.Sprintf("%T", value), "*ast.")]++
			}
		}
	}
	if len(files) < 40 || total < 1000 {
		t.Fatalf("depo düzeni değişmiş: %d dosyada %d ilk değer", len(files), total)
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		return byKind[kinds[i]] > byKind[kinds[j]] || byKind[kinds[i]] == byKind[kinds[j]] && kinds[i] < kinds[j]
	})
	var parts []string
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s %d", k, byKind[k]))
	}
	t.Logf("%d dosya, %d ilk değer; tipi bilinmeyen (TR_Any): %d (%s)", len(files), total, unknown, strings.Join(parts, ", "))
}
