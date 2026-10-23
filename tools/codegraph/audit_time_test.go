package main

import (
	"strings"
	"testing"
)

const timeTestSource = `package p

import (
	"testing"
	"time"
)

const ceiling = 2 * time.Second

func work() {}

func TestDirectSince(t *testing.T) {
	begin := time.Now()
	work()
	if time.Since(begin) > 3*time.Second {
		t.Fatal("slow")
	}
}

func TestVariable(t *testing.T) {
	begin := time.Now()
	work()
	d := time.Since(begin)
	if d > 5*time.Second {
		t.Fatal("slow")
	}
}

func TestSubAndSeconds(t *testing.T) {
	start := time.Now()
	work()
	end := time.Now()
	elapsed := end.Sub(start)
	if elapsed.Seconds() > 2 {
		t.Fatal("slow")
	}
}

func TestPackageConstant(t *testing.T) {
	begin := time.Now()
	work()
	if time.Since(begin) >= ceiling {
		t.Fatal("slow")
	}
}

// Büyümeye bakan test: iki ölçüm birbiriyle karşılaştırılır, sabit tavan yok.
func TestScales(t *testing.T) {
	a := time.Now()
	work()
	small := time.Since(a)
	b := time.Now()
	work()
	big := time.Since(b)
	if big > 4*small {
		t.Fatal("does not scale")
	}
}

// Sabit tavan değil: süreyle ilgisi olmayan bir karşılaştırma.
func TestPlain(t *testing.T) {
	n := 3
	if n > 2 {
		work()
	}
}
`

// Duvar saati tavanları: dosya:satır, test adı ve sabit; iki ölçümü birbirine bağlayan karşılaştırma ve
// süreyle ilgisiz karşılaştırmalar listelenmez.
func TestTimeCeilingsListsConstantCeilings(t *testing.T) {
	root := auditModule(t, map[string]string{"p.go": "package p\n", "p_test.go": timeTestSource})
	res := auditJSON(t, "time-ceilings", root)
	got := map[string]string{}
	for _, v := range res["ceilings"].([]any) {
		m := v.(map[string]any)
		got[m["test"].(string)] = m["constant"].(string)
	}
	want := map[string]string{"TestDirectSince": "3s", "TestVariable": "5s", "TestSubAndSeconds": "2", "TestPackageConstant": "2s"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: ceiling %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	for _, quiet := range []string{"TestScales", "TestPlain"} {
		if _, bad := got[quiet]; bad {
			t.Errorf("%s is not a wall-clock ceiling", quiet)
		}
	}
	text := auditText(t, "time-ceilings", root)
	if !strings.Contains(text, "4 wall-clock ceiling(s)") || !strings.Contains(text, "p_test.go:15") || !strings.Contains(text, "time.Since(begin) > 3*time.Second") {
		t.Errorf("text:\n%s", text)
	}
}
