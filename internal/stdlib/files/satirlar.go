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
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// SatirOkuyucuTipi dosya_satırları'nın verdiği okuyucunun tip adıdır (kod listesinde A257).
var SatirOkuyucuTipi = interp.TypeName(codes.A257)

// satirSiniri bir satırın en çok bayt sayısıdır (64 MB); daha uzun bir "satır" çoğunlukla
// satırlara bölünmemiş bir dosyadır ve belleği tüketirdi.
var satirSiniri = 64 << 20

// okumaTamponu okuyucunun tampon boyutudur; satırlar bu tampondan yorumlayıcı kilidi
// bırakılmadan alınır, kilit yalnızca diskten okunurken bırakılır.
const okumaTamponu = 64 << 10

// SatirOkuyucu dosyayı akışla satır satır okur (yurut.Akis). Dosya sonuna gelince, kapat()
// çağrılınca ya da program bitince (TumunuKapat) kapanır; erken bırakılan okuyucunun dosyası
// çöp toplayıcı onu topladığında kapanır.
type SatirOkuyucu struct {
	yol       string // kullanıcının yazdığı
	mu        sync.Mutex
	r         io.ReadCloser
	br        *bufio.Reader
	sayac     int
	bitti     bool // dosya kapandı (sonuna gelindi, okuma hatası ya da kapat())
	kapatildi bool // kullanıcı kapat() ile kapattı: sonraki satır istemek hatadır
}

var (
	_ interp.Stream         = (*SatirOkuyucu)(nil)
	_ interp.PropertyGetter = (*SatirOkuyucu)(nil)
)

// acikOkuyucular açık satır okuyucularının dosyalarıdır (bağlantıları çözülmüş mutlak yollarıyla):
// program bitince kapatılırlar; sil açık bir dosyayı içeren yolu silmeye kalkınca yol gösterir.
var acikOkuyucular = struct {
	sync.Mutex
	m map[io.Closer]string
}{m: map[io.Closer]string{}}

func kayitEkle(c io.Closer, yol string) {
	acikOkuyucular.Lock()
	acikOkuyucular.m[c] = yol
	acikOkuyucular.Unlock()
}

// acikOkuyucuIcinde yolun kendisinde ya da altında açık bir satır okuyucusunun dosyası varsa
// onun yolunu verir.
func acikOkuyucuIcinde(yol string) string {
	acikOkuyucular.Lock()
	defer acikOkuyucular.Unlock()
	for _, y := range acikOkuyucular.m {
		if icindeMi(y, yol) {
			return y
		}
	}
	return ""
}

func kayitKapat(c io.Closer) {
	acikOkuyucular.Lock()
	_, var_ := acikOkuyucular.m[c]
	delete(acikOkuyucular.m, c)
	acikOkuyucular.Unlock()
	if var_ {
		_ = c.Close()
	}
}

// acikDosyaHatasi yolun kendisi ya da altındaki bir dosya bu programda bir satır okuyucusuyla
// açıksa yol gösteren hatayı verir (Windows'ta açık dosya silinemez ve taşınamaz; işlem yarıda
// kalırdı).
func acikDosyaHatasi(fn, ham, gercek string) error {
	acik := acikOkuyucuIcinde(gercek)
	if acik == "" {
		return nil
	}
	ne := fmt.Sprintf("içindeki %q", filepath.ToSlash(acik))
	if ayniYol(acik, gercek) {
		ne = "dosya"
	}
	return fmt.Errorf("%s: %q: %s bu programda bir %s ile okunuyor; önce okuyucuyu kapatın (x.%s()) ya da dosyayı sonuna kadar okuyun",
		fn, ham, ne, SatirOkuyucuTipi, codes.Name(codes.Y012))
}

// TumunuKapat açık kalmış satır okuyucularının dosyalarını kapatır, ardından geçici_klasör()
// ile oluşturulan klasörleri siler (program bitince); silinemeyen klasörler standart hataya
// uyarı olarak yazılır.
func TumunuKapat() {
	okuyuculariKapat()
	for _, err := range geciciKlasorleriSil() {
		fmt.Fprintf(os.Stderr, "Uyarı: %v\n", err)
	}
}

