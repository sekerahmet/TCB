// Package yurut TR'nin derleyicisi ve yorumlayıcısıdır. Sözdizimi ağacı çalıştırılmadan
// önce bir kez Go kapanışlarına derlenir (derleyici*.go); değişkenler derleme anında
// yuvalara çözülür. Paket ayrıca değerleri, çekirdek yerleşikleri ve metotları,
// modülleri, eşzamanlı görevleri ve yorumlayıcı kilidini içerir. Motorlar (tensör,
// veri, parçalayıcı) ve hata ayıklayıcı bu paketi içe aktarıp kendilerini kancalarla
// bağlar (MotorEkle, YeniHataAyiklayici); yorumlayıcı onları bilmez.
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package yurut

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
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ipucu"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
)

type flowSignal int

const (
	signalNone flowSignal = iota
	signalReturn
	signalBreak
	signalContinue
)

// PropertyGetter nokta (.) operatörüyle özel nesne metodlarına erişim sağlar. kod, adın
// sözlükteki üye kodudur (internal/dil/kod, Y; ad bir üye adı değilse kod.Yok); derleyici onu
// derleme anında bir kez çözer, uygulamalar adlar yerine kodlara göre dallanır. ad
// kullanıcının yazdığı biçimdir (hata mesajları ve öneriler için).
type PropertyGetter interface {
	GetProperty(kd kod.Kod, ad string) (Value, error)
}

// GorevliUyeKancasi, PropertyGetter'ın üyeyi okuyan görevi (VM görünümünü) de alan
// biçimidir. Sonucu göreve özel duruma bağlı olan özellikler (ör. Yol'un var_mı'sı: göreli
// yol, kodun çalıştığı modülün ve programın klasörüne göre çözülür) bunu uygular; uygulayan
// değerde nokta erişimi GetProperty yerine bunu çağırır (GetProperty öneri ve hata yolunda
// görevsiz okumalar için kalır).
type GorevliUyeKancasi interface {
	GorevliUye(v *VM, kd kod.Kod, ad string) (Value, error)
}

// HataAyiklayici, VM'nin her deyimden önce danıştığı hata ayıklayıcıdır. Uygulaması
// tr_dili/internal/arac/ayikla paketindedir; internal/dil/yurut onu içe aktarmaz.
type HataAyiklayici interface {
	// BeforeStatement her deyim çalışmadan önce çağrılır. 0'dan büyük bir satır
	// döndürürse yürütme o satıra atlar.
	BeforeStatement(v *VM, stmt agac.Statement, env *Environment) int
	// KesmeNoktasi koddaki kesme_noktası deyimi çalıştığında, bir sonraki deyimde
	// verilen gerekçeyle durmak için çağrılır.
	KesmeNoktasi(gerekce string)
}

// YeniHataAyiklayici, kesme_noktası deyimi çalıştığında henüz bir hata ayıklayıcı
// bağlı değilse onu oluşturur. tr_dili/internal/motor tarafından atanır; nil ise
// kesme_noktası yok sayılır.
var YeniHataAyiklayici func(sourceFile, sourceCode string) HataAyiklayici

