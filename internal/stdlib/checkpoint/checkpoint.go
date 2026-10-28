// Package checkpoint uzun eğitimin kontrol noktasıdır: kontrol_noktası(koşu, {ad: nesne, ...})
// (TR_fn_checkpoint; tip KontrolNoktası, TR_type_Checkpoint) eğitimin bütün durumunu koşu klasörüne atomik kaydeder ve
// yeniden başlatılan programı kaldığı adımdan, kesintisiz eğitimle bit düzeyinde aynı
// sürecek biçimde devam ettirir. Kayıt adım sınırında yapılır: kn.güncelle(adım) (TR_mem_update)
// her_dakika süresi dolunca ya da durdurma istenince (Ctrl+C), kn.kaydet() (TR_mem_save) hemen.
//
// Klasör düzeni (biçim sürümü 3; bkz. format.go):
//
//	<koşu>/checkpoints/
//	  latest.json           {"version":3,"step":1500,"dir":"step_0000001500"} (araçlar için)
//	  step_0000001500/
//	    tensors.trt         modellerin tensörleri (türev izlemeyenler dahil), iyileştirici durumları
//	    state.json          adım, program, ayarlar, okuyucu durumları, düz değerler, günlük boyları
//	    digest.json         dosyaların bayt sayısı ve SHA-256 özeti (en son yazılır)
//	  step_0000001000/      tut: 2 → bir önceki de durur
//
// Yazma: nokta önce step_N.tmp-<süreç> klasörüne yazılıp diske işlenir, sonra tek bir
// yeniden adlandırmayla yerine konur, latest.json atomik güncellenir ve tut'u aşan eskiler
// silinir; her aşamada kesilme önceki geçerli noktayı bozmaz. Aralıkla kayıtta yazma hatası
// eğitimi durdurmaz (uyarı, sonraki aralıkta yeniden deneme; art arda 3 başarısızlıkta açık
// hata). Yükleme: en yeni nokta klasörü özetleriyle doğrulanır, bütün nesnelerin uyumu
// hiçbir şey yüklenmeden denetlenir, sonra okuyucular, parametreler (yerinde), iyileştirici
// durumları (parametrelerin cihazına), düz değerler (yerinde) yüklenir ve günlüklere devam
// kaydı yazılır. Bozuk bir nokta açık hatadır ve bir önceki geçerli noktayı önerir.
//
// Kontrol noktası kurulunca ilk durdurma isteği programı hemen durdurmaz (interp.VM.RequestStop):
// istekten sonraki ilk .güncelle ya da .kaydet noktayı kaydeder ve program nazikçe durur (1.6).
// Erteleme süreyle sınırlıdır: istek ikinci .güncelle'den önce geldiyse ya da şimdiye kadarki en uzun iki ardışık .güncelle
// arası sürenin iki katı içinde kayda varılmazsa istek hemen uygulanır. Program durdurmayı
// TR_handle_stop ile de üstlendiyse nokta yalnız kaydedilir. Biterken ek kayıt yapılmaz.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini ve öbür motorları içe aktarır.
package checkpoint

import (
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// CheckpointType KontrolNoktası değerlerinin tip adıdır.
var CheckpointType = interp.TypeName(codes.TypeCheckpoint)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameEveryMinutes = bind.NamedArg(codes.ArgEveryMinutes)
	nameKeep         = bind.NamedArg(codes.ArgKeep)
	nameSeed         = bind.NamedArg(codes.ArgSeed)
)

// Varsayılanlar ve sınırlar.
const (
	defaultMinutes = 15
	defaultKeep    = 2
	maxMinutes     = 1e6
	// maxFailures art arda bu kadar aralıklı kayıt başarısız olursa eğitim durur.
	maxFailures = 3
)

// clock şimdiki zamandır; testler aralığı sınamak için değiştirir.
var clock = time.Now

// Register kontrol_noktası yerleşiğini VM'ye kaydeder. s programın süreç durumudur (komut
// satırı ayarları noktaya yazılır); nil olabilir.
func Register(v *interp.VM, s *process.State) {
	v.RegisterBuiltinCode(codes.FnCheckpoint, bind.SafeBuiltin(interp.CallPrefix(codes.FnCheckpoint), func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return setup(m, s, args, named)
	}))
}

