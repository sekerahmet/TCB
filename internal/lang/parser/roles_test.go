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
	src := `yapı Nokta {
    x: Tam
    fonksiyon uzak(a: Metin): Tam { sonuç 1 }
}
değişken n: Liste = []
değişken d = {ad: 1}
n.ekle(1)
test "t" { }
aktar "yol" olarak m
yazdır("{n.uzunluk}")
`
	p := New(lexer.New(src))
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
		{2, "x", RoleStructField},
		{2, "Tam", RoleType},
		{3, "uzak", RoleStructMethod},
		{3, "Metin", RoleType},
		{5, "Liste", RoleType},
		{6, "ad", RoleDictKey},
		{7, "ekle", RoleMember},
		{8, "test", RoleTest},
		{9, "olarak", RoleAs},
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
	// Dönüş tipi (`): Tam`) de kaydedilir: satır 3'te Tam ikinci geçiştir.
	l3 := lines[2]
	col := utf8.RuneCountInString(l3[:strings.LastIndex(l3, "Tam")]) + 1
	if roles[[2]int{3, col}] != RoleType {
		t.Errorf("3:%d dönüş tipi kaydedilmedi", col)
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
