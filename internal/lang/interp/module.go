package interp

import (
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Source is a source file: the main program or a module brought in with 'aktar'. The source of
// running code belongs to the task (VM view) and the call frame: a function defined in a module
// runs in its own file wherever it is called from (relative paths are also searched relative to
// its directory, and its errors are written with its name and lines).
type Source struct {
	Path string // dosyanın çözülmüş yolu (kitaplık modülünde "tr/katman.tr")
	Name string // hata mesajlarında gösterilen yol (mümkünse çalışma klasörüne göre)
	Code string
	// Embedded says the source lives in the language's embedded library, not on disk (11.6):
	// relative file paths are not searched relative to its directory, and a relative 'aktar' is
	// resolved inside the library.
	Embedded bool
}

// shownPath bir dosya yolunu hata mesajında gösterilecek biçime çevirir: çalışma
// klasörünün altındaysa ona göre, değilse mutlak; ayırıcı '/'.
func shownPath(filePath string) string {
	if filePath == "" || strings.HasPrefix(filePath, "<") {
		return filePath
	}
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return filepath.ToSlash(filePath)
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, absolute); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(absolute)
}

// mainSource ana programın kaynağıdır (SourceFile ve SourceCode'dan).
func (v *VM) mainSource() *Source {
	return &Source{Path: v.SourceFile, Name: shownPath(v.SourceFile), Code: v.SourceCode}
}

// ModuleError is an error of a file (a module, or code of it called from another file). The
// messages refer to that file's lines and start with the file's name; the code frame is drawn
// from that file's source (see ErrorItems).
//
// It has two forms: a runtime error (Err) and the module's checks before loading (Kind,
// Errors): no line of a module with a syntax or type error runs.
type ModuleError struct {
	Source *Source
	Err    error

	Kind       string  // source.StageSyntax ya da source.StageType (denetim hatalarında)
	Errors     []error // denetim hataları, modülün satırlarıyla
	ModulePath string  // 'aktar'da yazılan yol
	ImportLine int     // 'aktar' deyiminin satırı
	Importer   *Source // 'aktar' deyiminin bulunduğu dosya (nil: ana program)
}

func (e *ModuleError) Error() string {
	if len(e.Errors) == 0 {
		return e.Source.Name + ", " + e.Err.Error()
	}
	pieces := make([]string, len(e.Errors))
	for i, h := range e.Errors {
		pieces[i] = e.Source.Name + ", " + h.Error()
	}
	return e.header() + ": " + strings.Join(pieces, "; ")
}

func (e *ModuleError) Unwrap() error { return e.Err }

// header denetim hatasının 'aktar' satırını gösteren ilk bölümüdür.
func (e *ModuleError) header() string {
	if e.Importer != nil {
		return messages.Text(messages.H2002, e.Importer.Name, e.ImportLine, e.ModulePath, e.Kind)
	}
	return messages.Text(messages.H2003, e.ImportLine, e.ModulePath, e.Kind)
}

// ErrorItem is one error line to show the user and the code frame drawn from the file it
// belongs to (the frame is empty when the position is unknown).
type ErrorItem struct {
	Message string
	Frame   string
}

// ErrorItems splits a runtime error into items to show the user. anaKod is the main program's
// source; the frame of an error that belongs to a module is drawn from the module's source.
// Each of the module's check errors becomes its own item after the 'aktar' line.
func ErrorItems(err error, mainCode string) []ErrorItem {
	message := err.Error()
	var mh *ModuleError
	if !errors.As(err, &mh) {
		return []ErrorItem{{message, turkish.ErrorFrame(mainCode, err)}}
	}
	if len(mh.Errors) == 0 {
		return []ErrorItem{{message, turkish.ErrorFrame(mh.Source.Code, mh.Err)}}
	}
	importerCode := mainCode
	if mh.Importer != nil {
		importerCode = mh.Importer.Code
	}
	b := mh.header()
	entries := []ErrorItem{{b, turkish.LineFrame(importerCode, mh.ImportLine)}}
	for _, h := range mh.Errors {
		entries = append(entries, ErrorItem{mh.Source.Name + ", " + h.Error(), turkish.ErrorFrame(mh.Source.Code, h)})
	}
	return entries
}

// sourceError k dosyasında çalışan koddan çıkan konumlu bir hatayı o dosyaya bağlar
// (k nil ise ana program). Zaten bir dosyaya bağlı hata ve konumsuz hatalar ('fırlat' ile
// atılan değerler: yakalayan onları olduğu gibi görür) değişmez.
func (v *VM) sourceError(k *Source, err error) error {
	var mh *ModuleError
	if errors.As(err, &mh) || !HasLine(err) {
		return err
	}
	if k == nil {
		k = v.mainSource()
	}
	return &ModuleError{Source: k, Err: err}
}

