package main

import (
	"strings"
	"testing"
)

const docSource = `package p

// Active programın okunduğu dildir: her zaman Türkçe.
// İkinci satır da gelir.
//
// İkinci paragraf gelmez.
var Active = 1

// Kind bir türdür.
type Kind int

// Limit tek belirtimli sabittir (yorum bildirime bağlıdır).
const Limit = 3

const (
	// A grup içi sabittir.
	A = 1
	B = 2 // B satır sonu yorumudur.
	C = 3
)

// Store alanları belgelidir.
type Store struct {
	// Name alanın doc yorumudur.
	Name string
	Size int // Size satır sonu yorumudur.
}

// Reader arayüzdür.
type Reader interface {
	// Read yöntem yorumudur.
	Read() int
}

// Run bir işlevdir.
func Run() {}

// Close bir metottur.
func (s *Store) Close() {}

//go:noinline
func Plain() {}
`

// Kural: her Go düğümü (değişken, tür, tek belirtimli ve gruplu sabit, alan, arayüz yöntemi, işlev, metot) belge
// yorumunun ilk paragrafını (en çok 3 satır) taşır, yoksa satır sonu yorumunu; `query` onu "doc:" satırında yazar.
// Yönergeler (//go:noinline) yorum değildir.
func TestQueryShowsGoDocComments(t *testing.T) {
	root := auditModule(t, map[string]string{"p/p.go": docSource})
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	for name, want := range map[string]string{
		"p.Active":      "doc: Active programın okunduğu dildir: her zaman Türkçe. İkinci satır da gelir.\n",
		"p.Kind":        "doc: Kind bir türdür.\n",
		"p.Limit":       "doc: Limit tek belirtimli sabittir (yorum bildirime bağlıdır).\n",
		"p.A":           "doc: A grup içi sabittir.\n",
		"p.B":           "doc: B satır sonu yorumudur.\n",
		"p.Store.Name":  "doc: Name alanın doc yorumudur.\n",
		"p.Store.Size":  "doc: Size satır sonu yorumudur.\n",
		"p.Reader.Read": "doc: Read yöntem yorumudur.\n",
		"p.Run":         "doc: Run bir işlevdir.\n",
		"p.Store.Close": "doc: Close bir metottur.\n",
	} {
		out.Reset()
		if code := run([]string{"query", name, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
			t.Fatalf("query %s: %d %s", name, code, errb.String())
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("query %s lacks %q:\n%s", name, want, out.String())
		}
	}
	for _, name := range []string{"p.C", "p.Plain"} {
		out.Reset()
		run([]string{"query", name, "--root", root, "--no-rebuild"}, &out, &errb)
		if strings.Contains(out.String(), "doc:") {
			t.Errorf("%s has no comment, so no doc line:\n%s", name, out.String())
		}
	}
	if strings.Contains(out.String(), "İkinci paragraf") {
		t.Error("only the first paragraph is kept")
	}
}
