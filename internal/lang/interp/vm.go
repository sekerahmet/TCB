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
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
// yet. It is set by github.com/sekerahmet/TCB/internal/devtools/debugger; when nil, kesme_noktası is
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
	liveTasks  atomic.Int32 // başlamış, bitmemiş eşzamanlı görev sayısı (wg'nin sayısı; okunabilir)
	SourceFile string
	SourceCode string
	// CommandName programı başlatan komuttur (ör. "trlang egit.tr", derlenmiş uygulamada
	// "uygulama.exe"); komut satırı seçeneklerinin yardımında ve hatalarında gösterilir.
	// Boşsa "trlang <SourceFile>" kullanılır.
	CommandName string
	isImported  sync.Mutex
	Imported    map[string]*DictVal
	// moduleRoot modüllerin genel kapsamıdır: yalnız yerleşikler (New'in sonunda GlobalEnv'den
	// kopyalanır). Bir dosya kendisini aktaranın adlarını görmez (5.14); ana programın adları
	// GlobalEnv'de yerleşiklerden sonra gelir, buraya girmez.
	moduleRoot *Environment
	// taskErrors, bir test koşarken (RunTest) testin başlattığı görevlerin hatalarını toplar;
	// test bitince testin hatası olurlar (10.1). nil iken görev hataları Err'e yazılır.
	taskErrorsMu sync.Mutex
	taskErrors   *[]error
	// heldTasks are the tasks whose value was kept and that ended with an error; at the end of the
	// program or the test the ones nobody waited for are written (10.6, reportUnwaitedTasks).
	heldTasks []heldTaskError
	// loaded yüklenmekte olan modüllerdir (mutlak yol → yükleme); başka bir görev aynı
	// modülü isterse yüklemenin bitmesini bekler.
	loaded map[string]*moduleLoading
	// waiting is the module each task (VM view) is waiting for another task to load; with
	// moduleLoading.owner it detects import cycles across tasks.
	waiting map[*VM]string
	// Err programı durdurmayan hataların (eşzamanlı görev hataları) yazıldığı yerdir;
	// varsayılanı standart hatadır. Düzenleyici oturumu ve trlang test kendi çıktılarına bağlar.
	Err         io.Writer
	isInput     sync.Mutex
	inputReader *bufio.Reader
	// builtinConstants genel kapsama kaydedilen yerleşik sabitlerdir (ad → değer; ör. pi);
	// hata ayıklayıcı onları kullanıcı değişkenlerinden ayırır (YerlesikSabitMi).
	builtinConstants map[string]Value
	// typeCalls tiplerin çağrısıdır (3.12; tip kodu → kurucu yerleşik, RegisterTypeCall). Yalnız
	// kayıt sırasında yazılır.
	typeCalls map[codes.Code]*BuiltinVal
	// programValues motorların programa bağlı durumudur (bütün görevler paylaşır; ör. tensör
	// motorunun bir kez gösterilen uyarıları). Süreç geneli değişkende tutulsaydı aynı süreçte
	// çalışan programlar (trlang test'in dosyaları, düzenleyici oturumu) birbirini etkilerdi.
	isProgramValue sync.Mutex
	programValues  map[any]any

	// gil (Yorumlayıcı Kilidi): Aynı anda yalnızca bir görev TR kodu çalıştırır. Böylece
	// paylaşılan Liste/Sözlük/Yapı değerleri eşzamanlı yazımda bozulmaz ve 's += 1' gibi
	// işlemler bölünmez. Kilit; bekle(), bekle_hepsi(), ağ/disk/süreç beklemeleri gibi
	// engelleyici işlemlerde bırakılır (bkz. Blocking), böylece görevler yine iç içe ilerler.
	// Bekleyenler kilidi geldikleri sırayla alır (gil.go, 10.2).
	gil gilQueue
	// pending, kilidi almak için bekleyen görev sayısıdır (durdurma da bir ekler, bkz.
	// Interrupt). Kilidi tutan görev her deyim sınırında bu sayaca bakar; sıfır değilse ve kilidi
	// lockHold'dan uzun tuttuysa devreder (yieldPoint). Böylece CPU-yoğun bir döngü eşzamanlı
	// görevleri sonuna kadar bekletmez.
	pending atomic.Int32
	// gilWaitHook yalnız testler içindir: kilidi bekleyerek alan görev, kuyruğa girdiğinde önünde
	// bekleyen sayısını, numarasını ve bekleme süresini (tekdüze saatle) bildirir.
	gilWaitHook func(ahead int, ticket uint64, waited time.Duration)
	// taskHooks, bir görev kilidi her aldığında çağrılan motor kancalarıdır (bkz.
	// GorevKancasiEkle). Yalnızca kayıt sırasında (program başlamadan) yazılır.
	taskHooks []func(*VM)
	// forkHooks, görev açılırken çağrılan motor kancalarıdır (bkz. AddForkHook). Yalnızca kayıt
	// sırasında yazılır.
	forkHooks []func(parent, task *VM)
	// mainEnds program bitince, görevler beklenmeden önce çalışan motor işleridir (AddMainEnd).
	mainEnds []func(y *VM)

	// arguments betiğe komut satırında verilen argümanlardır (bkz. program_args.go).
	arguments []string

	// Durdurma durumu (bkz. interrupt.go). interrupted 1.5 uygulandığında açılır, interruptChan
	// o anda kapanır; stopSignal ilk isteğin sinyal numarasıdır (0: istek gelmedi); stopHandled
	// TR_handle_stop çağrıldıysa açıktır, stopRequested isteğin kaydedildiğini (üstlenilmiş ya da
	// ertelenmiş) söyler; stopChan üstlenilmiş programa istek gelince kapanır; stopWindows
	// kontrol noktalarının erteleme sürelerini verir, stopTimer ertelemenin bitiş zamanlayıcısıdır;
	// finishing en dıştaki ertelenen işler çalışırken açıktır (1.15). stopMu bu kararları korur.
	interrupted   atomic.Bool
	interruptChan chan struct{}
	stopMu        sync.Mutex
	stopSignal    atomic.Int32
	stopHandled   atomic.Bool
	stopRequested atomic.Bool
	stopChan      chan struct{}
	stopChanSent  bool // stopChan kapandı (stopMu altında)
	stopWindows   []func() (time.Duration, bool)
	hardStops     []func() // HardStop'un işleri
	// endWork motorların program ya da test biterken yaptığı işlerdir (AddEndWork).
	endWork   []func() []EndError
	stopTimer *time.Timer
	finishing atomic.Bool
	// interactive etkileşimli kabukta açıktır: VM birden çok Run ile sürer, hatayla biten satır
	// görevleri durdurmaz (bkz. SetInteractive).
	interactive bool

	// Çıkış durumu (bkz. program_exit.go): TR_exit'in son çağrısı kazanır (1.13). testing bir test
	// koşarken sıfırdan büyüktür: orada çıkış süreci bitirmez, testi düşürür (1.12). taskFailed
	// bir görevde yakalanmayan hata olduğunu söyler (1.9).
	exitMu        sync.Mutex
	exitRequested bool
	exitCode      int
	exitMessage   string
	testing       atomic.Int32
	taskFailed    atomic.Bool

	// pendingRead girdi() için başlamış, sonucu henüz alınmamış satır okumasıdır: bırakılan bir
	// bekleme okunan satırı yitirmez (bkz. input.go). inputEnded girdi() girdinin sonunu bir kez
	// TR_none ile bildirdikten sonra açıktır: sonraki çağrı H2561'dir (13.1). İkisi isInput'la korunur.
	pendingRead *lineRead
	inputEnded  bool

	// isOutput çıktı noktalarını (CiktiYaz) etkin canlı satırla sıralar; canliVar canlı satır
	// varken açıktır (yoksa kilit alınmaz). Bkz. output.go.
	isOutput sync.Mutex
	live     LiveLine
	hasLive  atomic.Bool

	// gizliler gizli değer süzgecine kayıtlı değerlerdir (kaçışlı biçimleriyle); maske onlardan
	// kurulmuş değer maskesidir (gizli değer yoksa nil: adressiz çıktıda maliyet bir atomik okuma ve
	// bir strings.Index).
	isSecret sync.Mutex
	secrets  []string
	mask     atomic.Pointer[valueMask]
	// maskBuilds süzgecin kaç kez kurulduğudur (isSecret'la korunur); yalnız toplu kaydın testi okur.
	maskBuilds int

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

	holdsGIL bool // bu görünüm şu an yorumlayıcı kilidini tutuyor mu (yalnızca kendi goroutine'i okur/yazar)
	// blocking bu görünümün Blocking içinde kilidi bırakmış olduğudur (Locked kilidi yalnız o
	// zaman geri alır; yalnızca kendi goroutine'i okur/yazar).
	blocking  bool
	asyncTask bool // 'eşzamanlı' bloğu içinde mi çalışıyor
	// task bu görünümün çalıştırdığı görevin değeridir (ana programda nil; task_value.go).
	task *TaskVal
	// exclusive boş değilse görev kilidi başka göreve devretmez (bkz. Exclusive); değeri kipi
	// isteyen yerleşiğin kullanıcıya görünen adıdır.
	exclusive any
	// gilGen ve gilSince bu görevin tuttuğu kilidin kuşağı ve alındığı andır (monotonic); clock
	// yedek saattir (gil.go). Yalnız kilidi tutan görev okur ve yazar.
	gilGen   uint64
	gilSince time.Duration
	clock    lockClock

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
	// kendi goroutine'i okur ve yazar; yeni görev (NewTaskVM) boş başlar, açanın durumundan
	// yalnız motorun kancasının (AddForkHook) verdiğini alır.
	taskValues map[any]any

	// interruptThrown bu görevin kesilme paniğini attığını söyler: yığın çözülürken çalışan
	// 'ertele' blokları yeniden kesilmez (bkz. checkInterrupt). cleanupDepth görevin şu an kaç
	// temizlik işinin (ertele bloğu, durdurma hatasını yakalayan yakala gövdesi) içinde olduğudur:
	// orada beklemeler bırakılmaz ve durdurma hatası vermez (1.5, 1.6).
	interruptThrown bool
	cleanupDepth    int
	// builtinLine son yerleşik çağrısının satırıdır (BuiltinLine).
	builtinLine int
	// unwindErrors yığın panikle çözülürken (durdurma 1.5, iptal 10.9) çalışan ertelenen işlerin
	// hatalarıdır: çerçevenin dönüş değeri atıldığından burada birikir, programın, testin ya da
	// görevin hatasına eklenir (9.4; takeUnwindErrors).
	unwindErrors []error

	// owners çağrı derinliğine göre sahiplik çerçeveleri (0: üst düzey); yalnızca görevin
	// kendi goroutine'i kullanır (ownership.go).
	owners []*ownerFrame
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

