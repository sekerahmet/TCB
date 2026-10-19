package files

import (
	"path"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestJoker(t *testing.T) {
	durumlar := []struct {
		kalip, ad string
		ister     bool
	}{
		{"*.npy", "a.npy", true}, {"*.npy", "a.npz", false}, {"*", "", true},
		{"a?c", "abc", true}, {"a?c", "ac", false}, {"a?c", "açc", true},
		{"[a-c]x", "bx", true}, {"[a-c]x", "dx", false}, {"[!a-c]x", "dx", true}, {"[^a-c]x", "ax", false},
		{"[]a]", "]", true}, {"*a*b*c", "xxaxxbxxc", true}, {"*a*b*c", "xxaxxcxxb", false},
		{"parça_*_[0-9][0-9].npy", "parça_eğitim_07.npy", true},
		{"ğ*ş", "ğüş", true},
	}
	for _, d := range durumlar {
		if got := joker(d.kalip, d.ad); got != d.ister {
			t.Errorf("joker(%q, %q) = %v", d.kalip, d.ad, got)
		}
	}
}

func TestKalipHatalari(t *testing.T) {
	for kalip, hata := range map[string]error{
		"": errKalipBos, "./.": errKalipBos, "/": errKalipMutlak, "/a": errKalipMutlak, "../a": errKalipUst, "a/../b": errKalipUst,
		"a**": errKalipCift, "a/**b/c": errKalipCift, "[a": errKalipKume, "a[]": errKalipKume, "[!]": errKalipKume,
	} {
		if _, err := kalipDerle(kalip); err != hata {
			t.Errorf("kalipDerle(%q) = %v, %v bekleniyordu", kalip, err, hata)
		}
	}
	k, err := kalipDerle("./a/**/**/*.npy")
	if err != nil || len(k.bilesenler) != 3 || !k.bilesenler[1].cift {
		t.Errorf("**/** tek ** olmalı: %+v %v", k, err)
	}
}

// Gizli adlar kalıbın bileşeni nokta ile başlamadıkça eşlenmez.
func TestKalipGizli(t *testing.T) {
	k, _ := kalipDerle("*")
	if k.bilesenler[0].eslesir(".git") {
		t.Error("* gizli adı eşlememeli")
	}
	k, _ = kalipDerle(".*")
	if !k.bilesenler[0].eslesir(".git") || !k.gizli {
		t.Error(".* gizli adı eşlemeli")
	}
}

// FuzzKalip: kalıp kullanıcıdan (ya da dışarıdan gelen bir ayar dosyasından) gelir. Derleme ve
// eşleme hiçbir girdide çökmemeli, sonsuz döngüye girmemeli; özel karakteri olmayan bileşen ad
// eşitliğine, öteki kalıplar (harf duyarlı kipte, ortak sözdiziminde) dilin standart
// eşleyicisine denk olmalıdır. Derin arama:
//
//	go test ./internal/stdlib/files -run '^$' -fuzz FuzzKalip -fuzztime 5m
func FuzzKalip(f *testing.F) {
	for _, s := range [][2]string{{"*.npy", "a.npy"}, {"**/*.tr", "x"}, {"[a-z]?*", "ab"}, {"[!x]", "y"}, {"a*b*c*d", "aXbYcZd"}, {"[]-]", "-"}, {"ç*", "çğ"}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, desen, ad string) {
		k, err := kalipDerle(desen)
		if err != nil || !utf8.ValidString(desen) || !utf8.ValidString(ad) || strings.Contains(ad, "/") {
			return
		}
		eski := harfDuyarsiz
		harfDuyarsiz = false
		defer func() { harfDuyarsiz = eski }()
		for _, b := range k.bilesenler {
			if b.cift {
				continue
			}
			got := joker(b.metin, ad)
			if b.sabit && got != (b.metin == ad) {
				t.Fatalf("sabit %q, %q: %v", b.metin, ad, got)
			}
			if ortakSozdizimi(b.metin) {
				if ister, err := path.Match(b.metin, ad); err == nil && ister != got {
					t.Fatalf("joker(%q, %q) = %v, standart eşleyici %v", b.metin, ad, got, ister)
				}
			}
		}
	})
}

// ortakSozdizimi kalıp, standart eşleyiciyle aynı anlamı taşıyan sözdiziminde mi (kaçış yok,
// ! ile olumsuzlama yok, kümenin kenarında - ya da ] yok)?
func ortakSozdizimi(k string) bool {
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

// FuzzYolCoz: yol(...) parçaları dışarıdan gelebilir (veri kümesinin adı, ayar dosyası).
// Temizlenmiş yol tek yazımlıdır (yeniden temizlemek değiştirmez), ikinci parça mutlaksa
// hata verilir, üyeler tutarlıdır (uzantısız_ad + uzantı = dosya_adı) ve çözme çökmez.
func FuzzYolCoz(f *testing.F) {
	for _, s := range [][2]string{{"veri", "a.npy"}, {"a/./b", "../c"}, {"C:", "x"}, {"/", ".."}, {".", ".gitignore"}, {"a", "/etc/passwd"}, {`a\b`, `..\..\c`}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if a == "" || b == "" || strings.ContainsAny(a+b, "\t\n\r\x00") {
			return
		}
		if mutlakParcaMi(b) {
			return // yolYerlesik reddeder (TestYolHatalari)
		}
		y := YeniYol(filepath.Join(filepath.FromSlash(a), filepath.FromSlash(b)))
		if temizle(y.yol) != y.yol {
			t.Fatalf("temizlenmiş yol yeniden temizlenince değişti: %q", y.yol)
		}
		ad := y.dosyaAdi()
		if u := uzanti(ad); !strings.HasSuffix(ad, u) || strings.Contains(ad, "/") {
			t.Fatalf("dosya_adı %q, uzantı %q", ad, u)
		}
		if y.yol != "/" && strings.HasSuffix(y.yol, "/") && filepath.Dir(filepath.FromSlash(y.yol)) != filepath.FromSlash(y.yol) {
			t.Fatalf("kök olmayan yol / ile bitiyor: %q", y.yol)
		}
		_, _ = y.coz(nil)
	})
}
