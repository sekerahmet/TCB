// Package interp is TR's compiler and interpreter. Before running, the syntax tree is compiled
// once into Go closures (compiler*.go); variables are resolved to slots at compile time. The
// package also holds values, the core builtins and methods, modules, concurrent tasks and the
// interpreter lock. Engines (tensor, data, tokenizer) and the debugger import this package and
// attach themselves through hooks (AddEngine, NewDebugger); the interpreter does not know them.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package interp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

type flowSignal int

const (
	signalNone flowSignal = iota
	signalReturn
	signalBreak
	signalContinue
)

// PropertyGetter gives access to an object's members with the dot (.) operator. kod is the
// name's member code in the dictionary (internal/lang/codes, Y; codes.None when the name is not
// a member name); the compiler resolves it once at compile time and implementations branch on
// codes rather than names. ad is the spelling the user wrote (for error messages and
// suggestions).
type PropertyGetter interface {
	GetProperty(kd codes.Code, name string) (Value, error)
}

// TaskMemberHook is the form of PropertyGetter that also receives the task reading the member
// (the VM view). Properties whose result depends on task-specific state (e.g. Yol's var_mı: a
// relative path resolves against the directories of the running module and the program)
// implement it; for such values dot access calls it instead of GetProperty (GetProperty stays
// for task-less reads on the suggestion and error paths).
type TaskMemberHook interface {
	TaskMember(v *VM, kd codes.Code, name string) (Value, error)
}

// Debugger is the debugger the VM consults before every statement. Its implementation lives in
// github.com/sekerahmet/TCB/internal/devtools/debugger; internal/lang/interp does not import it.
type Debugger interface {
	// BeforeStatement is called before each statement runs. If it returns a line
	// greater than 0, execution jumps to that line.
	BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int
	// Breakpoint is called when a kesme_noktası statement runs, to stop at the next
	// statement with the given reason.
	Breakpoint(reason string)
}

// NewDebugger creates a debugger when a kesme_noktası statement runs and none is attached
// yet. It is set by github.com/sekerahmet/TCB/internal/stdlib; when nil, kesme_noktası is
// ignored.
var NewDebugger func(sourceFile, sourceCode string) Debugger

// vmShared tüm görevlerin (ana program ve eşzamanlı bloklar) paylaştığı yorumlayıcı durumudur.
type vmShared struct {
	// Debugger her deyimde okunur; sık okunan alanlar başta tutulur.
	Debugger  Debugger
	GlobalEnv *Environment
	Out       io.Writer
	// Stdin girdi() için okunacak kaynaktır (nil ise os.Stdin). Düzenleyicinin hata ayıklama
	// modunda standart giriş protokole ayrıldığı için burası değiştirilir. Okuma VM'in tek
	// tamponlu okuyucusuyla yapılır (bkz. GirdiOkuyucu).
	Stdin      io.Reader
	wg         sync.WaitGroup
	SourceFile string
	SourceCode string
	// CommandName programı başlatan komuttur (ör. "tr egit.tr", derlenmiş uygulamada
	// "uygulama.exe"); komut satırı seçeneklerinin yardımında ve hatalarında gösterilir.
	// Boşsa "tr <SourceFile>" kullanılır.
	CommandName string
	isImported  sync.Mutex
	Imported    map[string]*DictVal
	// loaded yüklenmekte olan modüllerdir (mutlak yol → yükleme); başka bir görev aynı
	// modülü isterse yüklemenin bitmesini bekler.
	loaded map[string]*moduleLoading
	// waiting is the module each task (VM view) is waiting for another task to load; with
	// moduleLoading.owner it detects import cycles across tasks.
	waiting map[*VM]string
	// Err programı durdurmayan hataların (eşzamanlı görev hataları) yazıldığı yerdir;
	// varsayılanı standart hatadır. Düzenleyici oturumu ve tr test kendi çıktılarına bağlar.
	Err         io.Writer
	isInput     sync.Mutex
	inputReader *bufio.Reader
	// builtinConstants genel kapsama kaydedilen yerleşik sabitlerdir (ad → değer; ör. pi);
	// hata ayıklayıcı onları kullanıcı değişkenlerinden ayırır (YerlesikSabitMi).
	builtinConstants map[string]Value

	// gil (Yorumlayıcı Kilidi): Aynı anda yalnızca bir görev TR kodu çalıştırır. Böylece
	// paylaşılan Liste/Sözlük/Yapı değerleri eşzamanlı yazımda bozulmaz ve 's += 1' gibi
	// işlemler bölünmez. Kilit; bekle(), bekle_hepsi(), ağ/disk/süreç beklemeleri gibi
	// engelleyici işlemlerde bırakılır (bkz. Blocking), böylece görevler yine iç içe ilerler.
	gil sync.Mutex
	// pending, kilidi almak için bekleyen görev sayısıdır. Kilidi tutan görev döngü
	// turlarında ve fonksiyon çağrılarında bu sayaca bakar; bekleyen varsa kilidi devreder
	// (bkz. devret). Böylece CPU-yoğun bir döngü eşzamanlı görevleri sonuna kadar
	// bekletmez.
	pending atomic.Int32
	// taskHooks, bir görev kilidi her aldığında çağrılan motor kancalarıdır (bkz.
	// GorevKancasiEkle). Yalnızca kayıt sırasında (program başlamadan) yazılır.
	taskHooks []func(*VM)

	// arguments betiğe komut satırında verilen argümanlardır; argumanlarOkundu program
	// onları okuduysa açıktır (bkz. program_args.go).
	arguments []string
	argsRead  atomic.Bool

	// interrupted programın nazikçe durdurulması istendiyse açıktır; kesilmeKanali o anda
	// kapanır (bkz. interrupt.go).
	interrupted   atomic.Bool
	interruptChan chan struct{}
	// interruptDeferrers kesme isteğini adım sınırına erteleyen kontrol noktası sayısıdır;
	// durdurmaIstendi ertelenen bir istek geldiyse açıktır (bkz. KesmeIste).
	interruptDeferrers atomic.Int32
	stopRequested      atomic.Bool

	// isOutput çıktı noktalarını (CiktiYaz) etkin canlı satırla sıralar; canliVar canlı satır
	// varken açıktır (yoksa kilit alınmaz). Bkz. output.go.
	isOutput sync.Mutex
	live     LiveLine
	hasLive  atomic.Bool

	// gizliler gizli değer süzgecine kayıtlı değerlerdir; maske onlardan kurulmuş
	// değiştiricidir (gizli değer yoksa nil: çıktı noktalarında maliyet bir atomik okuma).
	isSecret sync.Mutex
	secrets  []string
	mask     atomic.Pointer[strings.Replacer]

	// stringCache son indekslenen metnin karakter bilgisidir (bkz. metinKarakterleri).
	stringCache struct {
		text  *StringVal
		ascii bool
		runes []rune
	}
}

