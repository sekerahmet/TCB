package files

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// LineReaderType dosya_satırları'nın verdiği okuyucunun tip adıdır (kod listesinde A257).
var LineReaderType = interp.TypeName(codes.TypeLineReader)

// lineLimit bir satırın en çok bayt sayısıdır (64 MB); daha uzun bir "satır" çoğunlukla
// satırlara bölünmemiş bir dosyadır ve belleği tüketirdi.
var lineLimit = 64 << 20

// readBuffer okuyucunun tampon boyutudur; satırlar bu tampondan yorumlayıcı kilidi
// bırakılmadan alınır, kilit yalnızca diskten okunurken bırakılır.
const readBuffer = 64 << 10

// LineReader dosyayı akışla satır satır okur (interp.Stream). Dosya sonuna gelince, close()
// çağrılınca ya da program bitince (CloseAll) kapanır; erken bırakılan okuyucunun dosyası
// çöp toplayıcı onu topladığında kapanır.
type LineReader struct {
	fpath   string // kullanıcının yazdığı
	mu      sync.Mutex
	r       io.ReadCloser
	br      *bufio.Reader
	counter int
	done    bool // dosya kapandı (sonuna gelindi, okuma hatası ya da close())
	closed  bool // kullanıcı close() ile kapattı: sonraki satır istemek hatadır
}

// openReaders açık satır okuyucularının dosyalarıdır (bağlantıları çözülmüş mutlak yollarıyla):
// program bitince kapatılırlar; sil açık bir dosyayı içeren yolu silmeye kalkınca yol gösterir.
var openReaders = struct {
	sync.Mutex
	m map[io.Closer]string
}{m: map[io.Closer]string{}}

func addEntry(c io.Closer, fpath string) {
	openReaders.Lock()
	openReaders.m[c] = fpath
	openReaders.Unlock()
}

// openReaderInside yolun kendisinde ya da altında açık bir satır okuyucusunun dosyası varsa
// onun yolunu verir.
func openReaderInside(fpath string) string {
	openReaders.Lock()
	defer openReaders.Unlock()
	for _, y := range openReaders.m {
		if isInside(y, fpath) {
			return y
		}
	}
	return ""
}

func closeEntry(c io.Closer) {
	openReaders.Lock()
	_, var_ := openReaders.m[c]
	delete(openReaders.m, c)
	openReaders.Unlock()
	if var_ {
		_ = c.Close()
	}
}

// openFileError yolun kendisi ya da altındaki bir dosya bu programda bir satır okuyucusuyla
// açıksa yol gösteren hatayı verir (Windows'ta açık dosya silinemez ve taşınamaz; işlem yarıda
// kalırdı).
func openFileError(fn any, raw string, real string) error {
	open := openReaderInside(real)
	if open == "" {
		return nil
	}
	if samePath(open, real) {
		return messages.New(messages.H7125, fn, raw, LineReaderType, codes.NameRef(codes.MemClose))
	}
	return messages.New(messages.H7154, fn, raw, filepath.ToSlash(open), LineReaderType, codes.NameRef(codes.MemClose))
}

// CloseAll açık kalmış satır okuyucularının dosyalarını kapatır, ardından geçici_klasör()
// ile oluşturulan klasörleri siler (program bitince); silinemeyen klasörler standart hataya
// uyarı olarak yazılır.
func CloseAll() {
	closeReaders()
	for _, err := range deleteTempDirs() {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H7128, err))
	}
}

// closeReaders açık kalmış satır okuyucularının dosyalarını kapatır.
func closeReaders() {
	openReaders.Lock()
	toClose := make([]io.Closer, 0, len(openReaders.m))
	for c := range openReaders.m {
		toClose = append(toClose, c)
	}
	openReaders.m = map[io.Closer]string{}
	openReaders.Unlock()
	for _, c := range toClose {
		_ = c.Close()
	}
}

// linesBuiltin dosya_satırları(yol) yerleşiğidir: dosyayı açar (hata hemen verilir).
func linesBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnReadLines)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.FnReadLines, 1, len(args), messages.Text(messages.H2307))
	}
	raw, err := readPath(codes.FnReadLines, 1, args[0])
	if err != nil {
		return nil, err
	}
	resolved, tried := m.ResolvePath(filepath.FromSlash(raw))
	if b, err := os.Stat(resolved); err == nil && b.IsDir() {
		return nil, messages.New(messages.H7129, fn, raw)
	}
	var r io.ReadCloser
	m.Blocking(func() { r, err = interp.OpenFileReader(resolved) })
	if err != nil {
		return nil, interp.ReadError(fn, raw, messages.H2100, err, tried)
	}
	o := &LineReader{fpath: raw, r: r, br: bufio.NewReaderSize(r, readBuffer)}
	addEntry(r, realPath(resolved))
	runtime.AddCleanup(o, closeEntry, io.Closer(r))
	return o, nil
}

func (o *LineReader) Type() interp.ValueType { return LineReaderType }
func (o *LineReader) IsTruthy() bool         { return true }

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (o *LineReader) Inspect() string { return o.Text(codes.Display()) }

var _ interp.LanguageText = (*LineReader)(nil)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (o *LineReader) Text(lang *codes.Language) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	state := messages.H7130 // n satır okundu
	switch {
	case o.closed:
		state = messages.H7131 // ..., kapatıldı
	case o.done:
		state = messages.H7132 // ..., bitti
	}
	return messages.TextIn(lang.Code, state, lang.Name(codes.TypeLineReader), o.fpath, o.counter)
}

