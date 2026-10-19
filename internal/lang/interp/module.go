package interp

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
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

// gosterilenYol bir dosya yolunu hata mesajında gösterilecek biçime çevirir: çalışma
// klasörünün altındaysa ona göre, değilse mutlak; ayırıcı '/'.
func gosterilenYol(yol string) string {
	if yol == "" || strings.HasPrefix(yol, "<") {
		return yol
	}
	mutlak, err := filepath.Abs(yol)
	if err != nil {
		return filepath.ToSlash(yol)
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, mutlak); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(mutlak)
}

// anaKaynak ana programın kaynağıdır (SourceFile ve SourceCode'dan).
func (v *VM) anaKaynak() *Source {
	return &Source{Path: v.SourceFile, Name: gosterilenYol(v.SourceFile), Code: v.SourceCode}
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

	Kind       string   // "sözdizimi" ya da "tip denetimi" (denetim hatalarında)
	Errors     []string // denetim hataları, modülün satırlarıyla
	ModulePath string   // 'aktar'da yazılan yol
	ImportLine int      // 'aktar' deyiminin satırı
	Importer   *Source  // 'aktar' deyiminin bulunduğu dosya (nil: ana program)
}

func (e *ModuleError) Error() string {
	if len(e.Errors) == 0 {
		return e.Source.Name + ", " + e.Err.Error()
	}
	parcalar := make([]string, len(e.Errors))
	for i, h := range e.Errors {
		parcalar[i] = e.Source.Name + ", " + h
	}
	return e.baslik() + ": " + strings.Join(parcalar, "; ")
}

func (e *ModuleError) Unwrap() error { return e.Err }