// okuyuculariKapat açık kalmış satır okuyucularının dosyalarını kapatır.
func okuyuculariKapat() {
	acikOkuyucular.Lock()
	kapatilacak := make([]io.Closer, 0, len(acikOkuyucular.m))
	for c := range acikOkuyucular.m {
		kapatilacak = append(kapatilacak, c)
	}
	acikOkuyucular.m = map[io.Closer]string{}
	acikOkuyucular.Unlock()
	for _, c := range kapatilacak {
		_ = c.Close()
	}
}

// satirlarYerlesik dosya_satırları(yol) yerleşiğidir: dosyayı açar (hata hemen verilir).
func satirlarYerlesik(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G112)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G112, "1", len(args), "dosya")
	}
	ham, err := yolOku(codes.G112, 1, args[0])
	if err != nil {
		return nil, err
	}
	cozulen, bakilanlar := m.ResolvePath(filepath.FromSlash(ham))
	if b, err := os.Stat(cozulen); err == nil && b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir klasör; satırları okunacak dosyanın yolunu verin", fn, ham)
	}
	var r io.ReadCloser
	m.Blocking(func() { r, err = interp.OpenFileReader(cozulen) })
	if err != nil {
		return nil, interp.ReadError(fn, ham, "okunamadı", err, bakilanlar)
	}
	o := &SatirOkuyucu{yol: ham, r: r, br: bufio.NewReaderSize(r, okumaTamponu)}
	kayitEkle(r, gercekYol(cozulen))
	runtime.AddCleanup(o, kayitKapat, io.Closer(r))
	return o, nil
}

func (o *SatirOkuyucu) Type() interp.ValueType { return SatirOkuyucuTipi }
func (o *SatirOkuyucu) IsTruthy() bool         { return true }

func (o *SatirOkuyucu) Inspect() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	durum := fmt.Sprintf("%d satır okundu", o.sayac)
	switch {
	case o.kapatildi:
		durum += ", kapatıldı"
	case o.bitti:
		durum += ", bitti"
	}
	return fmt.Sprintf("%s(%s, %s)", SatirOkuyucuTipi, o.yol, durum)
}

func (o *SatirOkuyucu) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	if kd == codes.Y012 {
		return o.kapatMetodu(), nil
	}
	if kd == codes.Y602 {
		b := &interp.BuiltinVal{Name: codes.Name(codes.Y602), Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() parametre almaz, %d verildi", codes.Name(codes.Y602), len(args))
			}
			d, devam, err := o.Next(m)
			if err != nil || !devam {
				return interp.NULL, err
			}
			return d, nil
		}}
		b.NoNamedArgs = true
		return b, nil
	}
	return nil, bind.UyeHatasi(string(SatirOkuyucuTipi), prop, codes.Names(codes.Y602, codes.Y012))
}

// kapatMetodu x.kapat() metodudur: dosyayı bırakır; bu çağrı kapattıysa doğru, zaten kapalıysa
// yanlış verir. Kapatma (dosya sistemi çağrısı) yorumlayıcı kilidi bırakılarak yapılır; başka
// bir görev o sırada diskten okuyorsa okuması bitince kapanır.
func (o *SatirOkuyucu) kapatMetodu() *interp.BuiltinVal {
	b := &interp.BuiltinVal{Name: codes.Name(codes.Y012), Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() parametre almaz, %d verildi", codes.Name(codes.Y012), len(args))
		}
		var kapatti bool
		m.Blocking(func() {
			o.mu.Lock()
			defer o.mu.Unlock()
			kapatti = !o.bitti
			o.kapatildi = true
			o.kapat()
		})
		return interp.BoolToVal(kapatti), nil
	}}
	b.NoNamedArgs = true
	return b
}

