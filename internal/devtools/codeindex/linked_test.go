package codeindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	res := r.RenameImpact("parameter:m.tr#Model.kurucu.baş", "bakış", RenameOptions{Root: dir})
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
	var linked []string
	for _, n := range res.Linked {
		linked = append(linked, n.ID)
	}
	want := "field:m.tr#Dikkat.baş parameter:m.tr#Blok.kurucu.baş parameter:m.tr#Dikkat.kurucu.baş"
	if strings.Join(linked, " ") != want {
		t.Errorf("linked = %v, want %s", linked, want)
	}
	for _, c := range res.Certain {
		if c.Node != "parameter:m.tr#Model.kurucu.baş" {
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

// Kural: sözlük anahtarı, ardından ':' gelen ve en yakın açık parantezi '{' olan addır; '(' içindeki `ad:` adlı
// argümandır, ':=' atamadır.
func TestIsDictKey(t *testing.T) {
	lines := []string{"x = {a: 1,", "    b: f(c: 2)}", "d := 3"}
	for _, c := range []struct {
		line, col int
		word      string
		want      bool
	}{{1, 6, "a", true}, {2, 5, "b", true}, {2, 13, "c", false}, {3, 1, "d", false}} {
		if got := isDictKey(lines, Pos{Line: c.line, Col: c.col}, c.word); got != c.want {
			t.Errorf("isDictKey(%d:%d %s) = %v, want %v", c.line, c.col, c.word, got, c.want)
		}
	}
}
