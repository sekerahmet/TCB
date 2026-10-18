package vm

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/turkce"
)

type flowSignal int

const (
	signalNone flowSignal = iota
	signalReturn
	signalBreak
	signalContinue
)

// MatrixOperator Faz 2'deki Yapay Zeka Matris/Tensör motorunun VM'ye bağlanması için kancadır.
type MatrixOperator interface {
	Add(other Value) (Value, error)
	Sub(other Value) (Value, error)
	Mul(other Value) (Value, error)
	Div(other Value) (Value, error)
	MatMul(other Value) (Value, error)
	GetProperty(prop string) (Value, error)
}

// PropertyGetter nokta (.) operatörüyle özel nesne metodlarına erişim sağlar.
type PropertyGetter interface {
	GetProperty(prop string) (Value, error)
}

// HataAyiklayici, VM'nin her deyimden önce danıştığı hata ayıklayıcıdır. Uygulaması
// tr_dili/pkg/hataayiklama paketindedir; pkg/vm onu içe aktarmaz.
type HataAyiklayici interface {
	// BeforeStatement her deyim çalışmadan önce çağrılır. 0'dan büyük bir satır
	// döndürürse yürütme o satıra atlar.
	BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int
	// KesmeNoktasi koddaki kesme_noktası deyimi çalıştığında, bir sonraki deyimde
	// verilen gerekçeyle durmak için çağrılır.
	KesmeNoktasi(gerekce string)
}

// YeniHataAyiklayici, kesme_noktası deyimi çalıştığında henüz bir hata ayıklayıcı
// bağlı değilse onu oluşturur. tr_dili/pkg/motor tarafından atanır; nil ise
// kesme_noktası yok sayılır.
var YeniHataAyiklayici func(sourceFile, sourceCode string) HataAyiklayici

// vmShared tüm görevlerin (ana program, eşzamanlı bloklar, web/GUI istekleri) paylaştığı yorumlayıcı durumudur.
type vmShared struct {
	GlobalEnv *Environment
	Out       io.Writer
	// Stdin girdi() için okunacak kaynaktır (nil ise os.Stdin). VS Code hata ayıklama
	// modunda standart giriş protokole ayrıldığı için burası değiştirilir.
	Stdin      io.Reader
	wg         sync.WaitGroup
	Debugger   HataAyiklayici
	SourceFile string
	SourceCode string
	importedMu sync.Mutex
	Imported   map[string]*DictVal
	importing  map[string]bool

	// gil (Yorumlayıcı Kilidi): Aynı anda yalnızca bir görev TR kodu çalıştırır. Böylece
	// paylaşılan Liste/Sözlük/Yapı değerleri eşzamanlı yazımda bozulmaz ve 's += 1' gibi
	// işlemler bölünmez. Kilit; bekle(), bekle_hepsi(), ağ/disk/süreç beklemeleri gibi
	// engelleyici işlemlerde bırakılır (bkz. Blocking), böylece görevler yine iç içe ilerler.
	gil sync.Mutex
	// bekleyen, kilidi almak için bekleyen görev sayısıdır. Kilidi tutan görev döngü
	// turlarında ve fonksiyon çağrılarında bu sayaca bakar; bekleyen varsa kilidi devreder
	// (bkz. devret). Böylece CPU-yoğun bir döngü eşzamanlı görevleri ve web isteklerini
	// sonuna kadar bekletmez.
	bekleyen atomic.Int32

	// metinOnbellegi son indekslenen metnin karakter bilgisidir (bkz. metinKarakterleri).
	metinOnbellegi struct {
		metin *StringVal
		ascii bool
		runes []rune
	}
}

type StackFrame struct {
	FuncName string
	Line     int
}

// VM, bir yürütme görevinin görünümüdür: paylaşılan durum + göreve özel çağrı yığını.
// Her 'eşzamanlı' blok ve her web/GUI isteği kendi VM görünümüyle çalışır (bkz. NewTaskVM).
//
// Çağrı yığını kilitsizdir: yalnızca görevin kendi goroutine'i okur ve yazar (hata
// ayıklayıcının istekleri de duraklatılmış görevin goroutine'inde işlenir).
type VM struct {
	*vmShared
	CallStack []StackFrame

	holdsGIL  bool // bu görünüm şu an yorumlayıcı kilidini tutuyor mu (yalnızca kendi goroutine'i okur/yazar)
	asyncTask bool // 'eşzamanlı' bloğu içinde mi çalışıyor
}

func New() *VM {
	env := NewEnvironment(nil)
	v := &VM{
		vmShared: &vmShared{
			GlobalEnv: env,
			Out:       os.Stdout,
			Imported:  make(map[string]*DictVal),
			importing: make(map[string]bool),
		},
		CallStack: []StackFrame{},
	}
	v.registerBuiltins()
	v.registerMotors()
	v.registerTestBuiltins()
	return v
}

