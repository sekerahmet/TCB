// Package logging, model eğitimini izlemenin iki TR yerleşiğini içerir: JSON satırlarıyla
// eğitim günlüğü (günlük, TR_type_Logger; tip Günlük, TR_type_Logger) ve ilerleme çubuğu (ilerleme, TR_type_Progress; tip
// İlerleme, TR_type_Progress).
//
// Günlük dosyası biçimi (sürüm 2): her satır tek bir JSON nesnesidir ve tek bir yazma
// çağrısıyla dosyanın sonuna eklenir; süreç çökse de en çok son satır yarım kalır.
//
//	{"kind":"start","time":"2026-10-18T14:05:09.123+03:00","version":2,"program":"egit.tr","total":5000,"settings":{"adım":5000}}
//	{"time":"2026-10-18T14:05:10.004+03:00","step":1,"kayıp":4.12,"rate":11.3}
//	{"time":"2026-10-18T14:19:12.870+03:00","doğrulama_kaybı":2.48}
//	{"kind":"resume","time":"...","step":1500}
//	{"kind":"end","time":"2026-10-18T15:06:12.411+03:00","step":5000,"seconds":3663.3}
//
// "kind" taşımayan satırlar ölçü kayıtlarıdır. Biçimin anahtarları ve kayıt türleri dilden
// bağımsız İngilizce tanımlayıcılardır ve çevrilmez (dil tanımı 17.3; TRTENSOR imzası gibi);
// okuyan araçlar (tr izle, tr profil, testkit) bunlara bakar. Ölçülerin ve ayarların adları
// programındır, olduğu gibi yazılır.
// Tanımsız (NaN) ve sonsuz değerler gizlenmez, "NaN", "Infinity", "-Infinity" metni olarak
// yazılır. Bir "resume" kaydı (kontrol noktasından sürdürme, bkz. Log.WriteResume) kendinden
// önce gelen ve adımı ondan büyük olan ölçü kayıtlarını geçersiz kılar (adımı eşit olan kayıt
// kaydedilen noktaya dahildir).
//
// İlerleme çubuğu standart hataya (VM.Err) yazar: terminalde tek satırda en çok saniyede 10
// kez yenilenir (yorumlayıcının canlı satır kancasıyla: yazdır ve hata çıktısı çubuğa
// karışmaz), terminal değilse 10 saniyede bir düz satır ve sonda özet satırı yazılır. Arka
// plan işinde (TR_IS_KLASORU ortam değişkeni) ayrıca iş klasörüne progress.json anlık görüntüsü
// yazılır. Her İlerleme.güncelle çağrısı eğitimin bir adım sınırıdır (SetStepHook).
//
// Gizli değerler (interp.VM.AddSecret) günlükte, çubukta ve progress.json'da ***
// olarak yazılır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package logging

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// Tip adları.
var (
	LogType      = interp.TypeName(codes.TypeLogger)   // Günlük
	ProgressType = interp.TypeName(codes.TypeProgress) // İlerleme
)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameLog  = bind.NamedArg(codes.ArgLog)
	nameStep = bind.NamedArg(codes.ArgStep)
)

// Dosya biçiminin (sürüm 2) anahtarları ve kayıt türleri; bkz. paket belgesi. Dilin adları
// değildir (çevrilmez), bu yüzden tek bir metinde tutulurlar.
var formatNames = strings.Fields("kind time version program total settings step rate seconds message metrics remaining_seconds done start end resume")

var (
	keyKind, keyTime, keyVersion, keyProgram          = formatNames[0], formatNames[1], formatNames[2], formatNames[3]
	keyTotal, keySettings, keyStep, keyRate           = formatNames[4], formatNames[5], formatNames[6], formatNames[7]
	keyDuration, keyMessage, keyMetrics, keyRemaining = formatNames[8], formatNames[9], formatNames[10], formatNames[11]
	keyDone                                           = formatNames[12]
	kindStart, kindEnd, kindResume                    = formatNames[13], formatNames[14], formatNames[15]
)

// formatVersion günlük dosyası biçiminin sürümüdür (başlangıç kaydında).
const formatVersion = 2

// timeFormat kayıtların zaman damgasıdır (milisaniyeli, saat dilimli).
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

// state bir programın (VM'nin) günlük ve ilerleme durumudur. Kaydet her VM için bir tane
// kurar; alanları yorumlayıcı kilidi altında değişir.
type state struct {
	process *process.State
	// active ekranda görünen (canlı satırı olan) çubuktur; yenisi açılınca o görünür.
	active *Progress
	// onScreen terminalde çizili canlı satırın karakter sayısıdır (silmek için); bütün
	// çubuklar aynı satırı paylaşır. Çıktı kilidi (VM.CanliSatirGuncelle) altında değişir.
	onScreen int
}

// Register günlük ve ilerleme yerleşiklerini VM'ye kaydeder. s programın süreç durumudur
// (komut satırı ayarları başlangıç kaydına yazılır); nil olabilir.
func Register(v *interp.VM, s *process.State) {
	d := &state{process: s}
	v.RegisterTypeCall(codes.TypeLogger, &interp.BuiltinVal{NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		return bind.Safe(interp.CallPrefix(codes.TypeLogger), func() (interp.Value, error) { return d.openLog(m, args) })
	}})
	v.RegisterTypeCall(codes.TypeProgress, &interp.BuiltinVal{Fn: bind.SafeBuiltin(interp.CallPrefix(codes.TypeProgress), d.openProgress)})
}

// Açık günlükler ve bitmemiş çubuklar: program bitince CloseAll hepsini kapatır.
var (
	isOpen   sync.Mutex
	openLogs = map[*Log]struct{}{}
	openBars = map[*Progress]struct{}{}
)

// CloseAll bitmemiş çubukları bitirir (son satır ve günlükteki bitiş kaydı) ve açık
// günlük dosyalarını kapatır. Program bitince (hata ya da kesilmeyle bitse de) çağrılır;
// birden çok kez çağrılabilir. Kapatılan bir günlüğe sonra yazılırsa dosya yeniden açılır.
func CloseAll() {
	isOpen.Lock()
	bars := make([]*Progress, 0, len(openBars))
	for c := range openBars {
		bars = append(bars, c)
	}
	isOpen.Unlock()
	// Önce sonra açılanlar: iç içe çubuklarda görünen çubuk önce biter.
	slices.SortFunc(bars, func(a, b *Progress) int { return cmp.Compare(b.order, a.order) })
	for _, c := range bars {
		c.finish(clock())
	}
	isOpen.Lock()
	logs := make([]*Log, 0, len(openLogs))
	for g := range openLogs {
		logs = append(logs, g)
	}
	isOpen.Unlock()
	for _, g := range logs {
		g.close()
	}
}
