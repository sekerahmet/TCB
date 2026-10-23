package main

import (
	"go/ast"
	"go/types"
	"path"
	"strings"
)

// Bu dosya alt süreç testlerini bağlar. Bir test dosyası `exec.Command("go", "build", ..., paket)` ile bir ana paketi
// derleyip ikiliyi çalıştırıyorsa ikilinin içindeki yol işlev işlev izlenemez: ikiliyi koşan testler paketin hangi
// işlevine vardığını grafiğe göstermez. Derleyen işlevden pakete `builds` kenarı çekilir; `tests-for` o paketin
// işlevlerinde bu testleri tek satırda sayar ("covered by subprocess tests", query_tests.go), "no tests found" demez
// (bağımsız denetim Y5).

// buildCall test dosyasındaki `go build` çağrısının derlediği modül paketine `builds` kenarı çeker. Paket bir modül
// yolu ("github.com/.../cmd/tr") ya da test dosyasının klasörüne göre "." / "./x" olabilir.
func (x *goExtractor) buildCall(fc *fileCtx, owner string, call *ast.CallExpr) {
	id := nameIdent(call.Fun)
	if !fc.isTest || id == nil || (id.Name != "Command" && id.Name != "CommandContext") {
		return
	}
	if fn, ok := fc.u.Info.Uses[id].(*types.Func); !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os/exec" {
		return
	}
	args := call.Args
	if id.Name == "CommandContext" && len(args) > 0 {
		args = args[1:]
	}
	if len(args) < 3 || constString(fc, args[0]) != "go" || constString(fc, args[1]) != "build" {
		return
	}
	target := constString(fc, args[len(args)-1])
	switch {
	case target == "." || strings.HasPrefix(target, "./"):
		target = x.m.Path + "/" + path.Join(path.Dir(fc.rel), target)
	case target != x.m.Path && !strings.HasPrefix(target, x.m.Path+"/"):
		return
	}
	x.b.addEdge(owner, pkgID(target), edgeBuilds, extracted, fc.rel, x.line(call.Pos()))
}
