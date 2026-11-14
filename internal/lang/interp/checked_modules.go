package interp

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// checkedModules, çalışmadan önceki modül denetiminin (CheckImportedModules) kullanıcı modülü
// ağaçlarıdır, modülün mutlak yoluna göre. Çalışma anı modülü aynı metinle okursa yeniden
// ayrıştırmaz, denetlenmiş ağacı kullanır: denetleyicinin ağaca yazdığı bilgi (TR_sum'ın boş
// listesinin öğe tipi, ast.CallExpression.ListElem) modülde de çalışır (15.18, 13). Dosya denetimden
// sonra değiştiyse metin tutmaz, modül eskisi gibi yeniden ayrıştırılır. Dilin gömülü kitaplığı
// checkedLibrary'dedir.
var checkedModules sync.Map // mutlak yol → checkedModule

// checkedLibrary dilin gömülü kitaplığının denetlenmiş modülleridir, kitaplıktaki adına göre
// ("layers.tr"). Kitaplığın metni süreç boyunca değişmez; modül süreç başına bir kez denetlenir.
var checkedLibrary sync.Map // ad → checkedModule

// checkedLibraryModule kitaplıktaki name modülünün text metniyle denetlenmiş sonucudur: ilk istekte
// CheckModule ile denetlenir, sonra saklanandır. Denetimden önce kitaplık değiştiyse (SetLibrary)
// metin tutmaz ve modül yeniden denetlenir.
func checkedLibraryModule(name, text string) *source.Result {
	if v, ok := checkedLibrary.Load(name); ok && v.(checkedModule).text == text {
		return v.(checkedModule).result
	}
	r := CheckModule(LibraryPrefix+name, text)
	checkedLibrary.Store(name, checkedModule{text: text, result: r})
	return r
}

type checkedModule struct {
	text   string
	result *source.Result
}

// rememberCheckedModule path'teki modülün text metniyle denetlenmiş sonucunu saklar.
func rememberCheckedModule(path, text string, r *source.Result) {
	checkedModules.Store(absPath(path), checkedModule{text: text, result: r})
}

// checkedModuleSource path'teki modülün text metniyle denetlenmiş sonucudur; metin denetlenenle bayt
// bayt aynı değilse yoktur.
func checkedModuleSource(path, text string) (*source.Result, bool) {
	v, ok := checkedModules.Load(absPath(path))
	if !ok {
		return nil, false
	}
	m := v.(checkedModule)
	if m.text != text {
		checkedModules.Delete(absPath(path))
		return nil, false
	}
	return m.result, true
}