// callInSource başka bir dosyada tanımlanmış fonksiyonu kendi dosyasının bağlamında
// çağırır; hatası o dosyaya bağlanır. Aynı dosyadaki çağrılar buraya uğramaz.
func (v *VM) callInSource(fn *FuncVal, fk *funcCode, fnEnv *Environment, self Value, args []Value, nargs int, named map[string]Value, line int) (Value, error) {
	prev := v.source
	v.source = fk.source
	defer func() { v.source = prev }()
	val, err := v.callPrepared(fn, fk, fnEnv, self, args, nargs, named, line)
	if err != nil {
		return nil, v.sourceError(fk.source, err)
	}
	return val, nil
}

// CheckModule checks a module's source (the whole file) before running it: it parses it as a
// module (a top-level 'sonuç' is an error) and type-checks it; on errors the result's Stage
// ("sözdizimi" or "tip denetimi") and Errors are set. 'aktar', 'tr kontrol' and 'tr derle' use
// the same check. Since a module runs under the importer's global scope (11.1), genelAdlar
// count as defined in the module, and a name defined nowhere else is an undefined-name error
// (5.14); dinamik says the global names are not fully known (then undefined names are not
// reported).
func CheckModule(text string, globalNames []string, dynamic bool) *source.Result {
	return source.Prepare(text, source.Options{Module: true, TypeCheck: true,
		InGlobalScope: true, GlobalNames: globalNames, GlobalDynamic: dynamic})
}

// moduleLoading yüklenmekte olan bir modüldür; aynı modülü isteyen başka görevler yüklemenin
// bitmesini bekler.
type moduleLoading struct {
	finished chan struct{}
	owner    *VM // the task loading the module
	mod      *DictVal
	err      error
}

// imp 'aktar' deyimini çalıştırır. Modül, genel kapsamın altında kendi ortamında çalışır
// ve ortamının tüm adları dışa aktarılır. takma, 'aktar ... olarak ad' biçiminde takma adı
// çağıranın kapsamına tanımlar; takma adsız aktarımda adlar çağıranın ortamına adla
// tanımlanır.
func (v *VM) imp(s *ast.ImportStatement, env *Environment, alias func(*Environment, Value, bool)) (Value, flowSignal, error) {
	path := s.Path
	if !strings.HasSuffix(path, ".tr") {
		path += ".tr"
	}
	mod, err := v.loadModule(s, path)
	if err != nil {
		return nil, signalNone, err
	}
	if alias != nil {
		alias(env, mod, true)
		return mod, signalNone, nil
	}
	for _, k := range mod.Order {
		env.Define(k, mod.Pairs[k], false)
	}
	return mod, signalNone, nil
}

// loadModule modülü (gerekirse) yükler. Bir modül program boyunca bir kez çalışır.
// Başka bir görev aynı modülü yüklüyorsa bitmesi beklenir; yükleyen görevin kendi aktarım
// zinciri modülü içeriyorsa bu döngüsel bir aktarımdır.
func (v *VM) loadModule(s *ast.ImportStatement, path string) (*DictVal, error) {
	if err := CheckPath(codes.Name(codes.A070), path); err != nil {
		return nil, messages.At(s.Line(), 0, err)
	}
	dest, searched, err := v.moduleTarget(s, path)
	if err != nil {
		return nil, err
	}
	keyStr := dest.Path
	if !dest.Embedded {
		if absolute, err := filepath.Abs(dest.Path); err == nil {
			keyStr = absolute
		}
	}

	v.isImported.Lock()
	if mod, ok := v.Imported[keyStr]; ok {
		v.isImported.Unlock()
		return mod, nil
	}
	if y := v.loaded[keyStr]; y != nil {
		if slices.Contains(v.loading, keyStr) {
			v.isImported.Unlock()
			return nil, messages.At(s.Line(), 0, messages.New(messages.H2481, path))
		}
		return v.waitForModule(s, path, keyStr, y)
	}
	y := &moduleLoading{finished: make(chan struct{}), owner: v}
	v.loaded[keyStr] = y
	v.isImported.Unlock()

	defer func() {
		v.isImported.Lock()
		delete(v.loaded, keyStr)
		if y.err == nil && y.mod != nil {
			v.Imported[keyStr] = y.mod
		}
		v.isImported.Unlock()
		close(y.finished)
	}()
	y.mod, y.err = v.runModule(s, path, dest, keyStr, searched)
	if y.err == nil && y.mod == nil {
		y.err = messages.At(s.Line(), 0, messages.New(messages.H2482, path))
	}
	return y.mod, y.err
}

