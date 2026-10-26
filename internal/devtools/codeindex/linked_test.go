package codeindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

const linkedSource = `yapı Dikkat {
    baş
    fonksiyon kurucu(boyut, baş) {
        kendi.baş = baş
    }
}

yapı Blok {
    dikkat
    fonksiyon kurucu(boyut, baş) {
        kendi.dikkat = Dikkat(boyut, baş)
    }
}

yapı Model {
    ayar
    blok
    fonksiyon kurucu(boyut, baş = 2) {
        kendi.ayar = {boyut: boyut, baş: baş}
        kendi.blok = Blok(boyut, baş: baş)
    }
}
`

// Kural: aynı adla iletilen parametre (adlı ya da konumlu) ve aynı adlı alana atanan parametre bağlı bildirimdir:
// yeniden adlandırma bunları kesin değil ayrı bir grupta verir (bağlı kümenin bütünü, geçişli). Aynı adlı sözlük
// anahtarı veri biçimidir: olası grupta "sözlük anahtarı" rolüyle durur, kendiliğinden değişmez (deneme 7 R2, R3).
func TestRenameShowsLinkedDeclarationsAndDictKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(linkedSource), 0o644); err != nil {
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
	for _, c := range res.Likely {
		if c.Pos.Line == 19 && c.Role == RoleDictKey && c.Match == MatchDataFormat {
			dictKey = true
		}
		if c.Pos.Line == 19 && c.Role != RoleDictKey && strings.Contains(c.Context, "baş: baş") && c.Pos.Col < 30 {
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
const dictKeySource = `yapı Kişi {
    a: Metin
}

fonksiyon hedef(a) {
    sonuç a
}

fonksiyon çağır(nesne) {
    sonuç nesne.koş(")", a: 1)
}

fonksiyon sözlükler(x) {
    değişken s = {ad: "(", a: x}
    değişken t = {
        // açıklama (
        a: x,
    }
    sonuç [s, t]
}

fonksiyon yerel() {
    değişken a: Tam = 1
    sonuç a
}

fonksiyon tipli(a: Metin) {
    sonuç 1
}
`

// Kural: sözlük anahtarı kararı satır metninden değil sözdizimi ağacından verilir: yalnızca sözlük değişmezinin
// tırnaksız anahtarı "sözlük anahtarı"dır; yapı alanı, adlı argüman ve tip yazımı da `a:` yazılır ama değildir, dizgedeki
// ve yorumdaki parantez kararı değiştirmez (bağımsız denetim grup 6, R3: bunlar sessizce "kendiliğinden değişmez"
// etiketi alıyordu ya da gerçek anahtar kaçıyordu).
func TestDictKeysComeFromTheSyntaxTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(dictKeySource), 0o644); err != nil {
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
	if want := "m.tr:14:28 m.tr:17:9"; strings.Join(keys, " ") != want {
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
		res := readFile(string(data))
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
	src := "fonksiyon başlat(n) {\n    sonuç n\n}\n\nyapı Katman {\n    w\n    fonksiyon kurucu(n) {\n        kendi.w = başlat(n)\n    }\n}\n"
	test := "test \"katman kurulur\" {\n    değişken k = Katman(2)\n    doğrula(k.w == 2)\n}\n"
	for name, text := range map[string]string{"m_test.tr": src + "\n" + test} { // aynı dosya: aktarma gerekmez
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
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
const forwardSource = `fonksiyon g(b, a) {
    sonuç b + a
}

fonksiyon f(a) {
    sonuç g(a, 1)
}

fonksiyon h(a) {
    sonuç g(1, a)
}

yapı Kişi {
    ad
    fonksiyon kurucu(ad, adlar) {
        döngü ad içinde adlar {
            kendi.ad = ad
        }
    }
}

yapı Yer {
    ad
    fonksiyon kurucu(ad) {
        kendi.ad = ad
    }
}
`

// Kural: konumlu argüman yalnızca çağrılanın aynı sıradaki parametresi aynı adı taşıyorsa bağlıdır; gövdede yeniden
// bağlanan (gölgelenen) parametre ne iletimde ne alana atamada bağ kurar (bağımsız denetim grup 6, R2).
func TestForwardsFollowPositionAndShadowing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(forwardSource), 0o644); err != nil {
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
