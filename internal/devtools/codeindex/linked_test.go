package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
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
        TR_self.TR_config = {TR_embed_dim: boyut, baş: baş}
        TR_self.blok = Blok(boyut, baş: baş)
    }
}
`

// Kural: aynı adla iletilen parametre (adlı ya da konumlu) ve aynı adlı alana atanan parametre bağlı bildirimdir:
// yeniden adlandırma bunları kesin değil ayrı bir grupta verir (bağlı kümenin bütünü, geçişli). Aynı adlı sözlük
// anahtarı veri biçimidir: olası grupta "sözlük anahtarı" rolüyle durur, kendiliğinden değişmez (deneme 7 R2, R3).
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
	key := srcAt(t, linkedSource, "TR_self.TR_config = {", "baş", 1) // {TR_embed_dim: boyut, baş: baş} içindeki anahtar
	for _, c := range res.Likely {
		at := fmt.Sprintf("%d:%d", c.Pos.Line, c.Pos.Col)
		if at == key && c.Role == RoleDictKey && c.Match == MatchDataFormat {
			dictKey = true
		}
		if at == key && c.Role != RoleDictKey {
			t.Errorf("the dictionary key is labelled as one: %+v", c)
		}
	}
	if !dictKey {
		t.Errorf("the key in {boyut: boyut, baş: baş} is a dictionary key:\n%s", RenameText(res))
	}
	text := RenameText(res)
	if !strings.Contains(text, "bağlı bildirimler (3;") && !strings.Contains(text, "linked declarations (3;") {
		t.Errorf("the linked group is rendered with its count:\n%s", text)
	}
}

// dictKeySource `a:` yazımının bütün biçimleri: yapı alanı, adlı argüman (dizgede ')' ile), sözlük anahtarı (dizgede ve
// yorumda '(' ile), tip yazımı.
const dictKeySource = `TR_struct Kişi {
    a: TR_String
}

TR_func hedef(a) {
    TR_return a
}

TR_func çağır(nesne) {
    TR_return nesne.koş(")", a: 1)
}

TR_func sözlükler(x) {
    TR_var s = {ad: "(", a: x}
    TR_var t = {
        # açıklama (
        a: x,
    }
    TR_return [s, t]
}

TR_func yerel() {
    TR_var a: TR_Int = 1
    TR_return a
}

TR_func tipli(a: TR_String) {
    TR_return 1
}
`

// Kural: sözlük anahtarı kararı satır metninden değil sözdizimi ağacından verilir: yalnızca sözlük değişmezinin
// tırnaksız anahtarı "sözlük anahtarı"dır; yapı alanı, adlı argüman ve tip yazımı da `a:` yazılır ama değildir, dizgedeki
// ve yorumdaki parantez kararı değiştirmez (bağımsız denetim grup 6, R3: bunlar sessizce "kendiliğinden değişmez"
// etiketi alıyordu ya da gerçek anahtar kaçıyordu).
func TestDictKeysComeFromTheSyntaxTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), trText(filepath.Join(dir, "m.tr"), dictKeySource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil || len(idx.Skipped) > 0 {
		t.Fatal(err, idx.Skipped)
	}
	res := NewReader(idx).RenameImpact("parameter:m.tr#hedef.a", "yeni", RenameOptions{Root: dir})
	var keys []string
	for _, c := range res.Likely {
		if c.Role == RoleDictKey {
			keys = append(keys, c.Pos.String())
		}
	}
	if want := "m.tr:" + srcAt(t, dictKeySource, `TR_var s = {ad: "(", a: x}`, "a", 3) + " m.tr:" + srcAt(t, dictKeySource, "        a: x,", "a", 1); strings.Join(keys, " ") != want {
		t.Errorf("dictionary keys = %v, want %s:\n%s", keys, want, RenameText(res))
	}
}

// Kural: ayrıştırıcının "tırnaksız sözlük anahtarı" kaydı (parsed.dictKey) sözdizimi ağacıyla aynıdır: depodaki her
// .tr dosyasında sözlük değişmezinin tırnaksız yazılmış her anahtarı kayıtlıdır, kayıtlı her yer de böyle bir anahtardır.
// Ayrıştırıcıya sözlük yazan yeni bir yol eklenip kayıt unutulursa yeniden adlandırma o anahtarları kaçırır.
func TestDictKeyRecordMatchesTheSyntaxTree(t *testing.T) {
	root := reporoot.Root()
	files := ListFiles(root, []string{"lib", "examples", "tests", "tools"}, nil, TrackedFiles, func(rel string) bool {
		return strings.HasSuffix(rel, ".tr")
	})
	checked := 0
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		res := readFile(string(data), false)
		if len(res.errs) > 0 {
			continue // ayrıştırılamayan (olumsuz örnek) dosyada ağaç yarımdır
		}
		lines := res.lines
		want := map[[2]int]bool{}
		ast.Walk(res.program, func(n any) bool {
			if d, ok := n.(*ast.DictLiteral); ok {
				for _, p := range d.Pairs {
					k, ok := p.Key.(*ast.StringLiteral)
					if !ok || k.Token.Line < 1 || k.Token.Line > len(lines) {
						continue
					}
					if row := []rune(lines[k.Token.Line-1]); k.Token.Column >= 1 && k.Token.Column <= len(row) && row[k.Token.Column-1] != '"' {
						want[[2]int{k.Token.Line, k.Token.Column}] = true
					}
				}
			}
			return true
		})
		for at := range want {
			checked++
			if !res.dictKey(at[0], at[1]) {
				t.Errorf("%s:%d:%d: a bare dictionary key is not recorded as one", rel, at[0], at[1])
			}
		}
		for at, role := range res.roles {
			if role == parser.RoleDictKey && !want[at] {
				t.Errorf("%s:%d:%d: recorded as a dictionary key but the syntax tree has no bare key there", rel, at[0], at[1])
			}
		}
	}
	if checked < 50 {
		t.Errorf("only %d dictionary keys checked; the repository's .tr files were not found", checked)
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