// waitForModule waits for another task to finish loading a module (called with
// v.isImported held; it releases it). If the loading task is itself waiting, directly or
// through other tasks, for a module this task is loading, waiting would never end: that is an
// import cycle across tasks and an error.
func (v *VM) waitForModule(s *ast.ImportStatement, path, keyStr string, y *moduleLoading) (*DictVal, error) {
	for key, steps := keyStr, 0; steps <= len(v.loaded); steps++ {
		l := v.loaded[key]
		if l == nil {
			break
		}
		if l.owner == v {
			v.isImported.Unlock()
			return nil, messages.At(s.Line(), 0, messages.New(messages.H2483, path))
		}
		next, waiting := v.waiting[l.owner]
		if !waiting {
			break
		}
		key = next
	}
	v.waiting[v] = keyStr
	v.isImported.Unlock()
	v.Blocking(func() { <-y.finished })
	v.isImported.Lock()
	delete(v.waiting, v)
	v.isImported.Unlock()
	return y.mod, y.err
}

// moduleTarget 'aktar' yolunun gösterdiği modülü bulur (yolu ve gösterilen adı; kaynak yalnızca
// yüklenirken okunur, bakilanlar okuma hatası içindir). "tr/" önekli yol dilin gömülü
// kitaplığındandır (11.6); kitaplık modülünün içindeki göreli yol da
// kitaplıkta çözülür. Öbür göreli yollar önce aktaran dosyanın, sonra programın klasörüne
// göre aranır (11.1).
func (v *VM) moduleTarget(s *ast.ImportStatement, path string) (dest *Source, searched []string, err error) {
	name, inLibrary := LibraryModule(path)
	if !inLibrary && v.source != nil && v.source.Embedded && !filepath.IsAbs(path) {
		name, inLibrary = pathpkg.Join(pathpkg.Dir(strings.TrimPrefix(v.source.Path, LibraryPrefix)), filepath.ToSlash(path)), true
	}
	if inLibrary {
		return &Source{Path: LibraryPrefix + name, Name: LibraryPrefix + name, Embedded: true}, nil, nil
	}
	resolved, searched, _ := ResolveModulePath(path, v.searchDirs())
	return &Source{Path: resolved, Name: shownPath(resolved)}, searched, nil
}

// runModule modülü (diskten ya da kitaplıktan) okur, denetler ve kendi kaynağının bağlamında çalıştırır.
func (v *VM) runModule(s *ast.ImportStatement, path string, dest *Source, keyStr string, searched []string) (*DictVal, error) {
	if dest.Embedded {
		payload, err := ReadLibrary(strings.TrimPrefix(dest.Path, LibraryPrefix))
		if err != nil {
			return nil, messages.At(s.Line(), 0, err)
		}
		dest.Code = string(payload)
	} else {
		data, err := os.ReadFile(dest.Path)
		if err != nil {
			return nil, messages.At(s.Line(), 0, ReadError(codes.DisplayName(codes.A070), path, messages.H2112, err, searched))
		}
		dest.Code = string(data)
	}
	// Modül genel kapsamın altında çalışır: genel kapsamda tanımlı (ya da yuvası ayrılmış)
	// adları görür; yerleşikler de oradadır.
	ms := CheckModule(dest.Code, v.GlobalEnv.scope.nameList, false)
	modSource := &Source{Path: dest.Path, Name: dest.Name, Code: ms.Text, Embedded: dest.Embedded}
	if ms.Stage != "" {
		return nil, &ModuleError{Source: modSource, Kind: ms.Stage, Errors: ms.Errors, ModulePath: s.Path, ImportLine: s.Line(), Importer: v.source}
	}
	prog := ms.Program

	prev, prevChain := v.source, v.loading
	v.source = modSource
	v.loading = append(v.loading[:len(v.loading):len(v.loading)], keyStr)
	defer func() { v.source, v.loading = prev, prevChain }()

	// Modül, genel kapsamın altında kendi kapsamında derlenir
	c := v.newCompiler()
	modScope := subScope(c.root)
	modScope.predefine(prog.Statements)
	modCode := c.statements(modScope, prog.Statements)
	modEnv := newEnv(v.GlobalEnv, modScope.info)

	if _, _, err := modCode.run(v, modEnv); err != nil {
		return nil, v.sourceError(modSource, err)
	}
	// Başı _ olan adlar modüle özeldir (11.7, Dart'ın kitaplığa özel adları gibi): dışa
	// aktarılmaz; takma adla erişim açık bir hata verir.
	modDict := NewDictVal()
	modDict.module = &moduleExports{path: s.Path, private: map[string]bool{}}
	modEnv.EachLocal(func(k string, exportedVal Value) {
		if IsModulePrivate(k) {
			modDict.module.private[k] = true
			return
		}
		modDict.Set(k, exportedVal)
	})
	return modDict, nil
}

// IsModulePrivate ad modüle özel mi (başı _, ama tek başına _ değil; 11.7)?
func IsModulePrivate(name string) bool { return len(name) > 1 && name[0] == '_' }