// NewTaskVM aynı paylaşılan durum üzerinde, kendi çağrı yığınına sahip yeni bir görev görünümü üretir.
//
// async=true ('eşzamanlı' blok): çağıran, yorumlayıcı kilidini tutan VM goroutine'idir; görev
// başlatıldığı yerin çağrı yığınını devralır (hata ayıklayıcı yığını bu sayede anlamlıdır).
//
// async=false (web/arayüz isteği): çağrı sunucu goroutine'inden, kilit ALINMADAN yapılır. Bu
// sırada ana program kendi CallStack'ine ekleme yapıyor olabilir; dilimi okumak veri yarışı
// olurdu. Yeni bir istek ana programın yığınının çocuğu da değildir; bu yüzden boş yığınla başlar
// ve v'nin hiçbir göreve özel alanına dokunulmaz.
func (v *VM) NewTaskVM(async bool) *VM {
	if !async {
		return &VM{vmShared: v.vmShared, CallStack: []StackFrame{}}
	}
	return &VM{vmShared: v.vmShared, CallStack: v.StackSnapshot(), asyncTask: true}
}

func (v *VM) acquireGIL() {
	v.bekleyen.Add(1)
	v.gil.Lock()
	v.bekleyen.Add(-1)
	v.holdsGIL = true
}

// devret, kilidi bekleyen bir görev varsa kilidi bırakıp yeniden alır. Yalnızca deyimler
// arasında (döngü turu başında, fonksiyon çağrısı girişinde) çağrılır; bir deyimin
// okuma-yazma adımları ('s += 1') bu yüzden bölünmez. Sayaç denetimi tek görevli
// programlarda yalnızca bir bellek okumasıdır.
func (v *VM) devret() {
	if v.bekleyen.Load() != 0 && v.holdsGIL {
		v.kilidiDevret()
	}
}

func (v *VM) kilidiDevret() {
	v.releaseGIL()
	// Bekleyen görev kilidi alabilsin; sync.Mutex 1 ms'den uzun bekleyen göreve kilidi
	// doğrudan devreder (açlık kipi), Gosched ise uyanan göreve hemen sıra verir.
	runtime.Gosched()
	v.acquireGIL()
}

func (v *VM) releaseGIL() {
	v.holdsGIL = false
	v.gil.Unlock()
}

// Blocking fn'i yorumlayıcı kilidini bırakarak çalıştırır; uyku, ağ isteği, dış süreç gibi
// uzun süren işlemler sırasında diğer görevlerin (ve web isteklerinin) ilerlemesini sağlar.
func (v *VM) Blocking(fn func()) {
	if !v.holdsGIL {
		fn()
		return
	}
	v.releaseGIL()
	defer v.acquireGIL()
	fn()
}

// RunTask fn'i bu görünüm üzerinde yorumlayıcı kilidini alarak çalıştırır ve olası bir
// paniği (programı çökertmek yerine) hataya dönüştürür. Web/GUI istek işleyicileri kullanır.
func (v *VM) RunTask(fn func() (Value, error)) (res Value, err error) {
	v.acquireGIL()
	defer v.releaseGIL()
	yiginBoyu := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			v.yiginiKirp(yiginBoyu)
			res, err = nil, fmt.Errorf("İç hata (çökme önlendi): %v", r)
		}
	}()
	return fn()
}

// yiginiKirp çağrı yığınını n çerçeveye indirir. Derlenmiş çağrılar çerçevelerini Go
// 'defer' olmadan kaldırır; bir çöküş (panic) yakalandığında yarıda kalan çerçeveler
// burada atılır.
func (v *VM) yiginiKirp(n int) {
	if len(v.CallStack) > n {
		v.CallStack = v.CallStack[:n]
	}
}

func (v *VM) StackDepth() int {
	return len(v.CallStack)
}

func (v *VM) StackSnapshot() []StackFrame {
	snap := make([]StackFrame, len(v.CallStack))
	copy(snap, v.CallStack)
	return snap
}

func (v *VM) RegisterBuiltin(name string, asciiAlias string, fn BuiltinFunc) {
	b := &BuiltinVal{Name: name, Fn: fn}
	v.GlobalEnv.Define(name, b, true)
	if asciiAlias != "" && asciiAlias != name {
		v.GlobalEnv.Define(asciiAlias, b, true)
	}
}

func (v *VM) Run(prog *ast.Program) (result Value, err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	// Yerleşik fonksiyonlardaki beklenmedik paniklerin tüm süreci çökertmesini engelle
	yiginBoyu := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			v.yiginiKirp(yiginBoyu)
			result, err = nil, fmt.Errorf("İç hata (çökme önlendi): %v", r)
		}
	}()
	defer v.GlobalEnv.RunDefers()
	val, _, err := v.programCalistir(prog.Statements)
	if err != nil {
		return nil, err
	}
	// En üst düzey 'ertele' blokları, başlatılmış eşzamanlı görevler bittikten SONRA çalışmalı
	v.Blocking(v.wg.Wait)
	return val, nil
}

// WaitAsync başlatılmış tüm 'eşzamanlı' görevlerin bitmesini bekler.
func (v *VM) WaitAsync() {
	v.Blocking(v.wg.Wait)
}

// reportAsyncError eşzamanlı görevde oluşan hatayı sessizce yutmak yerine standart hataya yazar.
// suggestName kapsamdaki adlar arasında yazıma en yakın olanı "Bunu mu demek istediniz?" olarak önerir.
func suggestName(name string, env *Environment) string {
	if o := turkce.EnYakin(name, env.Names()); o != "" {
		return fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return ""
}

func (v *VM) reportAsyncError(line int, err error) {
	fmt.Fprintf(os.Stderr, "⚠️  Eşzamanlı görev hatası (%s başlatılan): %v\n", "satır "+turkce.SayiEk(int64(line), turkce.Bulunma), err)
}
