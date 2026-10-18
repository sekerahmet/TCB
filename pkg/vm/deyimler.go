package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

// aktar 'aktar' deyimini çalıştırır. Modül, genel kapsamın altında kendi ortamında çalışır
// ve ortamının tüm adları dışa aktarılır. takma, 'aktar ... olarak ad' biçiminde takma adı
// çağıranın kapsamına tanımlar; takma adsız aktarımda adlar çağıranın ortamına adla
// tanımlanır.
func (v *VM) aktar(s *ast.ImportStatement, env *Environment, takma func(*Environment, Value, bool)) (Value, flowSignal, error) {
	path := s.Path
	if !strings.HasSuffix(path, ".tr") {
		path += ".tr"
	}
	resolvedPath := path
	if _, err := os.Stat(resolvedPath); err != nil && !filepath.IsAbs(path) && v.SourceFile != "" && v.SourceFile != "<derlenmiş-uygulama>" {
		candidate := filepath.Join(filepath.Dir(v.SourceFile), path)
		if _, err2 := os.Stat(candidate); err2 == nil {
			resolvedPath = candidate
		}
	}
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		absPath = resolvedPath
	}

	v.importedMu.Lock()
	if v.importing[absPath] {
		v.importedMu.Unlock()
		return nil, signalNone, fmt.Errorf("Satır %d: %q modülünde döngüsel aktarım (circular import) tespit edildi", s.Line(), path)
	}
	cachedMod, hasCached := v.Imported[absPath]
	if !hasCached {
		v.importing[absPath] = true
	}
	v.importedMu.Unlock()

	if hasCached {
		if takma != nil {
			takma(env, cachedMod, true)
			return cachedMod, signalNone, nil
		}
		for _, k := range cachedMod.Order {
			env.Define(k, cachedMod.Pairs[k], false)
		}
		return cachedMod, signalNone, nil
	}

	defer func() {
		v.importedMu.Lock()
		delete(v.importing, absPath)
		v.importedMu.Unlock()
	}()

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, signalNone, fmt.Errorf("Satır %d: %q modülü okunamadı: %v", s.Line(), path, err)
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, signalNone, fmt.Errorf("Satır %d: %q modülünde sözdizimi hatası: %s", s.Line(), path, p.Errors()[0])
	}

	// Modül, genel kapsamın altında kendi kapsamında derlenir
	c := v.yeniDerleyici()
	modKapsam := altKapsam(c.kok)
	modKapsam.onTanimla(prog.Statements)
	modKod := c.deyimler(modKapsam, prog.Statements)
	modEnv := yeniOrtam(v.GlobalEnv, modKapsam.bilgi)

	prevSource := v.SourceFile
	v.SourceFile = resolvedPath
	val, sig, execErr := modKod.calistir(v, modEnv)
	v.SourceFile = prevSource
	if execErr != nil {
		return nil, signalNone, execErr
	}

	modDict := NewDictVal()
	modEnv.EachLocal(func(k string, exportedVal Value) {
		modDict.Set(k, exportedVal)
	})

	v.importedMu.Lock()
	v.Imported[absPath] = modDict
	v.importedMu.Unlock()

	if takma != nil {
		takma(env, modDict, true)
		return modDict, signalNone, nil
	}
	for _, k := range modDict.Order {
		env.Define(k, modDict.Pairs[k], false)
	}
	return val, sig, nil
}
