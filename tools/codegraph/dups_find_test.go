package main

import "testing"

// Kural: tekrar adayının satır aralığı yalnız eşleşen yöntemin satırlarıdır; yan yana tek satırlık
// yöntemler tek bir bölgeye birleşmez ve komşu çok satırlık yöntemin başlığına taşan eşleşme
// o yöntemin kapanış satırına uzamaz.
func TestDupsRegionStaysWithinMethodAndSplitsOneLiners(t *testing.T) {
	const fileA = `package p

type Box struct{ v []float64 }

func (b *Box) Log2() *Box    { return unary(b, 1, 0) }
func (b *Box) Log10() *Box   { return unary(b, 2, 0) }
func (b *Box) Sqrt() *Box    { return unary(b, 3, 0) }
func (b *Box) Rsqrt() *Box   { return unary(b, 4, 0) }
func (b *Box) Abs() *Box     { return unary(b, 5, 0) }
func (b *Box) Sign() *Box    { return unary(b, 6, 0) }
func (b *Box) Sigmoid() *Box { return unary(b, 7, 0) }
func (b *Box) Silu() *Box    { return unary(b, 8, 0) }

func (b *Box) Clamp(lo, hi float64) *Box {
	out := make([]float64, len(b.v))
	for i, x := range b.v {
		if x < lo {
			x = lo
		} else if x > hi {
			x = hi
		}
		out[i] = x
	}
	return &Box{v: out}
}
`
	const fileB = `package p

func (b *Box) Square() *Box  { return unary(b, 11, 0) }
func (b *Box) Exp() *Box     { return unary(b, 12, 0) }
func (b *Box) Tanh() *Box    { return unary(b, 13, 0) }
func (b *Box) Neg() *Box     { return unary(b, 14, 0) }
func (b *Box) Floor() *Box   { return unary(b, 15, 0) }
func (b *Box) Ceil() *Box    { return unary(b, 16, 0) }
func (b *Box) Round() *Box   { return unary(b, 17, 0) }
func (b *Box) Gelu() *Box    { return unary(b, 18, 0) }
func (b *Box) Softplus() *Box { return unary(b, 19, 0) }
`
	root := dupTree(t, map[string]string{"p/wide.go": "src:" + fileA, "p/base.go": "src:" + fileB})
	if clusters, _ := dupsOf(t, root, 90, false); len(clusters) != 0 {
		t.Fatalf("adjacent one-line methods merged into %d cluster(s): first %+v", len(clusters), clusters[0].Locs)
	}
	clusters, _ := dupsOf(t, root, 18, false)
	if len(clusters) == 0 {
		t.Fatal("expected single one-line methods to match at min=18")
	}
	for _, c := range clusters {
		for _, loc := range c.Locs {
			if loc.From != loc.To {
				t.Errorf("one-line method span crossed lines: %+v", loc)
			}
			if loc.File == "p/wide.go" && loc.To >= 14 {
				t.Errorf("region spilled into Clamp (line >= 14): %+v", loc)
			}
		}
	}

	const multiA = `package p

type Vec struct{ xs []int }

func (v *Vec) Normalize(scale int) []int {
	out := make([]int, len(v.xs))
	for i, x := range v.xs {
		if x < 0 {
			x = -x
		}
		out[i] = x * scale
	}
	return out
}

func (v *Vec) Clamp(lo, hi int) []int {
	out := make([]int, len(v.xs))
	for i, x := range v.xs {
		if x < lo {
			x = lo
		}
		out[i] = x + hi
	}
	return out
}
`
	const multiB = `package p

func (v *Vec) Rescale(factor int) []int {
	out := make([]int, len(v.xs))
	for i, x := range v.xs {
		if x < 0 {
			x = -x
		}
		out[i] = x * factor
	}
	return out
}

func (v *Vec) Next() int {
	return len(v.xs)
}
`
	root2 := dupTree(t, map[string]string{"q/a.go": "src:" + multiA, "q/b.go": "src:" + multiB})
	c := hasCluster(func() []*dupCluster { cs, _ := dupsOf(t, root2, 30, false); return cs }(), "q/a.go:Vec.Normalize", "q/b.go:Vec.Rescale")
	if c == nil {
		t.Fatal("expected Vec.Normalize ~ Vec.Rescale duplicate cluster")
	}
	for _, loc := range c.Locs {
		if loc.File == "q/a.go" && (loc.From != 5 || loc.To != 14) {
			t.Errorf("q/a.go Vec.Normalize span = %d-%d, want 5-14", loc.From, loc.To)
		}
		if loc.File == "q/b.go" && (loc.From != 3 || loc.To != 12) {
			t.Errorf("q/b.go Vec.Rescale span = %d-%d, want 3-12", loc.From, loc.To)
		}
	}
}
