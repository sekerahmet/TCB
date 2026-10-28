package parser

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Ağaçta düğümü olmayan adlar da kaybolmaz: tip açıklaması, üye adı, yapı alanı ve metodu, test
// ve olarak sözcükleri, sözlük anahtarı kayıt açıkken görevleriyle (satır, sütun) kaydedilir;
// kodlu biçime çevirici bu kayda dayanır. Yerleştirme içindeki adlar da dosyadaki konumlarıyla gelir.
func TestRolesRecordedForNamesWithoutNodes(t *testing.T) {
	src := `aktar "yol" olarak m
yapı Nokta {
    x: Tam
    fonksiyon uzak(a: Metin): Tam { sonuç 1 }
}
değişken n: Liste = []
değişken d = {ad: 1}
n.ekle(1)
test "t" { }
yazdır("\(n.uzunluk)")
`
	p := New(trLexer(src))
	p.RecordRoles()
	p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	roles := p.Roles()
	lines := strings.Split(src, "\n")
	want := []struct {
		line   int    // 1 tabanlı satır
		needle string // satırdaki sözcük (ilk geçtiği yer)
		role   NameRole
	}{
		{3, "x", RoleStructField},
		{3, "Tam", RoleType},
		{4, "uzak", RoleStructMethod},
		{4, "Metin", RoleType},
		{6, "Liste", RoleType},
		{7, "ad", RoleDictKey},
		{8, "ekle", RoleMember},
		{9, "test", RoleTest},
		{1, "olarak", RoleAs},
		{10, "uzunluk", RoleMember}, // yerleştirmenin içi, dosyadaki gerçek yeriyle
	}
	for _, w := range want {
		idx := strings.Index(lines[w.line-1], w.needle)
		if idx < 0 {
			t.Fatalf("satır %d'de %q yok", w.line, w.needle)
		}
		col := utf8.RuneCountInString(lines[w.line-1][:idx]) + 1
		if got := roles[[2]int{w.line, col}]; got != w.role {
			t.Errorf("%d:%d (%s): görev %v, beklenen %v", w.line, col, w.needle, got, w.role)
		}
	}
	// Dönüş tipi (`): Tam`) de kaydedilir: satır 4'te Tam ikinci geçiştir.
	l4 := lines[3]
	col := utf8.RuneCountInString(l4[:strings.LastIndex(l4, "Tam")]) + 1
	if roles[[2]int{4, col}] != RoleType {
		t.Errorf("4:%d dönüş tipi kaydedilmedi", col)
	}
	seen := map[NameRole]bool{}
	for _, r := range roles {
		seen[r] = true
	}
	for _, r := range []NameRole{RoleType, RoleMember, RoleStructField, RoleStructMethod, RoleTest, RoleAs, RoleDictKey} {
		if !seen[r] {
			t.Errorf("%v görevi hiç kaydedilmedi", r)
		}
	}
}

// Kayıt kapalıyken (sıradan ayrıştırma) hiçbir şey kaydedilmez.
func TestRolesOffByDefault(t *testing.T) {
	p := New(lexer.New("değişken x: Tam = 1\n"))
	p.ParseProgram()
	if len(p.Roles()) != 0 {
		t.Errorf("kayıt kapalıyken görev tutulmamalı: %v", p.Roles())
	}
}