// baslik denetim hatasının 'aktar' satırını gösteren ilk bölümüdür.
func (e *ModuleError) baslik() string {
	onek := ""
	if e.Importer != nil {
		onek = e.Importer.Name + ", "
	}
	return fmt.Sprintf("%sSatır %d: %q modülü %s hataları nedeniyle yüklenemedi", onek, e.ImportLine, e.ModulePath, e.Kind)
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
func ErrorItems(err error, anaKod string) []ErrorItem {
	mesaj := err.Error()
	var mh *ModuleError
	if !errors.As(err, &mh) {
		return []ErrorItem{{mesaj, turkish.CodeFrame(anaKod, mesaj)}}
	}
	if len(mh.Errors) == 0 {
		return []ErrorItem{{mesaj, turkish.CodeFrame(mh.Source.Code, mh.Err.Error())}}
	}
	aktaranKod := anaKod
	if mh.Importer != nil {
		aktaranKod = mh.Importer.Code
	}
	b := mh.baslik()
	maddeler := []ErrorItem{{b, turkish.CodeFrame(aktaranKod, b)}}
	for _, h := range mh.Errors {
		maddeler = append(maddeler, ErrorItem{mh.Source.Name + ", " + h, turkish.CodeFrame(mh.Source.Code, h)})
	}
	return maddeler
}

// kaynakHatasi k dosyasında çalışan koddan çıkan konumlu bir hatayı o dosyaya bağlar
// (k nil ise ana program). Zaten bir dosyaya bağlı hata ve konumsuz hatalar ('fırlat' ile
// atılan değerler: yakalayan onları olduğu gibi görür) değişmez.
func (v *VM) kaynakHatasi(k *Source, err error) error {
	var mh *ModuleError
	if errors.As(err, &mh) || !satirliMi(err) {
		return err
	}
	if k == nil {
		k = v.anaKaynak()
	}
	return &ModuleError{Source: k, Err: err}
}

// kaynaktaCagir başka bir dosyada tanımlanmış fonksiyonu kendi dosyasının bağlamında
// çağırır; hatası o dosyaya bağlanır. Aynı dosyadaki çağrılar buraya uğramaz.
func (v *VM) kaynaktaCagir(fn *FuncVal, fk *fonkKodu, fnEnv *Environment, kendi Value, args []Value, nargs int, named map[string]Value, line int) (Value, error) {
	onceki := v.kaynak
	v.kaynak = fk.kaynak
	defer func() { v.kaynak = onceki }()
	val, err := v.cagirHazir(fn, fk, fnEnv, kendi, args, nargs, named, line)
	if err != nil {
		return nil, v.kaynakHatasi(fk.kaynak, err)
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
func CheckModule(metin string, genelAdlar []string, dinamik bool) *source.Result {
	return source.Prepare(metin, source.Options{Module: true, TypeCheck: true,
		InGlobalScope: true, GlobalNames: genelAdlar, GlobalDynamic: dinamik})
}

// modulYukleme yüklenmekte olan bir modüldür; aynı modülü isteyen başka görevler yüklemenin
// bitmesini bekler.
type modulYukleme struct {
	bitti chan struct{}
	mod   *DictVal
	err   error
}

// aktar 'aktar' deyimini çalıştırır. Modül, genel kapsamın altında kendi ortamında çalışır
// ve ortamının tüm adları dışa aktarılır. takma, 'aktar ... olarak ad' biçiminde takma adı
// çağıranın kapsamına tanımlar; takma adsız aktarımda adlar çağıranın ortamına adla
// tanımlanır.
func (v *VM) aktar(s *ast.ImportStatement, env *Environment, takma func(*Environment, Value, bool)) (Value, flowSignal, error) {
	path := s.Path
	if !strings.HasSuffix(path, ".tr") {
		path += ".tr"
	}
	mod, err := v.modulYukle(s, path)
	if err != nil {
		return nil, signalNone, err
	}
	if takma != nil {
		takma(env, mod, true)
		return mod, signalNone, nil
	}
	for _, k := range mod.Order {
		env.Define(k, mod.Pairs[k], false)
	}
	return mod, signalNone, nil
}

// modulYukle modülü (gerekirse) yükler. Bir modül program boyunca bir kez çalışır.
// Başka bir görev aynı modülü yüklüyorsa bitmesi beklenir; yükleyen görevin kendi aktarım
// zinciri modülü içeriyorsa bu döngüsel bir aktarımdır.
func (v *VM) modulYukle(s *ast.ImportStatement, path string) (*DictVal, error) {
	if err := CheckPath(codes.Name(codes.A070), path); err != nil {
		return nil, fmt.Errorf("Satır %d: %w", s.Line(), err)
	}
	hedef, bakilanlar, err := v.modulHedefi(s, path)
	if err != nil {
		return nil, err
	}
	anahtar := hedef.Path
	if !hedef.Embedded {
		if mutlak, err := filepath.Abs(hedef.Path); err == nil {
			anahtar = mutlak
		}
	}

	v.importedMu.Lock()
	if mod, ok := v.Imported[anahtar]; ok {
		v.importedMu.Unlock()
		return mod, nil
	}
	if y := v.yuklenen[anahtar]; y != nil {
		v.importedMu.Unlock()
		if slices.Contains(v.yukleme, anahtar) {
			return nil, fmt.Errorf("Satır %d: %q modülünde döngüsel aktarım tespit edildi (modül, yüklenirken kendini dolaylı olarak yeniden aktarıyor)", s.Line(), path)
		}
		v.Blocking(func() { <-y.bitti })
		return y.mod, y.err
	}
	y := &modulYukleme{bitti: make(chan struct{})}
	v.yuklenen[anahtar] = y
	v.importedMu.Unlock()

	defer func() {
		v.importedMu.Lock()
		delete(v.yuklenen, anahtar)
		if y.err == nil && y.mod != nil {
			v.Imported[anahtar] = y.mod
		}
		v.importedMu.Unlock()
		close(y.bitti)
	}()
	y.mod, y.err = v.modulCalistir(s, path, hedef, anahtar, bakilanlar)
	if y.err == nil && y.mod == nil {
		y.err = fmt.Errorf("Satır %d: %q modülü yüklenemedi", s.Line(), path)
	}
	return y.mod, y.err
}

// modulHedefi 'aktar' yolunun gösterdiği modülü bulur (yolu ve gösterilen adı; kaynak yalnızca
// yüklenirken okunur, bakilanlar okuma hatası içindir). "tr/" önekli yol dilin gömülü
// kitaplığındandır (11.6); kitaplık modülünün içindeki göreli yol da
// kitaplıkta çözülür. Öbür göreli yollar önce aktaran dosyanın, sonra programın klasörüne
// göre aranır (11.1).
func (v *VM) modulHedefi(s *ast.ImportStatement, path string) (hedef *Source, bakilanlar []string, err error) {
	ad, kitaplikta := LibraryModule(path)
	if !kitaplikta && v.kaynak != nil && v.kaynak.Embedded && !filepath.IsAbs(path) {
		ad, kitaplikta = pathpkg.Join(pathpkg.Dir(strings.TrimPrefix(v.kaynak.Path, LibraryPrefix)), filepath.ToSlash(path)), true
	}
	if kitaplikta {
		return &Source{Path: LibraryPrefix + ad, Name: LibraryPrefix + ad, Embedded: true}, nil, nil
	}
	cozulen, bakilanlar, _ := ResolveModulePath(path, v.aramaKlasorleri())
	return &Source{Path: cozulen, Name: gosterilenYol(cozulen)}, bakilanlar, nil
}

// modulCalistir modülü (diskten ya da kitaplıktan) okur, denetler ve kendi kaynağının bağlamında çalıştırır.
func (v *VM) modulCalistir(s *ast.ImportStatement, path string, hedef *Source, anahtar string, bakilanlar []string) (*DictVal, error) {
	if hedef.Embedded {
		veri, err := ReadLibrary(strings.TrimPrefix(hedef.Path, LibraryPrefix))
		if err != nil {
			return nil, fmt.Errorf("Satır %d: %w", s.Line(), err)
		}
		hedef.Code = string(veri)
	} else {
		data, err := os.ReadFile(hedef.Path)
		if err != nil {
			return nil, fmt.Errorf("Satır %d: %w", s.Line(), ReadError(codes.Name(codes.A070), path, "modülü okunamadı", err, bakilanlar))
		}
		hedef.Code = string(data)
	}
	// Modül genel kapsamın altında çalışır: genel kapsamda tanımlı (ya da yuvası ayrılmış)
	// adları görür; yerleşikler de oradadır.
	ms := CheckModule(hedef.Code, v.GlobalEnv.kapsam.adlar, false)
	modKaynak := &Source{Path: hedef.Path, Name: hedef.Name, Code: ms.Text, Embedded: hedef.Embedded}
	if ms.Stage != "" {
		return nil, &ModuleError{Source: modKaynak, Kind: ms.Stage, Errors: ms.Errors, ModulePath: s.Path, ImportLine: s.Line(), Importer: v.kaynak}
	}
	prog := ms.Program

	onceki, oncekiZincir := v.kaynak, v.yukleme
	v.kaynak = modKaynak
	v.yukleme = append(v.yukleme[:len(v.yukleme):len(v.yukleme)], anahtar)
	defer func() { v.kaynak, v.yukleme = onceki, oncekiZincir }()

	// Modül, genel kapsamın altında kendi kapsamında derlenir
	c := v.yeniDerleyici()
	modKapsam := altKapsam(c.kok)
	modKapsam.onTanimla(prog.Statements)
	modKod := c.deyimler(modKapsam, prog.Statements)
	modEnv := yeniOrtam(v.GlobalEnv, modKapsam.bilgi)

	if _, _, err := modKod.calistir(v, modEnv); err != nil {
		return nil, v.kaynakHatasi(modKaynak, err)
	}
	modDict := NewDictVal()
	modEnv.EachLocal(func(k string, exportedVal Value) {
		modDict.Set(k, exportedVal)
	})
	return modDict, nil
}
