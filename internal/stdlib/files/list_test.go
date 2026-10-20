package files

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// buildTree testlerin ortak klasör ağacını kurar.
func buildTree(t *testing.T, k string) {
	t.Helper()
	for _, y := range []string{"v/a.npy", "v/b.txt", "v/alt/c.npy", "v/alt/derin/d.npy", "v/.gizli/e.npy", "v/.f.npy", "v/Büyük.NPY"} {
		write(t, filepath.Join(k, filepath.FromSlash(y)), "x")
	}
}

func TestListDir(t *testing.T) {
	k := tmpWork(t)
	buildTree(t, k)
	cases := map[string]string{
		`klasörü_listele("v")`:              "[v/Büyük.NPY, v/a.npy, v/alt, v/b.txt]",
		`klasörü_listele("v", "*.npy")`:     "[v/a.npy]",
		`klasörü_listele("v", "**/*.npy")`:  "[v/a.npy, v/alt/c.npy, v/alt/derin/d.npy]",
		`klasörü_listele("v", "**")`:        "[v/Büyük.NPY, v/a.npy, v/alt, v/alt/c.npy, v/alt/derin, v/alt/derin/d.npy, v/b.txt]",
		`klasörü_listele("v", "alt/*")`:     "[v/alt/c.npy, v/alt/derin]",
		`klasörü_listele("v", ".*")`:        "[v/.f.npy, v/.gizli]",
		`klasörü_listele("v", ".gizli/*")`:  "[v/.gizli/e.npy]",
		`klasörü_listele("v", "[ab].*")`:    "[v/a.npy, v/b.txt]",
		`klasörü_listele("v", "**/derin")`:  "[v/alt/derin]",
		`klasörü_listele("./v/", "a.npy")`:  "[v/a.npy]",
		`klasörü_listele(yol("v"), "yok*")`: "[]",
		`tip(klasörü_listele("v")[0])`:      "Yol",
		`klasörü_listele()`:                 "[v]",
	}
	if caseInsensitive {
		cases[`klasörü_listele("v", "*.npy")`] = "[v/Büyük.NPY, v/a.npy]"
		cases[`klasörü_listele("v", "**/*.npy")`] = "[v/Büyük.NPY, v/a.npy, v/alt/c.npy, v/alt/derin/d.npy]"
	}
	for code, wants := range cases {
		if got := mustRunTR(t, code); got != wants {
			t.Errorf("%s = %s, %s bekleniyordu", code, got, wants)
		}
	}
	// Mutlak taban: sonuçlar da mutlak.
	got := mustRunTR(t, `klasörü_listele(yol("`+forwardPath(filepath.Join(k, "v"))+`"), "*.txt")`)
	if !strings.HasSuffix(got, "/v/b.txt]") || !strings.Contains(got, ":") && !strings.HasPrefix(got, "[/") {
		t.Errorf("mutlak taban: %s", got)
	}
	errContains(t, `klasörü_listele("yok")`, `klasörü_listele(): "yok" listelenemedi: böyle bir dosya ya da klasör yok`)
	errHas(t, `klasörü_listele("v/a.npy")`, messages.H7136)
	errContains(t, `klasörü_listele("v", "../*")`, `kalıp "../*" geçersiz: kalıpta .. olamaz`)
	errHas(t, `klasörü_listele("v", "../*")`, messages.H7135, messages.H7143)
	errContains(t, `klasörü_listele("v", 5)`, "2. argüman olarak Metin bekler")
}

// ** klasör bağlantılarının içine girmez (döngü olmaz); adıyla yazılmış bağlantının içine
// girilir.
func TestListDoesNotEnterLinks(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "v", "a.npy"), "x")
	dirLink(t, filepath.Join(k, "v"), filepath.Join(k, "v", "dongu"))
	if got := mustRunTR(t, `klasörü_listele("v", "**/*.npy")`); got != "[v/a.npy]" {
		t.Errorf("** bağlantıya girdi: %s", got)
	}
	if got := mustRunTR(t, `klasörü_listele("v", "dongu/*.npy")`); got != "[v/dongu/a.npy]" {
		t.Errorf("adıyla yazılan bağlantı: %s", got)
	}
}
