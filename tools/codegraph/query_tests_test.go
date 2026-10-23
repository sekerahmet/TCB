package main

import (
	"strings"
	"testing"
)

var measurementFiles = map[string]string{
	"p/p.go": "package p\n\nfunc Work() int { return 1 }\n\nfunc Wrapped() int { return Work() }\n",
	"p/p_test.go": `package p

import "testing"

func TestWork(t *testing.T) { _ = Work() }

func BenchmarkWork(b *testing.B) { _ = Work() }

func BenchmarkWrapped(b *testing.B) { _ = Wrapped() }
`,
}

// Kural: `tests-for` ölçümleri (Benchmark* ve tools/bench) testlerin arasında değil, "measurements, not tests" başlığı
// altında verir; doğrudan ve dolaylı ölçüm ikisi de oraya gider, testler başlığın üstünde kalır.
func TestTestsForKeepsMeasurementsApartFromTests(t *testing.T) {
	root := auditModule(t, measurementFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "p.Work", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	got := out.String()
	head, measured, found := strings.Cut(got, "measurements, not tests")
	if !found {
		t.Fatalf("no measurements section:\n%s", got)
	}
	if !strings.Contains(head, "p.TestWork") || strings.Contains(head, "Benchmark") {
		t.Errorf("tests above the heading, benchmarks never:\n%s", got)
	}
	if !strings.Contains(measured, "p.BenchmarkWork") || !strings.Contains(measured, "p.BenchmarkWrapped") || strings.Contains(measured, "TestWork") {
		t.Errorf("both benchmarks (direct and through Wrapped) under the heading:\n%s", got)
	}
}

// Kural: tests-for çıkarımla bulunan testleri kanıt gücüne göre verir: kısa zincir ve aynı paket önce; zincir sınırının
// yarısından uzun zincirle bulunanlar en sonda "weak evidence" başlığı altında. Ad kodu üzerinden bir adımda bulunan
// aynı paket testi, sekiz adımlık bir zincirle bulunan başka paket testinden önce gelir.
func TestTestsForOrdersInferredTestsByEvidence(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{ID: "target", Kind: kindFunc, Name: "p.target", File: "p/x.go"},
		{ID: "name:Y010", Kind: kindName, Name: "Y010", Detail: "güncelle"},
		{ID: "near", Kind: kindFunc, Name: "p.TestNear", File: "p/x_test.go"},
		{ID: "far", Kind: kindFunc, Name: "q.TestFar", File: "q/y_test.go"},
	}}
	g.Edges = append(g.Edges,
		Edge{From: "target", To: "name:Y010", Kind: edgeUsesName, Confidence: extracted},
		Edge{From: "near", To: "name:Y010", Kind: edgeTests, Confidence: inferred})
	prev := "far"
	for i := 1; i < testMaxSteps; i++ {
		id := "q" + string(rune('a'+i))
		g.Nodes = append(g.Nodes, Node{ID: id, Kind: kindFunc, Name: "q." + id, File: "q/y.go"})
		g.Edges = append(g.Edges, Edge{From: prev, To: id, Kind: edgeCalls, Confidence: extracted})
		prev = id
	}
	g.Edges = append(g.Edges, Edge{From: prev, To: "target", Kind: edgeCalls, Confidence: extracted})
	ix := newIndex(g)
	var out strings.Builder
	ix.printTestsFor(&out, ix.byID["target"], 12)
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 3 || !strings.Contains(lines[2], "p.TestNear") {
		t.Errorf("the one-step same-package test must be the first test line:\n%s", out.String())
	}
	_, weak, found := strings.Cut(out.String(), "weak evidence")
	if !found || !strings.Contains(weak, "q.TestFar") {
		t.Errorf("the eight-step chain must be under weak evidence, last:\n%s", out.String())
	}
}

// Kural: test dosyasında işlev değeri olarak kullanılan (tablo öğesi, değişkene atanan) işlevin testleri "no tests found"
// değil, "tests that use it as a function value" grubunda gelir: kullanan test kendisi, tablo ise onu okuyan test (bağımsız
// denetim Y2). Test dosyasında hiç geçmeyen işlev testsiz kalır.
func TestTestsForFindsTestsThatUseAFunctionAsAValue(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p/p.go": "package p\n\nfunc Square(x int) int { return x * x }\n\nfunc Cube(x int) int { return x * x * x }\n\nfunc Unused(x int) int { return x }\n",
		"p/p_test.go": `package p

import "testing"

var ops = []func(int) int{Square}

func TestOps(t *testing.T) {
	for _, f := range ops {
		_ = f(2)
	}
}

func TestPowerValue(t *testing.T) {
	f := Cube
	_ = f(2)
}
`,
	})
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	ask := func(name string) string {
		var out, errb strings.Builder
		if code := run([]string{"tests-for", name, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
			t.Fatalf("tests-for %s: %d %s", name, code, errb.String())
		}
		return out.String()
	}
	if got := ask("p.Square"); !strings.Contains(got, "function value") || !strings.Contains(got, "p.TestOps") || !strings.Contains(got, "via p.ops") {
		t.Errorf("Square is tested through the table ops:\n%s", got)
	}
	if got := ask("p.Cube"); !strings.Contains(got, "p.TestPowerValue") || !strings.Contains(got, "in the test itself") {
		t.Errorf("Cube is used as a value by the test itself:\n%s", got)
	}
	if got := ask("p.Unused"); !strings.Contains(got, "no tests found") {
		t.Errorf("Unused is not in any test file:\n%s", got)
	}
}

// Kural: `tests-for <Go dosyası>` dosyadaki bildirimlerin testlerinin birleşimini verir (`changed`'in toplamasıyla) ve
// onların koruduğu kuralları; "bir dosyaya dokunmadan önce tests-for <dosya>" kuralı Go dosyasında boş dönmez.
func TestTestsForAGoFileListsTheTestsOfItsDeclarations(t *testing.T) {
	root := auditModule(t, measurementFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "p/p.go", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "declarations in this file: 2") || !strings.Contains(got, "p.TestWork") || strings.Contains(got, "no tests found") {
		t.Errorf("the file's two functions and their test:\n%s", got)
	}
}

// Kural: tests-for tek sıralamayla yazar: önce hedefi konu edinen testler (adı ya da belge yorumu hedefin adını anıyor,
// ya da hedefin ürettiği iletiyi bekliyor), sonra onu yalnızca kullanan doğrudan testler, sonra zincirler; her test bir
// kez, en güçlü kanıtının grubunda (deneme 8 S1: çok kullanılan bir adda onu sınayan testler en dipte kalıyordu).
func TestTestsForPutsTheTestsAboutItFirst(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p/p.go": "package p\n\nfunc Area(x int) int { return x * x }\n",
		"p/p_test.go": `package p

import "testing"

func TestUsesSomething(t *testing.T) { _ = Area(2) }

// Kare alanı kenarın karesidir.
func TestAreaOfSquare(t *testing.T) { _ = Area(3) }
`,
	})
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"tests-for", "p.Area", "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
		t.Fatalf("tests-for: %d %s", code, errb.String())
	}
	got := out.String()
	about, rest, ok := strings.Cut(got, "<- tests (")
	if !ok || !strings.Contains(about, "tests about it (1") || !strings.Contains(about, "p.TestAreaOfSquare") ||
		!strings.Contains(rest, "p.TestUsesSomething") || strings.Contains(rest, "TestAreaOfSquare") {
		t.Errorf("the test named after Area first, once; the other one under the direct tests:\n%s", got)
	}
}