// kapatildiHatasi kapatılmış okuyucudan satır istenince verilen hatadır.
func (o *SatirOkuyucu) kapatildiHatasi() error {
	return fmt.Errorf("%s: %q okuyucusu %s() ile kapatıldı; yeniden okumak için %s(...) ile yeni bir okuyucu açın",
		SatirOkuyucuTipi, o.yol, codes.Name(codes.Y012), codes.Name(codes.G112))
}

// Next sıradaki satırı verir; dosya bitince devam=false. Satır tamponda hazırsa yorumlayıcı
// kilidi bırakılmaz; diskten okumak gerekirse bırakılır (eşzamanlı görevler ilerler).
func (o *SatirOkuyucu) Next(m *interp.VM) (interp.Value, bool, error) {
	if o.mu.TryLock() {
		if o.bitti {
			kapatildi := o.kapatildi
			o.mu.Unlock()
			if kapatildi {
				return nil, false, o.kapatildiHatasi()
			}
			return nil, false, nil
		}
		satir, ok := o.tampondan()
		o.mu.Unlock()
		if ok {
			return &interp.StringVal{Val: satir}, true, nil
		}
	}
	var satir string
	var son, kapatildi bool
	var err error
	m.Blocking(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if kapatildi = o.kapatildi; !kapatildi {
			satir, son, err = o.oku()
		}
	})
	if kapatildi {
		return nil, false, o.kapatildiHatasi()
	}
	if err != nil {
		return nil, false, turkish.FileError(interp.CallPrefix(codes.G112), o.yol, "okunamadı", err)
	}
	if son {
		return nil, false, nil
	}
	return &interp.StringVal{Val: satir}, true, nil
}

// tampondan tamponda tam bir satır varsa onu alır (diske gitmez). o.mu tutulmalıdır.
func (o *SatirOkuyucu) tampondan() (string, bool) {
	n := o.br.Buffered()
	if n == 0 {
		return "", false
	}
	tampon, _ := o.br.Peek(n)
	i := bytes.IndexByte(tampon, '\n')
	if i < 0 {
		return "", false
	}
	satir := o.satirMetni(tampon[:i])
	_, _ = o.br.Discard(i + 1)
	return satir, true
}

// oku diskten okuyarak bir sonraki satırı verir; dosya bitince son=true ve dosya kapanır.
// o.mu tutulmalıdır.
func (o *SatirOkuyucu) oku() (satir string, son bool, err error) {
	if o.bitti {
		return "", true, nil
	}
	var uzun []byte
	for {
		parca, err := o.br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			if len(uzun)+len(parca) > satirSiniri {
				o.kapat()
				return "", false, turkish.Reason(fmt.Sprintf("%d. satır 64 MB'tan uzun; dosya satırlara bölünmemiş olabilir (bütün dosya için %s(...))", o.sayac+1, codes.Name(codes.G110)))
			}
			uzun = append(uzun, parca...)
			continue
		}
		if len(parca) > 0 && parca[len(parca)-1] == '\n' {
			parca = parca[:len(parca)-1]
		}
		if uzun != nil {
			parca = append(uzun, parca...)
		}
		if err == io.EOF {
			if len(parca) == 0 && uzun == nil {
				o.kapat()
				return "", true, nil
			}
			return o.satirMetni(parca), false, nil
		}
		if err != nil {
			o.kapat()
			return "", false, err
		}
		return o.satirMetni(parca), false, nil
	}
}

// satirMetni satır sonundaki \r'yi ve ilk satırın başındaki BOM'u atar, sayacı ilerletir.
func (o *SatirOkuyucu) satirMetni(b []byte) string {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		b = b[:len(b)-1]
	}
	if o.sayac == 0 {
		b = bytes.TrimPrefix(b, bomBaytlari)
	}
	o.sayac++
	return string(b)
}

func (o *SatirOkuyucu) kapat() {
	o.bitti = true
	kayitKapat(o.r)
}

// bomBaytlari UTF-8 bayt sırası işaretidir (bazı programlar metin dosyalarının başına yazar).
var bomBaytlari = []byte{0xEF, 0xBB, 0xBF}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*SatirOkuyucu)(nil)
	_ interp.Stream         = (*SatirOkuyucu)(nil)
)
