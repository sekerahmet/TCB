package vm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"tr_dili/internal/dil/ast"
	"tr_dili/internal/dil/checker"
	"tr_dili/internal/dil/lexer"
	"tr_dili/internal/dil/parser"
	"tr_dili/internal/dil/turkce"
)

// Kaynak bir kaynak dosyadır: ana program ya da 'aktar' ile alınan bir modül. Çalışan kodun
// kaynağı göreve (VM görünümüne) ve çağrı çerçevesine aittir: bir modülde tanımlanan
// fonksiyon nereden çağrılırsa çağrılsın kendi dosyasında çalışır (göreli yollar onun
// klasörüne göre de aranır, hataları onun adı ve satırlarıyla yazılır).
type Kaynak struct {
	Yol string // dosyanın çözülmüş yolu
	Ad  string // hata mesajlarında gösterilen yol (mümkünse çalışma klasörüne göre)
	Kod string
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
func (v *VM) anaKaynak() *Kaynak {
	return &Kaynak{Yol: v.SourceFile, Ad: gosterilenYol(v.SourceFile), Kod: v.SourceCode}
}

// ModulHatasi bir dosyanın (modülün ya da, başka bir dosyadan çağrılan kodunun) hatasıdır.
// Mesajlar o dosyanın satırlarına göredir ve dosyanın adıyla başlar; kod çerçevesi o
// dosyanın kaynağından çizilir (bkz. HataMaddeleri).
//
// İki biçimi vardır: çalışma hatası (Err) ve modülün yüklenmeden önceki denetim hataları
// (Tur, Hatalar): sözdizimi ya da tip denetimi hatası olan modülün hiçbir satırı çalışmaz.
type ModulHatasi struct {
	Kaynak *Kaynak
	Err    error

	Tur         string   // "sözdizimi" ya da "tip denetimi" (denetim hatalarında)
	Hatalar     []string // denetim hataları, modülün satırlarıyla
	ModulYolu   string   // 'aktar'da yazılan yol
	AktarSatiri int      // 'aktar' deyiminin satırı
	Aktaran     *Kaynak  // 'aktar' deyiminin bulunduğu dosya (nil: ana program)
}

func (e *ModulHatasi) Error() string {
	if len(e.Hatalar) == 0 {
		return e.Kaynak.Ad + ", " + e.Err.Error()
	}
	parcalar := make([]string, len(e.Hatalar))
	for i, h := range e.Hatalar {
		parcalar[i] = e.Kaynak.Ad + ", " + h
	}
	return e.baslik() + ": " + strings.Join(parcalar, "; ")
}

func (e *ModulHatasi) Unwrap() error { return e.Err }

// baslik denetim hatasının 'aktar' satırını gösteren ilk bölümüdür.
func (e *ModulHatasi) baslik() string {
	onek := ""
	if e.Aktaran != nil {
		onek = e.Aktaran.Ad + ", "
	}
	return fmt.Sprintf("%sSatır %d: %q modülü %s hataları nedeniyle yüklenemedi", onek, e.AktarSatiri, e.ModulYolu, e.Tur)
}

// HataMaddesi kullanıcıya gösterilecek bir hata satırı ve ait olduğu dosyadan çizilmiş
// kod çerçevesidir (çerçeve konum bilinmiyorsa boştur).
type HataMaddesi struct {
	Mesaj   string
	Cerceve string
}

// HataMaddeleri bir çalışma hatasını kullanıcıya gösterilecek maddelere ayırır. anaKod
// ana programın kaynağıdır; bir modüle ait hatanın çerçevesi modülün kaynağından çizilir.
// Modülün denetim hataları her biri ayrı madde olarak, 'aktar' satırının ardından gelir.
func HataMaddeleri(err error, anaKod string) []HataMaddesi {
	mesaj := err.Error()
	var mh *ModulHatasi
	if !errors.As(err, &mh) {
		return []HataMaddesi{{mesaj, turkce.KodCercevesi(anaKod, mesaj)}}
	}
	if len(mh.Hatalar) == 0 {
		return []HataMaddesi{{mesaj, turkce.KodCercevesi(mh.Kaynak.Kod, mh.Err.Error())}}
	}
	aktaranKod := anaKod
	if mh.Aktaran != nil {
		aktaranKod = mh.Aktaran.Kod
	}
	b := mh.baslik()
	maddeler := []HataMaddesi{{b, turkce.KodCercevesi(aktaranKod, b)}}
	for _, h := range mh.Hatalar {
		maddeler = append(maddeler, HataMaddesi{mh.Kaynak.Ad + ", " + h, turkce.KodCercevesi(mh.Kaynak.Kod, h)})
	}
	return maddeler
}

// kaynakHatasi k dosyasında çalışan koddan çıkan konumlu bir hatayı o dosyaya bağlar
// (k nil ise ana program). Zaten bir dosyaya bağlı hata ve konumsuz hatalar ('fırlat' ile
// atılan değerler: yakalayan onları olduğu gibi görür) değişmez.
func (v *VM) kaynakHatasi(k *Kaynak, err error) error {
	var mh *ModulHatasi
	if errors.As(err, &mh) || !satirliMi(err) {
		return err
	}
	if k == nil {
		k = v.anaKaynak()
	}
	return &ModulHatasi{Kaynak: k, Err: err}
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

// ModulDenetle bir modülün kaynağını çalıştırmadan önce denetler: modül olarak ayrıştırır
// (üst düzeyde 'sonuç' hatadır) ve tip denetiminden geçirir. Hata varsa tur
// ("sözdizimi" ya da "tip denetimi") ve hatalar döner; 'aktar', 'tr kontrol' ve
// 'tr derle' aynı denetimi kullanır.
func ModulDenetle(kod string) (prog *ast.Program, tur string, hatalar []string) {
	p := parser.New(lexer.New(kod))
	prog = p.ParseModule()
	if errs := p.Errors(); len(errs) > 0 {
		return prog, "sözdizimi", errs
	}
	if errs := checker.New().CheckProgram(prog); len(errs) > 0 {
		return prog, "tip denetimi", errs
	}
	return prog, "", nil
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
	// Modüller de dosyalar gibi aranır: önce çalışma klasörü, sonra kodun (ve programın) klasörü.
	resolvedPath, bakilanlar := v.YolCoz(path)
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		absPath = resolvedPath
	}

	v.importedMu.Lock()
	if mod, ok := v.Imported[absPath]; ok {
		v.importedMu.Unlock()
		return mod, nil
	}
	if y := v.yuklenen[absPath]; y != nil {
		v.importedMu.Unlock()
		if slices.Contains(v.yukleme, absPath) {
			return nil, fmt.Errorf("Satır %d: %q modülünde döngüsel aktarım tespit edildi (modül, yüklenirken kendini dolaylı olarak yeniden aktarıyor)", s.Line(), path)
		}
		v.Blocking(func() { <-y.bitti })
		return y.mod, y.err
	}
	y := &modulYukleme{bitti: make(chan struct{})}
	v.yuklenen[absPath] = y
	v.importedMu.Unlock()

	defer func() {
		v.importedMu.Lock()
		delete(v.yuklenen, absPath)
		if y.err == nil && y.mod != nil {
			v.Imported[absPath] = y.mod
		}
		v.importedMu.Unlock()
		close(y.bitti)
	}()
	y.mod, y.err = v.modulCalistir(s, path, resolvedPath, absPath, bakilanlar)
	if y.err == nil && y.mod == nil {
		y.err = fmt.Errorf("Satır %d: %q modülü yüklenemedi", s.Line(), path)
	}
	return y.mod, y.err
}

// modulCalistir modülü okur, denetler ve kendi kaynağının bağlamında çalıştırır.
func (v *VM) modulCalistir(s *ast.ImportStatement, path, resolvedPath, absPath string, bakilanlar []string) (*DictVal, error) {
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("Satır %d: %w", s.Line(), OkumaHatasi("aktar", path, "modülü okunamadı", err, bakilanlar))
	}
	code := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	modKaynak := &Kaynak{Yol: resolvedPath, Ad: gosterilenYol(resolvedPath), Kod: code}
	prog, tur, hatalar := ModulDenetle(code)
	if len(hatalar) > 0 {
		return nil, &ModulHatasi{Kaynak: modKaynak, Tur: tur, Hatalar: hatalar, ModulYolu: s.Path, AktarSatiri: s.Line(), Aktaran: v.kaynak}
	}

	onceki, oncekiZincir := v.kaynak, v.yukleme
	v.kaynak = modKaynak
	v.yukleme = append(v.yukleme[:len(v.yukleme):len(v.yukleme)], absPath)
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