func (o *LineReader) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	if kd == codes.MemClose {
		return o.closeMethod(), nil
	}
	if kd == codes.MemNext {
		b := &interp.BuiltinVal{Name: codes.Name(codes.MemNext), Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, messages.New(messages.H2014, codes.NameRef(codes.MemNext), len(args))
			}
			d, cont, err := o.Next(m)
			if err != nil || !cont {
				return interp.Null, err
			}
			return d, nil
		}}
		b.NoNamedArgs = true
		return b, nil
	}
	return nil, bind.MemberError(LineReaderType, prop, []codes.Code{codes.MemNext, codes.MemClose})
}

// closeMethod x.kapat() metodudur: dosyayı bırakır; bu çağrı kapattıysa doğru, zaten kapalıysa
// yanlış verir. Kapatma (dosya sistemi çağrısı) yorumlayıcı kilidi bırakılarak yapılır; başka
// bir görev o sırada diskten okuyorsa okuması bitince kapanır.
func (o *LineReader) closeMethod() *interp.BuiltinVal {
	b := &interp.BuiltinVal{Name: codes.Name(codes.MemClose), Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 0 {
			return nil, messages.New(messages.H2014, codes.NameRef(codes.MemClose), len(args))
		}
		var didClose bool
		m.Blocking(func() {
			o.mu.Lock()
			defer o.mu.Unlock()
			didClose = !o.done
			o.closed = true
			o.close()
		})
		return interp.BoolToVal(didClose), nil
	}}
	b.NoNamedArgs = true
	return b
}

// closedError kapatılmış okuyucudan satır istenince verilen hatadır.
func (o *LineReader) closedError() error {
	return messages.New(messages.H7134, LineReaderType, o.fpath, codes.NameRef(codes.MemClose), codes.NameRef(codes.FnReadLines))
}

// Next sıradaki satırı verir; dosya bitince devam=false. Satır tamponda hazırsa yorumlayıcı
// kilidi bırakılmaz; diskten okumak gerekirse bırakılır (eşzamanlı görevler ilerler).
func (o *LineReader) Next(m *interp.VM) (interp.Value, bool, error) {
	if o.mu.TryLock() {
		if o.done {
			closed := o.closed
			o.mu.Unlock()
			if closed {
				return nil, false, o.closedError()
			}
			return nil, false, nil
		}
		line, ok := o.fromBuffer()
		o.mu.Unlock()
		if ok {
			return &interp.StringVal{Val: line}, true, nil
		}
	}
	var line string
	var last, closed bool
	var err error
	m.Blocking(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if closed = o.closed; !closed {
			line, last, err = o.read()
		}
	})
	if closed {
		return nil, false, o.closedError()
	}
	if err != nil {
		return nil, false, turkish.FileError(interp.CallPrefix(codes.FnReadLines), o.fpath, messages.H2100, err)
	}
	if last {
		return nil, false, nil
	}
	return &interp.StringVal{Val: line}, true, nil
}

// fromBuffer tamponda tam bir satır varsa onu alır (diske gitmez). o.mu tutulmalıdır.
func (o *LineReader) fromBuffer() (string, bool) {
	n := o.br.Buffered()
	if n == 0 {
		return "", false
	}
	buf, _ := o.br.Peek(n)
	i := bytes.IndexByte(buf, '\n')
	if i < 0 {
		return "", false
	}
	line := o.lineText(buf[:i])
	_, _ = o.br.Discard(i + 1)
	return line, true
}

// read diskten okuyarak bir sonraki satırı verir; dosya bitince son=true ve dosya kapanır.
// o.mu tutulmalıdır.
func (o *LineReader) read() (line string, last bool, err error) {
	if o.done {
		return "", true, nil
	}
	var long []byte
	for {
		part, err := o.br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			if len(long)+len(part) > lineLimit {
				o.close()
				return "", false, turkish.NewReason(messages.H7105, o.counter+1, codes.NameRef(codes.FnReadFile))
			}
			long = append(long, part...)
			continue
		}
		if len(part) > 0 && part[len(part)-1] == '\n' {
			part = part[:len(part)-1]
		}
		if long != nil {
			part = append(long, part...)
		}
		if err == io.EOF {
			if len(part) == 0 && long == nil {
				o.close()
				return "", true, nil
			}
			// Son satır satır sonusuz: dosya bitti, hemen kapanır (sonraki çağrı yok verir); dosya
			// silinip taşınabilir, okuyucu kapatılmayı beklemez.
			line := o.lineText(part)
			o.close()
			return line, false, nil
		}
		if err != nil {
			o.close()
			return "", false, err
		}
		return o.lineText(part), false, nil
	}
}

// lineText satır sonundaki \r'yi ve ilk satırın başındaki BOM'u atar, sayacı ilerletir.
func (o *LineReader) lineText(b []byte) string {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		b = b[:len(b)-1]
	}
	if o.counter == 0 {
		b = bytes.TrimPrefix(b, bomBytes)
	}
	o.counter++
	return string(b)
}

func (o *LineReader) close() {
	o.done = true
	closeEntry(o.r)
}

// bomBytes UTF-8 bayt sırası işaretidir (bazı programlar metin dosyalarının başına yazar).
var bomBytes = []byte{0xEF, 0xBB, 0xBF}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*LineReader)(nil)
	_ interp.Stream         = (*LineReader)(nil)
)
