package tensor

import (
	"fmt"
	"go/format"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
)

// bf16 tolerans ölçümü: her gradyan kaydının bf16 sapmasını measureSeeds girdi tohumunda
// ölçer. Yalnızca TR_MEASURE_BF16 verilince koşar:
//
//	TR_MEASURE_BF16=1     go test ./internal/compute/tensor -run TestMeasureBF16 -v   # tabloyu basar
//	TR_MEASURE_BF16=write go test ./internal/compute/tensor -run TestMeasureBF16      # behavior_grad_bf16_tolerances_test.go'yu yeniden yazar
//
// Tolerans kuralı: en büyük ölçülen sapmanın 2 katı, 1e-3'ün katına yukarı yuvarlanır, en az
// bf16Floor. Alt sınır bir bf16 yuvarlamasıdır (2⁻⁸ ≈ 3,9e-3): tam çıkan işlemlerde (görünümler,
// karşılaştırmalar) ölçülen sapma 0'dır, ama toleransı 0'a çekmek aynı işlemi başka bir
// hesap sırasıyla yapan arka ucu (GPU) gereksiz yere düşürürdü.
const (
	measureSeeds = 32
	bf16Floor    = 4e-3
)

type bf16Row struct {
	name string
	max  float64
	tol  float64
}

func measureBF16(t *testing.T) []bf16Row {
	var rows []bf16Row
	for group, specs := range gradGroups {
		for _, s := range specs {
			var mx float64
			for k := 0; k < measureSeeds; k++ {
				if w, _ := bf16Deviation(t, s, k); w > mx {
					mx = w
				}
			}
			tol := math.Max(math.Ceil(2*mx*1000)/1000, bf16Floor)
			rows = append(rows, bf16Row{group + "/" + s.name, mx, tol})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

func TestMeasureBF16(t *testing.T) {
	mode := os.Getenv("TR_MEASURE_BF16")
	if mode == "" {
		t.Skip("ölçüm için TR_MEASURE_BF16=1 (tablo) ya da write (dosyayı yenile) verin")
	}
	rows := measureBF16(t)
	var b strings.Builder
	b.WriteString("| kayıt | en büyük sapma | tolerans |\n| :-- | --: | --: |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %.4f | %g |\n", r.name, r.max, r.tol)
	}
	t.Log("\n" + b.String())
	if mode != "write" {
		return
	}
	var g strings.Builder
	g.WriteString("package tensor\n\n// Bu dosya TestMeasureBF16 tarafından üretilir (TR_MEASURE_BF16=write); elle düzenlenmez.\n// Kural ve ölçüm tablosu: README.md \"bf16 toleransları\".\n\n")
	g.WriteString("// bf16Tolerances gradyan kaydı (\"grup/ad\") başına bf16 gradyan toleransıdır.\nvar bf16Tolerances = map[string]float64{\n")
	for _, r := range rows {
		fmt.Fprintf(&g, "\t%q: %g,\n", r.name, r.tol)
	}
	g.WriteString("}\n")
	src, err := format.Source([]byte(g.String()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("behavior_grad_bf16_tolerances_test.go", src, 0o644); err != nil {
		t.Fatal(err)
	}
}
