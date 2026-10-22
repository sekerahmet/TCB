package main

import (
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// docs/codes.md kod listesinden üretilenle aynı olmalı: bir kod, açıklama ya da tanım
// değiştiğinde `go run ./tools/gen/codesdoc` çalıştırılmadıysa bu test düşer.
func TestDocUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "codesdoc", docPath, func(string) ([]byte, error) { return Generate() })
}
