package main

import "github.com/sekerahmet/TCB/internal/devtools/goload"

// Modül yükleme ortak pakettedir (tools/internal/goload; tools/impact ile aynı yöntem). codegraph
// belge yorumlarını ve alan/metot seçimlerini okur.
type (
	module = goload.Module
	unit   = goload.Unit
)

func load(dir string) (*module, error) {
	return goload.Load(dir, goload.Options{Comments: true, Selections: true, SkipIgnored: true})
}