// ProgramValue programa bağlı motor durumunu verir; anahtar için ilk çağrıda create ile
// kurulur. Durum programın bütün görevlerince paylaşılır; eşzamanlı erişimi korumak durumun
// kendi işidir. Anahtar motorun dışa açık olmayan bir tipi olmalıdır (çakışmasın).
func (v *VM) ProgramValue(key any, create func() any) any {
	v.isProgramValue.Lock()
	defer v.isProgramValue.Unlock()
	if val, ok := v.programValues[key]; ok {
		return val
	}
	if v.programValues == nil {
		v.programValues = map[any]any{}
	}
	val := create()
	v.programValues[key] = val
	return val
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
			stopChan:      make(chan struct{}),
		},
		CallStack: []StackFrame{},
	}
	v.registerTypes()
	v.registerTypeCalls()
	v.registerBuiltins()
	v.registerStats()
	v.registerMath()
	v.registerEngines()
	v.registerTestBuiltins()
	v.registerMessageBuiltins()
	v.registerErrorType()
	v.moduleRoot = builtinsOnly(env)
	return v
}

// NewTaskVM creates a new task view for an 'eşzamanlı' task over the same shared state, with
// its own call stack. The caller is the VM goroutine holding the interpreter lock; the task
// inherits the call stack of the place it was started from (which keeps the debugger's stack
// meaningful).
func (v *VM) NewTaskVM() *VM {
	task := v.forkView()
	for _, f := range v.forkHooks {
		f(v, task)
	}
	return task
}