// Checkpoint TR'de kurulmuş bir kontrol noktasıdır (tip KontrolNoktası). Alanları
// yorumlayıcı kilidi altında değişir.
type Checkpoint struct {
	shown    string // kullanıcıya gösterilen klasör: <koşu>/checkpoints
	dir      string // çözülmüş klasör
	objects  []*object
	interval time.Duration
	keep     int
	process  *process.State

	resumed   int64     // yüklenen noktanın adımı (yeni koşuda 0)
	step      int64     // son güncelle(adım)
	lastSave  time.Time // son kayıt ya da kayıt denemesi (aralık buradan sayılır)
	failures  int       // art arda başarısız aralıklı kayıt
	lastValid int64     // son geçerli noktanın adımı (-1: yok)

	// Ertelenen durdurma isteği (1.6): updates .güncelle çağrı sayısı, lastUpdate sonuncusunun
	// zamanı (monoton saat), longest şimdiye kadarki en uzun iki ardışık güncelleme arası, window
	// onun iki katı (nanosaniye; 0: henüz iki güncelleme olmadı, erteleme yok); istek sinyal
	// goroutine'inden geldiği için window atomiktir. honored kayıtlı isteğin bu noktada
	// karşılandığını söyler.
	updates    int
	lastUpdate time.Time
	longest    time.Duration
	window     atomic.Int64
	honored    bool
}

func (kn *Checkpoint) Type() interp.ValueType { return CheckpointType }
func (kn *Checkpoint) IsTruthy() bool         { return true }

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (kn *Checkpoint) Inspect() string { return kn.Text(codes.Display()) }

var _ interp.LanguageText = (*Checkpoint)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (kn *Checkpoint) Text(lang *codes.Language) string {
	return messages.TextIn(lang.Code, messages.H7406, lang.Name(codes.TypeCheckpoint), kn.shown, kn.step, len(kn.objects))
}

// Dir noktaların çözülmüş klasörüdür (<koşu>/checkpoints).
func (kn *Checkpoint) Dir() string { return kn.dir }

func (kn *Checkpoint) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case codes.MemResumeStep:
		return interp.NewInt(kn.resumed), nil
	case codes.MemUpdate:
		return bind.CodeMethod(codes.MemUpdate, true, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return kn.update(m, args)
		}), nil
	case codes.MemSave:
		return bind.CodeMethod(codes.MemSave, true, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return kn.saveTR(m, args)
		}), nil
	case codes.MemRollback:
		return bind.CodeMethod(codes.MemRollback, true, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return kn.rollback(m, args)
		}), nil
	}
	return nil, bind.MemberError(CheckpointType, prop, []codes.Code{codes.MemUpdate, codes.MemResumeStep, codes.MemSave, codes.MemRollback})
}

// setup kontrol_noktası(koşu, nesneler, her_dakika:, tut:) yerleşiğidir.
func setup(m *interp.VM, s *process.State, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnCheckpoint)
	if err := bind.CheckNamed(fn, named, nameEveryMinutes, nameKeep); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.FnCheckpoint, 2, len(args), messages.Text(messages.H2313))
	}
	fpath, ok, err := interp.PathArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok || fpath == "" {
		return nil, interp.ArgTypeError(codes.FnCheckpoint, 1, args[0], codes.TypeString, codes.TypePath)
	}
	dict, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnCheckpoint, 2, args[1], codes.TypeDict)
	}
	interval, keep, err := options(fn, named)
	if err != nil {
		return nil, err
	}
	objects, err := splitObjects(fn, dict)
	if err != nil {
		return nil, err
	}
	runDir := bind.ResolvePath(m, fpath)
	if b, err := os.Stat(runDir); err == nil && !b.IsDir() {
		return nil, messages.New(messages.H7407, fn, fpath, dirName)
	}
	kn := &Checkpoint{shown: path.Join(filepath.ToSlash(fpath), dirName), dir: filepath.Join(runDir, dirName),
		objects: objects, interval: interval, keep: keep, process: s, lastValid: -1}
	if err := kn.load(m, fn); err != nil {
		return nil, err
	}
	kn.lastSave = clock()
	m.DeferStops(kn.stopWindow)
	setupUnseededWarning(m)
	return kn, nil
}