type StackFrame struct {
	FuncName string
	Line     int // çağrının satırı (çağıran dosyada)
	// Source fonksiyon gövdesinin çalıştığı dosyadır; nil ana programdır. Çerçevenin satırı
	// çağıran dosyadadır: bir önceki çerçevenin Source'u (ilk çerçevede ana program).
	Source *Source
}

// VM is the view of one execution task: shared state plus a task-private call stack. Each
// 'eşzamanlı' block runs with its own VM view (see NewTaskVM).
//
// The call stack has no lock: only the task's own goroutine reads and writes it (debugger
// requests are also handled on the paused task's goroutine).
type VM struct {
	*vmShared
	CallStack []StackFrame

	holdsGIL  bool // bu görünüm şu an yorumlayıcı kilidini tutuyor mu (yalnızca kendi goroutine'i okur/yazar)
	asyncTask bool // 'eşzamanlı' bloğu içinde mi çalışıyor

	// source şu an çalışan kodun dosyasıdır (nil: ana program; bkz. Kaynak). Paylaşılan
	// durumda değil görevde tutulur: bir modül yüklenirken kilidi bırakınca başka bir görev
	// kendi dosyasının bağlamında kalır. Başka dosyada tanımlı bir fonksiyon çağrılınca o
	// fonksiyonun dosyasına geçilir (kaynaktaCagir).
	source *Source
	// loading bu görevin yüklemekte olduğu modüllerin (mutlak yol) zinciridir; döngüsel
	// aktarımı, başka bir görevin yüklemesini beklemekten ayırır.
	loading []string
	// activeBlock hata ayıklayıcıya danışılan son deyimin bloğudur (bkz. ZiplamaHedefi).
	activeBlock *blockCode

	// taskValues motorların göreve özel durumudur (ör. tensör motorunun
	// karışık_hassasiyet bloğu); bkz. GorevDegeri. Çağrı yığını gibi yalnızca görevin
	// kendi goroutine'i okur ve yazar; yeni görevler (NewTaskVM) boş başlar.
	taskValues map[any]any

	// interruptThrown bu görevin kesilme paniğini attığını söyler: yığın çözülürken çalışan
	// 'ertele' blokları yeniden kesilmez (bkz. kesilmeyiDenetle).
	interruptThrown bool
}

// TaskValue returns the engine state bound to this task (VM view) under a key, or nil. The
// value is not visible to other tasks (concurrent blocks); when the task releases the GIL
// (Blocking) another task reads its own value. The key must be of an unexported type of the
// engine (to avoid collisions).
func (v *VM) TaskValue(keyStr any) any { return v.taskValues[keyStr] }

// SetTaskValue sets the engine state bound to this task; a nil deger removes it.
func (v *VM) SetTaskValue(keyStr, value any) {
	if value == nil {
		delete(v.taskValues, keyStr)
		return
	}
	if v.taskValues == nil {
		v.taskValues = map[any]any{}
	}
	v.taskValues[keyStr] = value
}