// vmShared tüm görevlerin (ana program ve eşzamanlı bloklar) paylaştığı yorumlayıcı durumudur.
type vmShared struct {
	// Debugger her deyimde okunur; sık okunan alanlar başta tutulur.
	Debugger  HataAyiklayici
	GlobalEnv *Environment
	Out       io.Writer
	// Stdin girdi() için okunacak kaynaktır (nil ise os.Stdin). Düzenleyicinin hata ayıklama
	// modunda standart giriş protokole ayrıldığı için burası değiştirilir. Okuma VM'in tek
	// tamponlu okuyucusuyla yapılır (bkz. GirdiOkuyucu).
	Stdin      io.Reader
	wg         sync.WaitGroup
	SourceFile string
	SourceCode string
	// KomutAdi programı başlatan komuttur (ör. "tr egit.tr", derlenmiş uygulamada
	// "uygulama.exe"); komut satırı seçeneklerinin yardımında ve hatalarında gösterilir.
	// Boşsa "tr <SourceFile>" kullanılır.
	KomutAdi   string
	importedMu sync.Mutex
	Imported   map[string]*DictVal
	// yuklenen yüklenmekte olan modüllerdir (mutlak yol → yükleme); başka bir görev aynı
	// modülü isterse yüklemenin bitmesini bekler.
	yuklenen map[string]*modulYukleme
	// Err programı durdurmayan hataların (eşzamanlı görev hataları) yazıldığı yerdir;
	// varsayılanı standart hatadır. Düzenleyici oturumu ve tr test kendi çıktılarına bağlar.
	Err          io.Writer
	girdiMu      sync.Mutex
	girdiOkuyucu *bufio.Reader

	// gil (Yorumlayıcı Kilidi): Aynı anda yalnızca bir görev TR kodu çalıştırır. Böylece
	// paylaşılan Liste/Sözlük/Yapı değerleri eşzamanlı yazımda bozulmaz ve 's += 1' gibi
	// işlemler bölünmez. Kilit; bekle(), bekle_hepsi(), ağ/disk/süreç beklemeleri gibi
	// engelleyici işlemlerde bırakılır (bkz. Blocking), böylece görevler yine iç içe ilerler.
	gil sync.Mutex
	// bekleyen, kilidi almak için bekleyen görev sayısıdır. Kilidi tutan görev döngü
	// turlarında ve fonksiyon çağrılarında bu sayaca bakar; bekleyen varsa kilidi devreder
	// (bkz. devret). Böylece CPU-yoğun bir döngü eşzamanlı görevleri sonuna kadar
	// bekletmez.
	bekleyen atomic.Int32
	// gorevKancalari, bir görev kilidi her aldığında çağrılan motor kancalarıdır (bkz.
	// GorevKancasiEkle). Yalnızca kayıt sırasında (program başlamadan) yazılır.
	gorevKancalari []func(*VM)

	// argumanlar betiğe komut satırında verilen argümanlardır; argumanlarOkundu program
	// onları okuduysa açıktır (bkz. program_argumanlari.go).
	argumanlar       []string
	argumanlarOkundu atomic.Bool

	// kesildi programın nazikçe durdurulması istendiyse açıktır; kesilmeKanali o anda
	// kapanır (bkz. kesilme.go).
	kesildi       atomic.Bool
	kesilmeKanali chan struct{}
	// kesmeErteleyen kesme isteğini adım sınırına erteleyen kontrol noktası sayısıdır;
	// durdurmaIstendi ertelenen bir istek geldiyse açıktır (bkz. KesmeIste).
	kesmeErteleyen  atomic.Int32
	durdurmaIstendi atomic.Bool

	// ciktiMu çıktı noktalarını (CiktiYaz) etkin canlı satırla sıralar; canliVar canlı satır
	// varken açıktır (yoksa kilit alınmaz). Bkz. cikti.go.
	ciktiMu  sync.Mutex
	canli    CanliSatir
	canliVar atomic.Bool

	// gizliler gizli değer süzgecine kayıtlı değerlerdir; maske onlardan kurulmuş
	// değiştiricidir (gizli değer yoksa nil: çıktı noktalarında maliyet bir atomik okuma).
	gizliMu  sync.Mutex
	gizliler []string
	maske    atomic.Pointer[strings.Replacer]

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
// Her 'eşzamanlı' blok kendi VM görünümüyle çalışır (bkz. NewTaskVM).
//
// Çağrı yığını kilitsizdir: yalnızca görevin kendi goroutine'i okur ve yazar (hata
// ayıklayıcının istekleri de duraklatılmış görevin goroutine'inde işlenir).
type VM struct {
	*vmShared
	CallStack []StackFrame

	holdsGIL  bool // bu görünüm şu an yorumlayıcı kilidini tutuyor mu (yalnızca kendi goroutine'i okur/yazar)
	asyncTask bool // 'eşzamanlı' bloğu içinde mi çalışıyor

	// kaynak şu an çalışan kodun dosyasıdır (nil: ana program; bkz. Kaynak). Paylaşılan
	// durumda değil görevde tutulur: bir modül yüklenirken kilidi bırakınca başka bir görev
	// kendi dosyasının bağlamında kalır. Başka dosyada tanımlı bir fonksiyon çağrılınca o
	// fonksiyonun dosyasına geçilir (kaynaktaCagir).
	kaynak *Kaynak
	// yukleme bu görevin yüklemekte olduğu modüllerin (mutlak yol) zinciridir; döngüsel
	// aktarımı, başka bir görevin yüklemesini beklemekten ayırır.
	yukleme []string
	// etkinBlok hata ayıklayıcıya danışılan son deyimin bloğudur (bkz. ZiplamaHedefi).
	etkinBlok *blokKodu

	// gorevDegerleri motorların göreve özel durumudur (ör. tensör motorunun
	// karışık_hassasiyet bloğu); bkz. GorevDegeri. Çağrı yığını gibi yalnızca görevin
	// kendi goroutine'i okur ve yazar; yeni görevler (NewTaskVM) boş başlar.
	gorevDegerleri map[any]any

	// kesmeAtildi bu görevin kesilme paniğini attığını söyler: yığın çözülürken çalışan
	// 'ertele' blokları yeniden kesilmez (bkz. kesilmeyiDenetle).
	kesmeAtildi bool
}

// GorevDegeri bu göreve (VM görünümüne) anahtarla bağlanmış motor durumunu verir; yoksa
// nil. Değer başka görevlere (eşzamanlı bloklar) görünmez; görev GIL'i
// bıraktığında (Blocking) başka bir görev kendi değerini okur. Anahtar, motorun dışa
// açılmamış bir tipinden olmalıdır (çakışma olmasın).
func (v *VM) GorevDegeri(anahtar any) any { return v.gorevDegerleri[anahtar] }

// GorevDegeriAyarla bu göreve bağlı motor durumunu ayarlar; deger nil ise siler.
func (v *VM) GorevDegeriAyarla(anahtar, deger any) {
	if deger == nil {
		delete(v.gorevDegerleri, anahtar)
		return
	}
	if v.gorevDegerleri == nil {
		v.gorevDegerleri = map[any]any{}
	}
	v.gorevDegerleri[anahtar] = deger
}

func New() *VM {
	env := NewEnvironment(nil)
	v := &VM{
		vmShared: &vmShared{
			GlobalEnv: env,
			Out:       os.Stdout,
			Err:       os.Stderr,
			Imported:  make(map[string]*DictVal),
			yuklenen:  make(map[string]*modulYukleme),

			kesilmeKanali: make(chan struct{}),
		},
		CallStack: []StackFrame{},
	}
	v.tipleriKaydet()
	v.registerBuiltins()
	v.registerIstatistik()
	v.registerMotors()
	v.registerTestBuiltins()
	return v
}

// NewTaskVM bir 'eşzamanlı' görev için aynı paylaşılan durum üzerinde, kendi çağrı
// yığınına sahip yeni bir görev görünümü üretir. Çağıran, yorumlayıcı kilidini tutan VM
// goroutine'idir; görev başlatıldığı yerin çağrı yığınını devralır (hata ayıklayıcı yığını bu
// sayede anlamlıdır).
func (v *VM) NewTaskVM() *VM {
	// Görev başlatıldığı yerin dosyasında ve aktarım zincirinde başlar (modül yüklenirken
	// başlatılan görev o modülü aktarırsa bu döngüdür, beklemek kilitlenirdi).
	return &VM{vmShared: v.vmShared, CallStack: v.StackSnapshot(), asyncTask: true,
		kaynak: v.kaynak, yukleme: slices.Clip(v.yukleme)}
}

func (v *VM) acquireGIL() {
	v.bekleyen.Add(1)
	v.gil.Lock()
	v.bekleyen.Add(-1)
	v.holdsGIL = true
	for _, f := range v.gorevKancalari {
		f(v)
	}
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
	v.kesilmeyiDenetle()
}

func (v *VM) releaseGIL() {
	v.holdsGIL = false
	v.gil.Unlock()
}

// Blocking fn'i yorumlayıcı kilidini bırakarak çalıştırır; uyku, ağ isteği, dış süreç gibi
// uzun süren işlemler sırasında diğer görevlerin ilerlemesini sağlar.
func (v *VM) Blocking(fn func()) {
	if !v.holdsGIL {
		fn()
		return
	}
	v.releaseGIL()
	defer func() {
		v.acquireGIL()
		// Bekleme sırasında kesme istendiyse (bekle, veri okuma ...) program burada durur.
		v.kesilmeyiDenetle()
	}()
	fn()
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

func (v *VM) Run(prog *agac.Program) (result Value, err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	// Yerleşik fonksiyonlardaki beklenmedik paniklerin tüm süreci çökertmesini engelle
	yiginBoyu := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			v.yiginiKirp(yiginBoyu)
			if KesilmeMi(r) {
				// Kesilme: ertelenen işler çalıştı; onların hatası varsa kesilmeye eklenir.
				result, err = nil, erteleHatasiEkle(ErrKesildi, err)
				return
			}
			result, err = nil, panikHatasi(r)
		}
	}()
	defer func() {
		// Programın üst düzey ertelenen işleri; hataları programın hatası olur (9.4)
		if derr := v.GlobalEnv.RunDefers(); derr != nil {
			result, err = nil, erteleHatasiEkle(err, derr)
		}
	}()
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
// Öneri denetleyiciyle ortaktır (ipucu.AdOnerisi): tanımsız ad çalışmadan önce de aynı
// mesajla bulunur.
func suggestName(name string, env *Environment) string {
	return ipucu.AdOnerisi(name, env.Names())
}

func (v *VM) reportAsyncError(line int, err error) {
	w := v.Err
	if w == nil {
		w = os.Stderr
	}
	v.CiktiYaz(w, fmt.Sprintf("⚠️  Eşzamanlı görev hatası (%s başlatılan): %v\n", "satır "+turkce.SayiEk(int64(line), turkce.Bulunma), err))
}
