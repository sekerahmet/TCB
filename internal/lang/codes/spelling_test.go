package codes

import (
	"strings"
	"testing"
)

// Kural (2.3a): dilin sözcükleri TR_ ve kimliğin adıyla yazılır, sınıfı yeri belirler. Bu yüzden bir
// yerde aynı yazımlı iki kimlik olamaz: TR_print öbür yerde tek bir kodu, x.TR_map üye yerinde tek bir
// kodu adlandırır. Yazım çizelgesi (spellings) aynı yazımı sessizce üst üste yazmasın diye her yazımın
// tek sahibi sınanır; her kodun yazımı kendi yerinde ona döner.
func TestNoTwoIdentitiesShareASpellingInAPlace(t *testing.T) {
	var owner [placeCount]map[string]Code
	for p := range owner {
		owner[p] = map[string]Code{}
	}
	written := 0
	for i, id := range identities {
		k := Code(i)
		p, ok := PlaceOf(k)
		if id == "" || k == None || !ok {
			continue
		}
		s := Spelling(k)
		if o, dup := owner[p][s]; dup {
			t.Errorf("%s ve %s aynı yerde (%s) aynı yazımı taşır: %s", o, k, p, s)
			continue
		}
		owner[p][s] = k
		if got, ok := FromSpelling(p, s); !ok || got != k {
			t.Errorf("FromSpelling(%s, %s) = %s %v; %s bekleniyordu", p, s, got, ok, k)
		}
		written++
	}
	if written < 500 {
		t.Fatalf("yalnızca %d kimlik yazımı sınandı; çizelge bozulmuş olabilir", written)
	}
}

// Yazım sınıf parçasını atar ve her dilde aynıdır; başka yerdeki yazım o yerde bulunmaz. Tanıdık ad
// (TR_'siz ya da başka bir dilin adı) TR_ ile yazımını önerir; anahtar kelimeyi gösteren tanıdık ad önce
// gelir (None → TR_none, TR_None tipi değil).
func TestSpellingAndSuggestion(t *testing.T) {
	for k, want := range map[Code]string{FnPrint: "TR_print", MemAppend: "TR_append", ArgLr: "TR_lr", TypeInt: "TR_Int", KwFor: "TR_for"} {
		if got := Spelling(k); got != want {
			t.Errorf("Spelling(%s) = %q, %q bekleniyordu", k, got, want)
		}
	}
	if _, ok := FromSpelling(PlaceMember, "TR_print"); ok {
		t.Error("TR_print üye yerinde bulunmamalı")
	}
	for name, want := range map[string]string{"print": "TR_print", "None": "TR_none", "True": "TR_true", "def": "TR_func", "elif": "TR_else TR_if"} {
		if got, ok := SuggestIdentity(name); !ok || got != want {
			t.Errorf("SuggestIdentity(%q) = %q %v, %q bekleniyordu", name, got, ok, want)
		}
	}
	if got, ok := SuggestIdentity("öğrenci_sayısı"); ok {
		t.Errorf("kullanıcı adına öneri olmamalı: %q", got)
	}
}

// Kural (2.3a): program metninde sınıf öneki yazılmaz (TR_fn_print yerine TR_print). Sözcük
// çözümleyicinin sınıflı yazımı (TR_<sınıf>_<ad>) sınıfsız yazımdan ayırabilmesi için hiçbir kodun
// kısa adı bir sınıf önekiyle (<sınıf>_) başlamamalıdır.
func TestNoShortNameStartsWithAClassPrefix(t *testing.T) {
	for i, id := range identities {
		k := Code(i)
		if id == "" || k == None {
			continue
		}
		short := strings.TrimPrefix(Spelling(k), IdentityPrefix)
		for _, c := range Classes {
			if strings.HasPrefix(short, string(c.Class)+"_") {
				t.Errorf("%s kodunun kısa adı (%s) %s_ sınıf önekiyle başlıyor", k, short, c.Class)
			}
		}
	}
}
