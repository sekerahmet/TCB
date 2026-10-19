package files

import (
	"path/filepath"
	"strings"
	"testing"
)

// agacKur testlerin ortak klasör ağacını kurar.
func agacKur(t *testing.T, k string) {
	t.Helper()
	for _, y := range []string{"v/a.npy", "v/b.txt", "v/alt/c.npy", "v/alt/derin/d.npy", "v/.gizli/e.npy", "v/.f.npy", "v/Büyük.NPY"} {
		yaz(t, filepath.Join(k, filepath.FromSlash(y)), "x")
	}
}

func TestKlasoruListele(t *testing.T) {
	k := geciciCalisma(t)
	agacKur(t, k)
	durumlar := map[string]string{
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
	if harfDuyarsiz {
		durumlar[`klasörü_listele("v", "*.npy")`] = "[v/Büyük.NPY, v/a.npy]"
		durumlar[`klasörü_listele("v", "**/*.npy")`] = "[v/Büyük.NPY, v/a.npy, v/alt/c.npy, v/alt/derin/d.npy]"
	}
	for kod, ister := range durumlar {
		if got := tamam(t, kod); got != ister {
			t.Errorf("%s = %s, %s bekleniyordu", kod, got, ister)
		}
	}
	// Mutlak taban: sonuçlar da mutlak.
	got := tamam(t, `klasörü_listele(yol("`+ileriYol(filepath.Join(k, "v"))+`"), "*.txt")`)
	if !strings.HasSuffix(got, "/v/b.txt]") || !strings.Contains(got, ":") && !strings.HasPrefix(got, "[/") {
		t.Errorf("mutlak taban: %s", got)
	}
	hataIcerir(t, `klasörü_listele("yok")`, `klasörü_listele(): "yok" listelenemedi: böyle bir dosya ya da klasör yok`)
	hataIcerir(t, `klasörü_listele("v/a.npy")`, "bir dosya, klasör değil")
	hataIcerir(t, `klasörü_listele("v", "../*")`, `kalıp "../*" geçersiz: kalıpta .. olamaz`)
	hataIcerir(t, `klasörü_listele("v", 5)`, "2. argüman olarak Metin bekler")
}

// ** klasör bağlantılarının içine girmez (döngü olmaz); adıyla yazılmış bağlantının içine
// girilir.
func TestListeleBaglantiyaGirmez(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "v", "a.npy"), "x")
	klasorBaglantisi(t, filepath.Join(k, "v"), filepath.Join(k, "v", "dongu"))
	if got := tamam(t, `klasörü_listele("v", "**/*.npy")`); got != "[v/a.npy]" {
		t.Errorf("** bağlantıya girdi: %s", got)
	}
	if got := tamam(t, `klasörü_listele("v", "dongu/*.npy")`); got != "[v/dongu/a.npy]" {
		t.Errorf("adıyla yazılan bağlantı: %s", got)
	}
}
