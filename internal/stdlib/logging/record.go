package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// logExt günlük dosyalarının uzantısıdır (JSON satırları).
const logExt = ".jsonl"

// Log TR'de açılmış bir eğitim günlüğüdür (tip Günlük). Kayıtlar dosyanın sonuna, her
// biri tek bir yazma çağrısıyla eklenir. Kontrol noktası gibi başka motorlar Go tarafından
// IsLog, Uzunluk ve WriteResume ile kullanır.
type Log struct {
	m        *interp.VM // gizli değer süzgeci ve program adı (paylaşılan durum)
	d        *state
	path     string // kullanıcının verdiği yol
	fullPath string // çözülmüş yol
	program  string
	opening  time.Time // başlangıç kaydının zamanı

	mu      sync.Mutex
	file    *os.File // nil: kapalı (yeniden yazılınca açılır)
	start   bool     // başlangıç kaydı yazıldı mı
	total   int64    // bağlı çubuğun toplamı (başlangıç kaydı için; 0: yok)
	records int64    // bu açılıştan beri yazılan kayıt sayısı
}

func (g *Log) Type() interp.ValueType { return LogType }
func (g *Log) IsTruthy() bool         { return true }

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (g *Log) Inspect() string { return g.Text(codes.Display()) }

var _ interp.LanguageText = (*Log)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (g *Log) Text(lang *codes.Language) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return messages.TextIn(lang.Code, messages.H7318, lang.Name(codes.A256), g.path, g.records)
}

// Path günlük dosyasının çözülmüş yoludur.
func (g *Log) Path() string { return g.fullPath }

// IsLog değer bir günlük mü? Kontrol noktası gibi motorlar günlüğü bununla tanır.
func IsLog(v interp.Value) (*Log, bool) {
	g, ok := v.(*Log)
	return g, ok
}

func (g *Log) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	if kd == codes.Y004 {
		return bind.CodeMethod(codes.Y004, true, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return g.add(args)
		}), nil
	}
	return nil, bind.MemberError(string(LogType), prop, codes.Names(codes.Y004))
}

// openLog günlük(yol) yerleşiğidir.
func (d *state) openLog(m *interp.VM, args []interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G810)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G810, "1", len(args), messages.Text(messages.H2307))
	}
	path, ok, err := interp.PathArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, interp.ArgTypeError(codes.G810, 1, args[0], codes.A202)
	}
	if !strings.EqualFold(filepath.Ext(path), logExt) {
		return nil, messages.New(messages.H7319,
			fn, path, logExt, strings.TrimSuffix(path, filepath.Ext(path))+logExt)
	}
	g := &Log{m: m, d: d, path: path, fullPath: bind.ResolvePath(m, path), program: filepath.Base(m.SourceFile), opening: clock()}
	if err := os.MkdirAll(filepath.Dir(g.fullPath), 0o755); err != nil {
		return nil, turkish.FileError(fn, path, messages.H2105, err)
	}
	g.mu.Lock()
	err = g.open()
	g.mu.Unlock()
	if err != nil {
		return nil, turkish.FileError(fn, path, messages.H2103, err)
	}
	return g, nil
}

// open dosyayı ekleme kipinde açar; son satır yarım kalmışsa (önceki koşu çöktüyse) yeni
// kayıtlar yeni bir satırda başlasın diye satır sonu ekler. g.mu tutulur.
func (g *Log) open() error {
	f, err := os.OpenFile(g.fullPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if halfLine(g.fullPath) {
		if _, err := f.Write([]byte("\n")); err != nil {
			f.Close()
			return err
		}
	}
	g.file = f
	isOpen.Lock()
	openLogs[g] = struct{}{}
	isOpen.Unlock()
	return nil
}

// halfLine dosya boş değilse ve satır sonuyla bitmiyorsa doğrudur.
func halfLine(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	last, err := f.Seek(-1, io.SeekEnd)
	if err != nil || last < 0 {
		return false
	}
	b := make([]byte, 1)
	n, _ := f.Read(b)
	return n == 1 && b[0] != '\n'
}

// close dosyayı kapatır (sonraki yazmada yeniden açılır). Hiç kayıt yazılmadıysa önce
// başlangıç kaydı yazılır: her açılışın bir başlangıç kaydı olur.
func (g *Log) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.file != nil {
		if !g.start {
			if line, err := g.line(g.startRecord()); err == nil {
				if _, err := g.file.Write(line); err == nil {
					g.start = true
				}
			}
		}
		g.file.Close()
		g.file = nil
	}
	isOpen.Lock()
	delete(openLogs, g)
	isOpen.Unlock()
}

// add Günlük.ekle(kayıt) metodudur: Sözlük bir kayıttır, Metin {"mesaj": ...} olur.
func (g *Log) add(args []interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.Y004)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.Y004, "1", len(args), messages.Text(messages.H2324), messages.Text(messages.H2325))
	}
	record := interp.NewDictVal()
	record.Set(keyTime, timeValue(clock()))
	switch x := args[0].(type) {
	case *interp.StringVal:
		record.Set(keyMessage, x)
	case *interp.DictVal:
		if err := addMetrics(fn, record, x, keyKind); err != nil {
			return nil, err
		}
	default:
		return nil, interp.ArgTypeError(codes.Y004, 1, args[0], codes.A206, codes.A202)
	}
	if err := g.write(record); err != nil {
		return nil, err
	}
	return interp.True, nil
}

