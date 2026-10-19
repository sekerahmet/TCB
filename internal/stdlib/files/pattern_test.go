package files

import (
	"path"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWildcard(t *testing.T) {
	cases := []struct {
		pattern, name string
		wants         bool
	}{
		{"*.npy", "a.npy", true}, {"*.npy", "a.npz", false}, {"*", "", true},
		{"a?c", "abc", true}, {"a?c", "ac", false}, {"a?c", "açc", true},
		{"[a-c]x", "bx", true}, {"[a-c]x", "dx", false}, {"[!a-c]x", "dx", true}, {"[^a-c]x", "ax", false},
		{"[]a]", "]", true}, {"*a*b*c", "xxaxxbxxc", true}, {"*a*b*c", "xxaxxcxxb", false},
		{"parça_*_[0-9][0-9].npy", "parça_eğitim_07.npy", true},
		{"ğ*ş", "ğüş", true},
	}
	for _, d := range cases {
		if got := wildcard(d.pattern, d.name); got != d.wants {
			t.Errorf("joker(%q, %q) = %v", d.pattern, d.name, got)
		}
	}
}

func TestPatternErrors(t *testing.T) {
	for pattern, failure := range map[string]error{
		"": errPatternEmpty, "./.": errPatternEmpty, "/": errPatternAbsolute, "/a": errPatternAbsolute, "../a": errPatternParent, "a/../b": errPatternParent,
		"a**": errPatternDouble, "a/**b/c": errPatternDouble, "[a": errPatternSet, "a[]": errPatternSet, "[!]": errPatternSet,
	} {
		if _, err := compilePattern(pattern); err != failure {
			t.Errorf("compilePattern(%q) = %v, %v bekleniyordu", pattern, err, failure)
		}
	}
	k, err := compilePattern("./a/**/**/*.npy")
	if err != nil || len(k.components) != 3 || !k.components[1].double {
		t.Errorf("**/** tek ** olmalı: %+v %v", k, err)
	}
}

// Gizli adlar kalıbın bileşeni nokta ile başlamadıkça eşlenmez.
func TestPatternHidden(t *testing.T) {
	k, _ := compilePattern("*")
	if k.components[0].match(".git") {
		t.Error("* gizli adı eşlememeli")
	}
	k, _ = compilePattern(".*")
	if !k.components[0].match(".git") || !k.hidden {
		t.Error(".* gizli adı eşlemeli")
	}
}

// FuzzPattern: kalıp kullanıcıdan (ya da dışarıdan gelen bir ayar dosyasından) gelir. Derleme ve
// eşleme hiçbir girdide çökmemeli, sonsuz döngüye girmemeli; özel karakteri olmayan bileşen ad
// eşitliğine, öteki kalıplar (harf duyarlı kipte, ortak sözdiziminde) dilin standart
// eşleyicisine denk olmalıdır. Derin arama:
//
//	go test ./internal/stdlib/files -run '^$' -fuzz FuzzPattern -fuzztime 5m
func FuzzPattern(f *testing.F) {
	for _, s := range [][2]string{{"*.npy", "a.npy"}, {"**/*.tr", "x"}, {"[a-z]?*", "ab"}, {"[!x]", "y"}, {"a*b*c*d", "aXbYcZd"}, {"[]-]", "-"}, {"ç*", "çğ"}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, glob, name string) {
		k, err := compilePattern(glob)
		if err != nil || !utf8.ValidString(glob) || !utf8.ValidString(name) || strings.Contains(name, "/") {
			return
		}
		old := caseInsensitive
		caseInsensitive = false
		defer func() { caseInsensitive = old }()
		for _, b := range k.components {
			if b.double {
				continue
			}
			got := wildcard(b.text, name)
			if b.fixed && got != (b.text == name) {
				t.Fatalf("sabit %q, %q: %v", b.text, name, got)
			}
			if commonSyntax(b.text) {
				if wants, err := path.Match(b.text, name); err == nil && wants != got {
					t.Fatalf("joker(%q, %q) = %v, standart eşleyici %v", b.text, name, got, wants)
				}
			}
		}
	})
}

// commonSyntax kalıp, standart eşleyiciyle aynı anlamı taşıyan sözdiziminde mi (kaçış yok,
// ! ile olumsuzlama yok, kümenin kenarında - ya da ] yok)?
func commonSyntax(k string) bool {
	if strings.ContainsAny(k, `\!`) || strings.Contains(k, "[]") || strings.Contains(k, "[^]") {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] == '[' && i+1 < len(k) && (k[i+1] == '-' || (k[i+1] == '^' && i+2 < len(k) && (k[i+2] == '-' || k[i+2] == ']'))) {
			return false
		}
		if k[i] == '-' && i+1 < len(k) && k[i+1] == ']' {
			return false
		}
	}
	return true
}

// FuzzPathResolve: yol(...) parçaları dışarıdan gelebilir (veri kümesinin adı, ayar dosyası).
// Temizlenmiş yol tek yazımlıdır (yeniden temizlemek değiştirmez), ikinci parça mutlaksa
// hata verilir, üyeler tutarlıdır (uzantısız_ad + uzantı = dosya_adı) ve çözme çökmez.
func FuzzPathResolve(f *testing.F) {
	for _, s := range [][2]string{{"veri", "a.npy"}, {"a/./b", "../c"}, {"C:", "x"}, {"/", ".."}, {".", ".gitignore"}, {"a", "/etc/passwd"}, {`a\b`, `..\..\c`}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if a == "" || b == "" || strings.ContainsAny(a+b, "\t\n\r\x00") {
			return
		}
		if isAbsPart(b) {
			return // pathBuiltin reddeder (TestPathErrors)
		}
		y := NewPath(filepath.Join(filepath.FromSlash(a), filepath.FromSlash(b)))
		if clean(y.fpath) != y.fpath {
			t.Fatalf("temizlenmiş yol yeniden temizlenince değişti: %q", y.fpath)
		}
		name := y.fileName()
		if u := ext(name); !strings.HasSuffix(name, u) || strings.Contains(name, "/") {
			t.Fatalf("dosya_adı %q, uzantı %q", name, u)
		}
		if y.fpath != "/" && strings.HasSuffix(y.fpath, "/") && filepath.Dir(filepath.FromSlash(y.fpath)) != filepath.FromSlash(y.fpath) {
			t.Fatalf("kök olmayan yol / ile bitiyor: %q", y.fpath)
		}
		_, _ = y.resolve(nil)
	})
}
