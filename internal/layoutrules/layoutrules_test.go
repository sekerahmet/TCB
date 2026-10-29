package layoutrules

import "testing"

// Kural: istisnanın düzeltme payı tavanın %10'udur, en az 5 satır; tavan + pay istisnanın aşamayacağı boydur.
func TestFixMarginIsATenthAtLeastFive(t *testing.T) {
	for _, c := range []struct{ ceiling, margin int }{{40, 5}, {59, 5}, {60, 6}, {172, 17}, {915, 91}} {
		if got := FixMargin(c.ceiling); got != c.margin {
			t.Errorf("FixMargin(%d) = %d, want %d", c.ceiling, got, c.margin)
		}
	}
}

// Kural: listede olmayanın sınırı FileLimit/FuncLimit, istisnanın sınırı tavanı + payıdır; kod grafı da bu sınırı gösterir.
func TestAnExceptionsLimitIsItsCeilingPlusTheMargin(t *testing.T) {
	for file, ceiling := range BigFiles {
		if max, exc := FileMax(file); !exc || max != ceiling+FixMargin(ceiling) {
			t.Errorf("FileMax(%s) = %d, %v", file, max, exc)
		}
	}
	if max, exc := FileMax("a/b.go"); exc || max != FileLimit {
		t.Errorf("FileMax of a file off the list = %d, %v", max, exc)
	}
	if max, exc := FuncMax("a/b.go:f"); exc || max != FuncLimit {
		t.Errorf("FuncMax of a function off the list = %d, %v", max, exc)
	}
}

// Kural: düzen testdata, bin, scratch, node_modules, .git, .claude ve '.' ya da '_' ile başlayan klasörleri ölçmez.
func TestSkippedDirs(t *testing.T) {
	for _, d := range []string{"testdata", "bin", ".git", "_old", ".codegraph"} {
		if !SkippedDir(d) {
			t.Errorf("%s is measured", d)
		}
	}
	for _, d := range []string{"interp", "codegraph", "lib"} {
		if SkippedDir(d) {
			t.Errorf("%s is skipped", d)
		}
	}
}