func New() *VM {
	env := NewEnvironment(nil)
	v := &VM{
		vmShared: &vmShared{
			GlobalEnv: env,
			Out:       os.Stdout,
			Err:       os.Stderr,
			Imported:  make(map[string]*DictVal),
			loaded:    make(map[string]*moduleLoading),
			waiting:   make(map[*VM]string),

			interruptChan: make(chan struct{}),
		},
		CallStack: []StackFrame{},
	}
	v.registerTypes()
	v.registerBuiltins()
	v.registerStats()
	v.registerMath()
	v.registerEngines()
	v.registerTestBuiltins()
	return v
}

// NewTaskVM creates a new task view for an 'eşzamanlı' task over the same shared state, with
// its own call stack. The caller is the VM goroutine holding the interpreter lock; the task
// inherits the call stack of the place it was started from (which keeps the debugger's stack
// meaningful).
func (v *VM) NewTaskVM() *VM {
	// Görev başlatıldığı yerin dosyasında ve aktarım zincirinde başlar (modül yüklenirken
	// başlatılan görev o modülü aktarırsa bu döngüdür, beklemek kilitlenirdi).
	return &VM{vmShared: v.vmShared, CallStack: v.StackSnapshot(), asyncTask: true,
		source: v.source, loading: slices.Clip(v.loading)}
}

func (v *VM) acquireGIL() {
	v.pending.Add(1)
	v.gil.Lock()
	v.pending.Add(-1)
	v.holdsGIL = true
	for _, f := range v.taskHooks {
		f(v)
	}
}

// handOff, kilidi bekleyen bir görev varsa kilidi bırakıp yeniden alır. Yalnızca deyimler
// arasında (döngü turu başında, fonksiyon çağrısı girişinde) çağrılır; bir deyimin
// okuma-yazma adımları ('s += 1') bu yüzden bölünmez. Sayaç denetimi tek görevli
// programlarda yalnızca bir bellek okumasıdır.
func (v *VM) handOff() {
	if v.pending.Load() != 0 && v.holdsGIL {
		v.handOffLock()
	}
}

func (v *VM) handOffLock() {
	v.releaseGIL()
	// Bekleyen görev kilidi alabilsin; sync.Mutex 1 ms'den uzun bekleyen göreve kilidi
	// doğrudan devreder (açlık kipi), Gosched ise uyanan göreve hemen sıra verir.
	runtime.Gosched()
	v.acquireGIL()
	v.checkInterrupt()
}

func (v *VM) releaseGIL() {
	v.holdsGIL = false
	v.gil.Unlock()
}

// Blocking runs fn with the interpreter lock released, so other tasks make progress during
// long operations such as sleeping, network requests or external processes.
func (v *VM) Blocking(fn func()) {
	if !v.holdsGIL {
		fn()
		return
	}
	v.releaseGIL()
	defer func() {
		v.acquireGIL()
		// Bekleme sırasında kesme istendiyse (bekle, veri okuma ...) program burada durur.
		v.checkInterrupt()
	}()
	fn()
}

// trimStack çağrı yığınını n çerçeveye indirir. Derlenmiş çağrılar çerçevelerini Go
// 'defer' olmadan kaldırır; bir çöküş (panic) yakalandığında yarıda kalan çerçeveler
// burada atılır.
func (v *VM) trimStack(n int) {
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

func (v *VM) Run(prog *ast.Program) (result Value, err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	// Yerleşik fonksiyonlardaki beklenmedik paniklerin tüm süreci çökertmesini engelle
	stackSize := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			v.trimStack(stackSize)
			if IsInterrupt(r) {
				// Kesilme: ertelenen işler çalıştı; onların hatası varsa kesilmeye eklenir.
				result, err = nil, addDeferError(ErrInterrupted, err)
				return
			}
			result, err = nil, panicError(r)
		}
	}()
	defer func() {
		// Programın üst düzey ertelenen işleri; hataları programın hatası olur (9.4)
		if derr := v.GlobalEnv.RunDefers(); derr != nil {
			result, err = nil, addDeferError(err, derr)
		}
	}()
	val, _, err := v.runProgram(prog.Statements)
	if err != nil {
		return nil, err
	}
	// En üst düzey 'ertele' blokları, başlatılmış eşzamanlı görevler bittikten SONRA çalışmalı
	v.Blocking(v.wg.Wait)
	return val, nil
}

// WaitAsync waits for every started 'eşzamanlı' task to finish.
func (v *VM) WaitAsync() {
	v.Blocking(v.wg.Wait)
}

// reportAsyncError eşzamanlı görevde oluşan hatayı sessizce yutmak yerine standart hataya yazar.
// suggestName kapsamdaki adlar arasında yazıma en yakın olanı "Bunu mu demek istediniz?" olarak önerir.
// Öneri denetleyiciyle ortaktır (ipucu.AdOnerisi): tanımsız ad çalışmadan önce de aynı
// mesajla bulunur.
func suggestName(name string, env *Environment) string {
	return hint.NameSuggestion(name, env.Names())
}

func (v *VM) reportAsyncError(line int, err error) {
	w := v.Err
	if w == nil {
		w = os.Stderr
	}
	v.WriteOutput(w, fmt.Sprintf("⚠️  Eşzamanlı görev hatası (%s başlatılan): %v\n", "satır "+turkish.NumberSuffix(int64(line), turkish.Locative), err))
}
