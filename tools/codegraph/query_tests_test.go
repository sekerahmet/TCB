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
