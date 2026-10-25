package checker

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// builtinCache etkin dilin yerleşik adlarını bir kez hesaplar.
var builtinCache struct {
	sync.Mutex
	lang  *codes.Language
	names map[string]bool
}

// builtinNames sözlükteki yerleşiklerin (fn, const) ve tip adlarının (type; ifadede tip
// değeridir: tip(x) == Tam) kabul edilen adlarıdır; denetimde bu adlar her kapsamda
// tanımlıdır (çalışma anında genel kapsama kaydedilirler).
func builtinNames() map[string]bool {
	o := &builtinCache
	o.Lock()
	defer o.Unlock()
	if o.lang != codes.Active() {
		m := map[string]bool{}
		for _, k := range codes.Codes() {
			if k.IsBuiltin() || k.Namespace() == codes.NamespaceType {
				for _, y := range codes.Spellings(k) {
					m[y] = true
				}
			}
		}
		o.lang, o.names = codes.Active(), m
	}
	return o.names
}