// AddForkHook adds a hook that runs when a task is opened, on the opener's goroutine, before the
// task starts: an engine copies what the task takes from where it was opened (10.4). The task
// then keeps its own copy: a later change in the opener does not reach it. Call it only during
// engine registration (before the program starts).
func (v *VM) AddForkHook(f func(parent, task *VM)) {
	v.forkHooks = append(v.forkHooks, f)
}

// Blocking runs fn with the interpreter lock released, so other tasks make progress during
// long operations such as sleeping, network requests or external processes.
// Inside Exclusive the lock is kept: fn runs while holding it.
func (v *VM) Blocking(fn func()) {
	if !v.holdsGIL || v.exclusive != nil {
		fn()
		return
	}
	v.releaseGIL()
	v.blocking = true
	defer func() {
		v.blocking = false
		v.acquireGIL()
		// Bekleme sırasında kesme istendiyse (bekle, veri okuma ...) program burada durur.
		v.checkInterrupt()
	}()
	fn()
}

// BuiltinLine is the line of the builtin call running on this task: a builtin reads it before it
// calls anything back, for a message that names its call's line.
func (v *VM) BuiltinLine() int { return v.builtinLine }

// HoldsLock reports whether this task holds the interpreter lock (false inside Blocking).
func (v *VM) HoldsLock() bool { return v.holdsGIL }