// stopWindow bir durdurma isteğinin adım sınırına ertelenebileceği süreyi verir: şimdiye kadarki en uzun
// iki ardışık .güncelle arasının iki katı (1.6). İkinci .güncelle'den önce gelen istek ertelenmez.
func (kn *Checkpoint) stopWindow() (time.Duration, bool) {
	w := kn.window.Load()
	return time.Duration(w), w > 0
}

// noteUpdate bir .güncelle çağrısının zamanını kaydeder ve erteleme süresini günceller: süre
// şimdiye kadarki en uzun iki ardışık güncelleme arasının iki katıdır (1.6); oynak adım süreli
// (değerlendirme adımı, günlük, ilk adımların derlemesi) bir eğitimde istek kısa bir aralıkla
// ölçülüp kayıtsız uygulanmaz.
func (kn *Checkpoint) noteUpdate(now time.Time) {
	if kn.updates > 0 {
		kn.longest = max(kn.longest, now.Sub(kn.lastUpdate))
		kn.window.Store(2 * int64(kn.longest))
	}
	kn.updates++
	kn.lastUpdate = now
}

// options her_dakika: ve tut: adlı argümanlarını okur.
func options(fn any, named map[string]interp.Value) (time.Duration, int, error) {
	minutes := float64(defaultMinutes)
	if v, ok := bind.GetNamed(named, nameEveryMinutes); ok {
		f, number := interp.ToFloat(v)
		if !number || math.IsNaN(f) || f < 0 || f > maxMinutes {
			return 0, 0, messages.New(messages.H7408, fn, nameEveryMinutes, float64(maxMinutes), interp.TypeAndSummary(v))
		}
		minutes = f
	}
	keep := defaultKeep
	if v, ok := bind.GetNamed(named, nameKeep); ok {
		n, integer := v.(*interp.IntVal)
		if !integer || n.Val < 1 || n.Val > 1000 {
			return 0, 0, messages.New(messages.H7409, fn, nameKeep, codes.NameRef(codes.TypeInt), interp.TypeAndSummary(v))
		}
		keep = int(n.Val)
	}
	return time.Duration(minutes * float64(time.Minute)), keep, nil
}

// setupUnseededWarning tohumsuz rastgelelik kullanılınca bir kez uyarır: sürdürülen eğitim
// kesintisizle aynı olmayacaktır (paylaşılan üretecin durumu kaydedilemez).
func setupUnseededWarning(m *interp.VM) {
	var one sync.Once
	ml.SetUnseededHook(func() {
		one.Do(func() {
			m.WriteOutput(m.Err, messages.Text(messages.H7410,
				codes.NameRef(codes.FnCheckpoint), codes.NameRef(codes.FnRandn), codes.NameRef(codes.FnSample), nameSeed)+"\n")
		})
	})
}

// stepArg güncelle'nin tamamlanan adım argümanını okur.
func stepArg(kd codes.Code, args []interp.Value) (int64, error) {
	if len(args) != 1 {
		return 0, interp.ArgCountError(kd, 1, len(args), messages.Text(messages.H2314))
	}
	n, ok := args[0].(*interp.IntVal)
	if !ok || n.Val < 0 {
		return 0, messages.New(messages.H7411, interp.CallPrefix(kd), codes.NameRef(codes.TypeInt), interp.TypeAndSummary(args[0]))
	}
	return n.Val, nil
}

// update kn.güncelle(adım) metodudur: adım sınırı. Durdurma istendiyse kaydeder ve
// programı durdurur; her_dakika süresi dolduysa kaydeder (kaydettiyse doğru).
func (kn *Checkpoint) update(m *interp.VM, args []interp.Value) (interp.Value, error) {
	n, err := stepArg(codes.MemUpdate, args)
	if err != nil {
		return nil, err
	}
	kn.step = n
	now := clock() // saat güncelleme başına bir kez okunur
	kn.noteUpdate(now)
	if m.StopRequested() && !kn.honored {
		kn.halt(m)
		return interp.True, nil // program durdurmayı üstlenmişti: kaydedildi, program sürer
	}
	if now.Sub(kn.lastSave) < kn.interval {
		return interp.False, nil
	}
	return kn.saveOnInterval(m)
}