// addMetrics kullanıcının sözlüğündeki ölçüleri kayda ekler: anahtarlar Metin olmalı,
// ayrılmış anahtarlar hata verir, tek elemanlı tensörler sayı olur. Kullanıcı "zaman"
// verirse kaydın zamanı onunla değişir.
func addMetrics(fn any, record *interp.DictVal, dict *interp.DictVal, reserved ...string) error {
	for _, k := range dict.Order {
		v, ok := dict.Pairs[k]
		if !ok {
			continue
		}
		if a := dict.KeyValue(k); a != nil {
			if _, text := a.(*interp.StringVal); !text {
				return messages.New(messages.H7320, fn, codes.NameRef(codes.A202), interp.TypeAndSummary(a))
			}
		}
		for _, r := range reserved {
			if k == r {
				return messages.New(messages.H7321, fn, k, reservedReason(k))
			}
		}
		d, err := metricValue(fn, k, v)
		if err != nil {
			return err
		}
		record.Set(k, d)
	}
	return nil
}

// reservedReason bir anahtarın neden ayrılmış olduğunu söyler.
func reservedReason(k string) string {
	switch k {
	case keyKind:
		return messages.Text(messages.H7322)
	case keyTime:
		return messages.Text(messages.H7323)
	case keyStep:
		return messages.Text(messages.H7324)
	}
	return messages.Text(messages.H7325)
}

// write bir kaydı (gerekirse önce başlangıç kaydını) tek bir yazma çağrısıyla ekler.
func (g *Log) write(record *interp.DictVal) error {
	line, err := g.line(record)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.start {
		head, err := g.line(g.startRecord())
		if err != nil {
			return err
		}
		line = append(head, line...)
	}
	if g.file == nil {
		if err := g.open(); err != nil {
			return turkish.FileError(interp.CallPrefix(codes.G810), g.path, messages.H2103, err)
		}
	}
	if _, err := g.file.Write(line); err != nil {
		return turkish.FileError(interp.CallPrefix(codes.G810), g.path, messages.H2101, err)
	}
	g.start = true
	g.records++
	return nil
}

// line kaydı JSON satırına çevirir: NaN ve sonsuz metin olarak, gizli değerler *** olarak.
func (g *Log) line(record *interp.DictVal) ([]byte, error) {
	b, err := jsonwrite.WriteOptions(record, jsonwrite.Options{NonFiniteAsText: true, TextFilter: g.m.Mask})
	if err != nil {
		return nil, messages.New(messages.H7326, interp.CallPrefix(codes.Y004), err)
	}
	return append(b, '\n'), nil
}

// startRecord programın adını, (varsa) bağlı çubuğun toplamını ve komut satırı
// ayarlarını taşır. g.mu tutulur.
func (g *Log) startRecord() *interp.DictVal {
	k := interp.NewDictVal()
	k.Set(keyKind, &interp.StringVal{Val: kindStart})
	k.Set(keyTime, timeValue(g.opening))
	k.Set(keyVersion, interp.NewInt(formatVersion))
	k.Set(keyProgram, &interp.StringVal{Val: g.program})
	if g.total > 0 {
		k.Set(keyTotal, interp.NewInt(g.total))
	}
	if g.d != nil && g.d.process != nil {
		if a := g.d.process.Settings(); a != nil {
			k.Set(keySettings, a)
		}
	}
	return k
}

// setTotal bağlanan çubuğun toplamını başlangıç kaydı için saklar (kayıt henüz
// yazılmadıysa).
func (g *Log) setTotal(n int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.start {
		g.total = n
	}
}

// Len dosyanın şimdiki bayt uzunluğudur. Kayıtlar tek yazma çağrısıyla eklendiğinden
// bu her zaman bir satır sınırıdır. Kontrol noktası kaydederken saklar.
func (g *Log) Len() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	b, err := os.Stat(g.fullPath)
	if err != nil {
		return 0, err
	}
	return b.Size(), nil
}

// WriteResume kontrol noktasından sürdürülen eğitimin {"tür":"devam","adım":N} kaydını yazar.
// N kaydedilen noktanın tamamlanmış adım sayısıdır. Okuyucu kuralı: bir devam kaydından önce
// gelen ve adımı N'den büyük olan ölçü kayıtları geçersizdir (kesilmeden önce yazılmış,
// yeniden hesaplanacak adımlar silinmez; adımı N olan kayıt noktaya dahildir).
func (g *Log) WriteResume(step int64) error {
	k := interp.NewDictVal()
	k.Set(keyKind, &interp.StringVal{Val: kindResume})
	k.Set(keyTime, timeValue(clock()))
	k.Set(keyStep, interp.NewInt(step))
	return g.write(k)
}

// timeValue kayıt zamanının metnidir.
func timeValue(t time.Time) *interp.StringVal {
	return &interp.StringVal{Val: t.Format(timeFormat)}
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Log)(nil)
)