// Locked runs fn holding the interpreter lock: inside this view's Blocking it takes the lock
// back for fn and releases it again afterwards; otherwise fn runs as is. Calls back into TR
// code (ApplyFunction) are made only holding the lock (S-kilit, step 4): Go code running with
// the lock released calls them through here.
//
// Only this view's own Blocking counts, not holdsGIL: a callback may run on a view that does
// not hold the lock while the goroutine holds it through another view (a custom gradient's
// backward started from Go without a context runs on the forward task's view). Taking the
// lock there would wait for the goroutine itself.
func (v *VM) Locked(fn func()) {
	if !v.blocking {
		fn()
		return
	}
	v.acquireGIL()
	v.blocking = false
	defer func() {
		v.blocking = true
		v.releaseGIL()
	}()
	fn()
}

// OtherTasksLive reports whether a task other than v may run: in the main program, an
// 'eşzamanlı' task that is started and not finished; in a task, always (the main program is
// counted as live even while it waits). Long tensor kernels that release the lock leave
// processors free only then (S-kilit).
func (v *VM) OtherTasksLive() bool {
	return v.asyncTask || v.liveTasks.Load() > 0
}

// Exclusive runs fn without handing the interpreter lock to another task, not even during
// blocking operations: no other task's work can run in between (tekrar_oynat's GPU recording,
// where another task's kernels would enter the recording). owner is the user-visible name of
// the builtin asking for it; waiting for tasks (bekle_hepsi) inside it is an error, since
// they could never run.
func (v *VM) Exclusive(owner any, fn func()) {
	prev := v.exclusive
	v.exclusive = owner
	defer func() { v.exclusive = prev }()
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

// Run programı çalıştırır. Program sona gelince (ya da hata, TR_exit ve durdurmayla yarıda
// kalınca) önce görevlerini bekler, sonra en dıştaki ertelenen işler çalışır (1.9); onların
// hataları programın hatasına eklenir (9.4). Dönen hata bir durdurmaysa ErrInterrupted, TR_exit
// ise *ExitRequest içerir; sürecin nasıl biteceğini End söyler.
func (v *VM) Run(prog *ast.Program) (Value, error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	val, err := v.runMain(prog)
	err = v.takeUnwindErrors(err) // durdurmada ertelenen işlerin hataları (9.4)
	// Yakalanmayan bir hatayla biten programda öbür görevler 1.5'teki gibi durur (1.9); etkileşimli
	// kabukta VM sonraki satırlarla sürdüğünden durdurulmaz.
	if _, main := splitProgramErrors(err); main != nil && !v.interactive {
		v.Interrupt()
	}
	v.runMainEnds() // açık sunucular kapanır (28.8): görevleri yeni istek beslemez
	// Görevlere durma emri gittiyse (durdurma, TR_exit, hata) ya da program hatasız bittiyse görevler
	// beklenir. Görevleri bekleyen bu bekleme bir temizlik işidir: bir istek onu kesmez.
	if err == nil || v.interrupted.Load() {
		v.cleanupDepth++
		v.Blocking(v.wg.Wait)
		v.cleanupDepth--
	}
	v.finishing.Store(true) // bundan sonra gelen ilk istek ertelenen işleri kesmez (1.15)
	if derr := v.runDefers(v.GlobalEnv); derr != nil {
		val, err = nil, addDeferError(err, derr)
	}
	v.reportUnwaitedTasks() // ertelenen iş görevi bekleyebilir: ondan sonra (10.6)
	return val, err
}

// runMain programın deyimlerini çalıştırır. Yerleşik fonksiyonlardaki beklenmedik paniklerin tüm
// süreci çökertmesini engeller; durdurma paniği ErrInterrupted olur.
func (v *VM) runMain(prog *ast.Program) (result Value, err error) {
	stackSize := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			v.trimStack(stackSize)
			if IsInterrupt(r) {
				result, err = nil, ErrInterrupted
				return
			}
			result, err = nil, panicError(r)
		}
	}()
	val, _, runErr := v.runProgram(prog.Statements)
	if runErr != nil {
		return nil, runErr
	}
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
func (v *VM) suggestName(name string, env *Environment) *messages.Error {
	// Program bir dosyadır, dili yoktur (2.1): öneri kimliğin yazımıdır (TR_print).
	return hint.NameSuggestion(name, env.Names(), nil)
}

func (v *VM) reportAsyncError(line int, err error) {
	v.taskErrorsMu.Lock()
	if v.taskErrors != nil {
		*v.taskErrors = append(*v.taskErrors, messages.New(messages.H2516, line, err))
		v.taskErrorsMu.Unlock()
		return
	}
	v.taskErrorsMu.Unlock()
	v.taskFailed.Store(true) // çıkış kodu 1 olur (1.9)
	w := v.Err
	if w == nil {
		w = os.Stderr
	}
	v.WriteOutput(w, turkish.Printable(messages.Text(messages.H2454, line, err)))
}
