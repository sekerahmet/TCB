package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const linkedSource = `TR_struct Dikkat {
    baş
    TR_func TR_init(boyut, baş) {
        TR_self.baş = baş
    }
}

TR_struct Blok {
    TR_attention
    TR_func TR_init(boyut, baş) {
        TR_self.TR_attention = Dikkat(boyut, baş)
    }
}

TR_struct Model {
    TR_config
    blok
    TR_func TR_init(boyut, baş = 2) {
        TR_self.TR_config = {"embed_dim": boyut, "baş": baş}
        TR_self.blok = Blok(boyut, baş: baş)
    }
}
`

// Kural: aynı adla iletilen parametre (adlı ya da konumlu) ve aynı adlı alana atanan parametre bağlı bildirimdir:
// yeniden adlandırma bunları kesin değil ayrı bir grupta verir (bağlı kümenin bütünü, geçişli). Aynı adlı sözlük
// anahtarı bir metindir (3.7: {"baş": baş}): olası grupta metin sabiti olarak durur, kendiliğinden değişmez (deneme 7 R2).
func TestRenameShowsLinkedDeclarationsAndDictKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), trText(filepath.Join(dir, "m.tr"), linkedSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	res := r.RenameImpact("parameter:m.tr#Model.TR_ctx_init.baş", "bakış", RenameOptions{Root: dir})
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
	var linked []string
	for _, n := range res.Linked {
		linked = append(linked, n.ID)
	}
	want := "field:m.tr#Dikkat.baş parameter:m.tr#Blok.TR_ctx_init.baş parameter:m.tr#Dikkat.TR_ctx_init.baş"
	if strings.Join(linked, " ") != want {
		t.Errorf("linked = %v, want %s", linked, want)
	}
	for _, c := range res.Certain {
		if c.Node != "parameter:m.tr#Model.TR_ctx_init.baş" {
			t.Errorf("a linked declaration is not a certain change: %+v", c)
		}
	}
	dictKey := false
	key := srcAt(t, linkedSource, "TR_self.TR_config = {", "baş", 1) // {"embed_dim": boyut, "baş": baş} içindeki anahtar
	for _, c := range res.Likely {
		if fmt.Sprintf("%d:%d", c.Pos.Line, c.Pos.Col) == key {
			dictKey = c.Where == WhereString
		}
	}
	for _, c := range res.Certain {
		if fmt.Sprintf("%d:%d", c.Pos.Line, c.Pos.Col) == key {
			t.Errorf("the dictionary key is a certain change: %+v", c)
		}
	}
	if !dictKey {
		t.Errorf("the key in {\"embed_dim\": boyut, \"baş\": baş} is a likely change in a string:\n%s", RenameText(res))
	}
	text := RenameText(res)
	if !strings.Contains(text, "bağlı bildirimler (3;") && !strings.Contains(text, "linked declarations (3;") {
		t.Errorf("the linked group is rendered with its count:\n%s", text)
	}
}

// Kural: `Yapı(...)` çağrısı yapının kurucusunu da çağırır; kurucunun çağırdığı işlev o çağrıyı yapan testle sınanır
// (bağımsız denetim Y12: ağırlık_başlat'ı yalnızca Doğrusal'ın kurucusu çağırıyordu, iki araç da "sınayan yok" diyordu).
func TestAConstructorCallReachesItsKurucu(t *testing.T) {
	dir := t.TempDir()
	src := "TR_func başlat(n) {\n    TR_return n\n}\n\nTR_struct Katman {\n    w\n    TR_func TR_init(n) {\n        TR_self.w = başlat(n)\n    }\n}\n"
	test := "TR_test \"katman kurulur\" {\n    TR_var k = Katman(2)\n    TR_assert(k.w == 2)\n}\n"
	for name, text := range map[string]string{"m_test.tr": src + "\n" + test} { // aynı dosya: aktarma gerekmez
		if err := os.WriteFile(filepath.Join(dir, name), trText(filepath.Join(dir, name), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := NewReader(idx).TestedBy("başlat", QueryOptions{}); len(got.Tests) != 1 {
		t.Errorf("başlat is tested through Katman's kurucu: %+v", got.Tests)
	}
}

// forwardSource konumlu iletimde sıra ve alana atamada gölgeleme: `f`'nin `a`'sı `g`'nin `b`'sine gider (bağ yok),
// `h`'nin `a`'sı `g`'nin `a`'sına (bağ var); Kişi'nin kurucusunda döngü değişkeni `ad` parametreyi gölgeler (bağ yok).
const forwardSource = `TR_func g(b, a) {
    TR_return b + a
}

TR_func f(a) {
    TR_return g(a, 1)
}

TR_func h(a) {
    TR_return g(1, a)
}

TR_struct Kişi {
    ad
    TR_func TR_init(ad, adlar) {
        TR_for ad TR_in adlar {
            TR_self.ad = ad
        }
    }
}

TR_struct Yer {
    ad
    TR_func TR_init(ad) {
        TR_self.ad = ad
    }
}
`

// Kural: konumlu argüman yalnızca çağrılanın aynı sıradaki parametresi aynı adı taşıyorsa bağlıdır; gövdede yeniden
// bağlanan (gölgelenen) parametre ne iletimde ne alana atamada bağ kurar (bağımsız denetim grup 6, R2).
func TestForwardsFollowPositionAndShadowing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), trText(filepath.Join(dir, "m.tr"), forwardSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	var got []string
	for _, e := range idx.Edges {
		if e.Kind == EdgeForwards {
			got = append(got, e.From+" -> "+e.To)
		}
	}
	want := "parameter:m.tr#Yer.TR_ctx_init.ad -> field:m.tr#Yer.ad parameter:m.tr#h.a -> parameter:m.tr#g.a"
	if strings.Join(got, " ") != want {
		t.Errorf("forwards = %v, want %s", got, want)
	}
}