// saveOnInterval aralıklı kaydı yapar: yazma hatası eğitimi durdurmaz (uyarı, sonraki
// aralıkta yeniden deneme), art arda maxFailures başarısızlık açık hatadır.
func (kn *Checkpoint) saveOnInterval(m *interp.VM) (interp.Value, error) {
	_, err := kn.save(m)
	kn.lastSave = clock()
	if err == nil {
		kn.failures = 0
		return interp.True, nil
	}
	if _, collecting := err.(collectError); collecting {
		return nil, err
	}
	kn.failures++
	if kn.failures >= maxFailures {
		return nil, messages.New(messages.H7412, err, kn.failures, kn.lastValidText())
	}
	// Süreli kayıtta yeniden deneme bir süre sonradır, yoksa sonraki kaydet çağrısında: iki
	// ayrı cümle, iki kod.
	warning := messages.Text(messages.H7485, err, kn.failures, maxFailures, codes.NameRef(codes.MemUpdate), kn.lastValidText())
	if kn.interval > 0 {
		warning = messages.Text(messages.H7413, err, kn.failures, maxFailures, durationText(kn.interval), kn.lastValidText())
	}
	m.WriteOutput(m.Err, warning+"\n")
	return interp.False, nil
}

// saveTR kn.kaydet() metodudur: hemen kaydeder; hata açık hatadır.
func (kn *Checkpoint) saveTR(m *interp.VM, args []interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemSave, 0, len(args))
	}
	fpath, err := kn.save(m)
	kn.lastSave = clock()
	if err != nil {
		return nil, err
	}
	kn.failures = 0
	if m.StopRequested() && !kn.honored {
		kn.stopAfterSave(m, fpath) // bu kayıt isteği de karşılar: ikinci kez kaydedilmez
	}
	return files.NewPath(fpath), nil
}

// rollback kn.geri_dön() metodudur (20.14): en yeni geçerli noktayı aynı programda yerinde
// yükler ve adımını verir; nokta yoksa açık hatadır. kaldığı_adım değişmez (kurulumun adımıdır).
func (kn *Checkpoint) rollback(m *interp.VM, args []interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.MemRollback, 0, len(args))
	}
	fn := interp.CallName(codes.MemRollback)
	o, err := kn.loadNewest(m, fn)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, messages.New(messages.H7486, fn, kn.shown, codes.NameRef(codes.MemSave), codes.NameRef(codes.MemUpdate))
	}
	m.WriteOutput(m.Err, messages.Text(messages.H7487, o.step, kn.shown, o.name)+"\n")
	return interp.NewInt(o.step), nil
}

// halt kayıtlı durdurma isteğini adım sınırında karşılar: noktayı kaydeder (hata uyarıdır) ve
// program durdurmayı üstlenmediyse nazikçe durdurur (dönmez). Üstlenen program kendi sınırında
// biter; kontrol noktası yalnız kaydeder (1.6).
func (kn *Checkpoint) halt(m *interp.VM) {
	fpath, err := kn.save(m)
	kn.lastSave = clock()
	if err != nil {
		kn.honored = true
		m.WriteOutput(m.Err, messages.Text(messages.H7416, err, kn.lastValidText())+"\n")
		if !m.StopHandled() {
			m.InterruptNow()
		}
		return
	}
	kn.stopAfterSave(m, fpath)
}

// stopAfterSave kaydı yapılmış durdurma isteğini karşılar: kaydı bildirir ve program
// durdurmayı üstlenmediyse nazikçe durdurur (dönmez).
func (kn *Checkpoint) stopAfterSave(m *interp.VM, fpath string) {
	kn.honored = true
	m.WriteOutput(m.Err, messages.Text(messages.H7417, kn.step, fpath)+"\n")
	if !m.StopHandled() {
		m.InterruptNow()
	}
}

// lastValidText son geçerli noktayı söyler (uyarı ve hata mesajları için).
func (kn *Checkpoint) lastValidText() string {
	if kn.lastValid < 0 {
		return messages.Text(messages.H7418)
	}
	return messages.Text(messages.H7419, kn.lastValid, kn.shown, ckptName(kn.lastValid))
}

// durationText bir aralığı kısaca yazar: 15 dk, 30 sn.
func durationText(d time.Duration) string {
	if d >= time.Minute {
		return messages.Text(messages.H7302, strconv.FormatFloat(d.Minutes(), 'g', 3, 64))
	}
	return messages.Text(messages.H7301, strconv.FormatFloat(d.Seconds(), 'g', 3, 64))
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Checkpoint)(nil)
)
