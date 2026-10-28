package interp

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ErrCyclic reports an attempt to write a list/dict that contains itself (directly or
// indirectly) to an external format (JSON, Go values): l = [1]; l.ekle(l); json_metin(l).
var ErrCyclic error = messages.New(messages.H2020)

// MaxExternalDepth is the deepest list/dict nesting that can be written to an external
// format; it equals the JSON reading limit (maxJSONDepth), so every written value can be read
// back.
const MaxExternalDepth = maxJSONDepth

// DepthError is the error returned when MaxExternalDepth is exceeded.
func DepthError() error {
	return messages.New(messages.H2021, MaxExternalDepth)
}

// ContainerWalker checks for cycles and depth while a TR value is written to an external
// format. Code that writes a value to another format (e.g. ordered JSON) calls Enter on every
// list/dict it enters and Leave when it leaves; the zero value is ready to use.
type ContainerWalker struct {
	// opened şu an yazılmakta olan (atası olunan) kapsayıcılardır; yalnızca bunlara geri
	// dönmek döngüdür.
	opened map[Value]bool
}

// Enter enters container v at the given depth; it returns an error on a cycle or excessive
// depth.
func (g *ContainerWalker) Enter(v Value, nestDepth int) error {
	if nestDepth > MaxExternalDepth {
		return DepthError()
	}
	if g.opened == nil {
		g.opened = map[Value]bool{}
	}
	if g.opened[v] {
		return ErrCyclic
	}
	g.opened[v] = true
	return nil
}

// Leave leaves container v.
func (g *ContainerWalker) Leave(v Value) { delete(g.opened, v) }
