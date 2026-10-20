package reporoot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Kök, çalışma klasöründen yukarı doğru go.mod'un bulunduğu ilk klasördür: mutlak yoldur,
// go.mod'u içerir ve çalışma klasörünün üstündedir (ya da kendisidir).
func TestRootHoldsGoMod(t *testing.T) {
	root := Root()
	if !filepath.IsAbs(root) {
		t.Errorf("kök mutlak yol olmalı: %q", root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Errorf("kökte go.mod yok: %v", err)
	}
	wd, _ := os.Getwd()
	if rel, err := filepath.Rel(root, wd); err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("çalışma klasörü (%s) kökün (%s) altında olmalı", wd, root)
	}
	if Root() != root {
		t.Error("Root her çağrıda aynı yolu vermeli")
	}
}
