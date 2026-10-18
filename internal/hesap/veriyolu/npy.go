// Package veriyolu, dil modeli eğitimini besleyen veri yükleyicinin çekirdeğidir.
//
// Katmanlar (aşağıdan yukarıya):
//
//   - .npy başlık okuyucu/yazıcı (BaslikOku, BaslikCoz, BaslikYaz)
//   - Parca: tek bir .npy dosyasına bellek eşlemeli (ya da ReadAt ile) erişim
//   - Kume: bir dizindeki parçalar ve isteğe bağlı meta.json
//   - Ornekleyici: tohumlu, tekrar üretilebilir rastgele pencere örnekleme
//   - SiraliOkuyucu: karıştırmasız, sıralı doğrulama okuması
//   - OnYukleyici: arka planda sonraki toplu işleri hazırlayan çoklu tampon
//   - Onbellek: yavaş kaynaktaki parçaları yerel diske kopyalayan kotalı katman
//
// Paket yalnızca standart kütüphaneyi kullanır.
package veriyolu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Tur, .npy dosyasındaki elemanların türüdür.
type Tur uint8

const (
	// TurGecersiz tanınmayan türdür.
	TurGecersiz Tur = iota
	// TurU16 küçük uçlu işaretsiz 16 bit ('<u2').
	TurU16
	// TurU32 küçük uçlu işaretsiz 32 bit ('<u4').
	TurU32
	// TurI32 küçük uçlu işaretli 32 bit ('<i4').
	TurI32
)

// Boyut, bir elemanın bayt cinsinden boyutudur.
func (t Tur) Boyut() int {
	switch t {
	case TurU16:
		return 2
	case TurU32, TurI32:
		return 4
	}
	return 0
}

// String, türün .npy 'descr' gösterimidir.
func (t Tur) String() string {
	switch t {
	case TurU16:
		return "<u2"
	case TurU32:
		return "<u4"
	case TurI32:
		return "<i4"
	}
	return "geçersiz"
}

func turCoz(descr string) (Tur, error) {
	switch descr {
	case "<u2":
		return TurU16, nil
	case "<u4":
		return TurU32, nil
	case "<i4":
		return TurI32, nil
	}
	if len(descr) == 3 && descr[0] == '>' {
		return TurGecersiz, fmt.Errorf("büyük uçlu tür %q desteklenmiyor (yalnızca küçük uçlu '<u2', '<u4', '<i4')", descr)
	}
	return TurGecersiz, fmt.Errorf("desteklenmeyen eleman türü %q (yalnızca '<u2', '<u4', '<i4')", descr)
}

// Baslik, çözümlenmiş bir .npy başlığıdır.
type Baslik struct {
	Surum      uint8 // ana sürüm: 1, 2 ya da 3
	AltSurum   uint8
	Tur        Tur
	Uzunluk    int64 // eleman (token) sayısı
	VeriOfseti int64 // verinin dosya başından itibaren bayt konumu
}

// VeriBoyutu, veri bölümünün bayt cinsinden boyutudur.
func (b Baslik) VeriBoyutu() int64 { return b.Uzunluk * int64(b.Tur.Boyut()) }

// BicimHatasi, geçersiz ya da desteklenmeyen bir .npy başlığını bildirir.
type BicimHatasi struct {
	Yol   string // biliniyorsa dosya yolu
	Neden string
}

func (h *BicimHatasi) Error() string {
	if h.Yol != "" {
		return fmt.Sprintf("veri: %s: geçersiz .npy dosyası: %s", h.Yol, h.Neden)
	}
	return "veri: geçersiz .npy dosyası: " + h.Neden
}

func bicimHatasi(bicim string, a ...any) error {
	return &BicimHatasi{Neden: fmt.Sprintf(bicim, a...)}
}

const (
	npyBuyu         = "\x93NUMPY"
	enFazlaBaslik   = 1 << 20 // makul olmayan büyüklükteki başlıkları reddet
	npyHizalama     = 64
	baslikOnEkV1    = 10 // büyü(6) + sürüm(2) + uzunluk(2)
	baslikOnEkV2    = 12 // büyü(6) + sürüm(2) + uzunluk(4)
	enKucukDosyaBas = baslikOnEkV1
)

// BaslikOku, r'nin başından .npy başlığını okur ve çözümler. Yalnızca
// başlık baytları okunur; veri bölümüne dokunulmaz.
func BaslikOku(r io.ReaderAt) (Baslik, error) {
	var on [baslikOnEkV2]byte
	n, err := r.ReadAt(on[:], 0)
	if n < enKucukDosyaBas {
		if err == nil || errors.Is(err, io.EOF) {
			return Baslik{}, bicimHatasi("dosya çok kısa (%d bayt), .npy başlığı yok", n)
		}
		return Baslik{}, fmt.Errorf("veri: başlık okunamadı: %w", err)
	}
	ana := on[6]
	var bl, onEk int
	switch ana {
	case 1:
		bl, onEk = int(binary.LittleEndian.Uint16(on[8:10])), baslikOnEkV1
	case 2, 3:
		if n < baslikOnEkV2 {
			return Baslik{}, bicimHatasi("dosya çok kısa, sürüm %d başlığı eksik", ana)
		}
		bl, onEk = int(binary.LittleEndian.Uint32(on[8:12])), baslikOnEkV2
	default:
		// Önce büyü baytlarını denetle ki yanlış dosyada daha anlamlı hata verelim.
		if string(on[:6]) != npyBuyu {
			return Baslik{}, bicimHatasi("büyü baytları eksik (dosya .npy değil)")
		}
		return Baslik{}, bicimHatasi("desteklenmeyen sürüm %d.%d (yalnızca 1.x, 2.x, 3.x)", ana, on[7])
	}
	if bl > enFazlaBaslik {
		return Baslik{}, bicimHatasi("başlık uzunluğu çok büyük (%d bayt)", bl)
	}
	tam := make([]byte, onEk+bl)
	copy(tam, on[:n])
	if onEk+bl > n {
		m, err := r.ReadAt(tam[n:], int64(n))
		if m < len(tam)-n {
			if err == nil || errors.Is(err, io.EOF) {
				return Baslik{}, bicimHatasi("başlık kesik: %d bayt beklendi", onEk+bl)
			}
			return Baslik{}, fmt.Errorf("veri: başlık okunamadı: %w", err)
		}
	}
	return BaslikCoz(tam)
}

// BaslikCoz, bellekteki baytlardan (en az tüm başlık) .npy başlığını çözümler.
func BaslikCoz(b []byte) (Baslik, error) {
	if len(b) < enKucukDosyaBas {
		return Baslik{}, bicimHatasi("başlık çok kısa (%d bayt)", len(b))
	}
	if string(b[:6]) != npyBuyu {
		return Baslik{}, bicimHatasi("büyü baytları eksik (dosya .npy değil)")
	}
	var bs Baslik
	bs.Surum, bs.AltSurum = b[6], b[7]
	var bl, onEk int
	switch bs.Surum {
	case 1:
		bl, onEk = int(binary.LittleEndian.Uint16(b[8:10])), baslikOnEkV1
	case 2, 3:
		if len(b) < baslikOnEkV2 {
			return Baslik{}, bicimHatasi("sürüm %d başlığı eksik", bs.Surum)
		}
		bl, onEk = int(binary.LittleEndian.Uint32(b[8:12])), baslikOnEkV2
	default:
		return Baslik{}, bicimHatasi("desteklenmeyen sürüm %d.%d (yalnızca 1.x, 2.x, 3.x)", bs.Surum, bs.AltSurum)
	}
	if bl > enFazlaBaslik {
		return Baslik{}, bicimHatasi("başlık uzunluğu çok büyük (%d bayt)", bl)
	}
	if len(b) < onEk+bl {
		return Baslik{}, bicimHatasi("başlık kesik: %d bayt beklendi, %d var", onEk+bl, len(b))
	}
	metin := b[onEk : onEk+bl]
	if bs.Surum < 3 {
		for _, c := range metin {
			if c >= 0x80 {
				return Baslik{}, bicimHatasi("sürüm %d başlığında ASCII dışı bayt", bs.Surum)
			}
		}
	}
	sozluk, err := sozlukCoz(string(metin))
	if err != nil {
		return Baslik{}, bicimHatasi("başlık sözlüğü çözümlenemedi: %v", err)
	}
	if err := baslikDoldur(&bs, sozluk); err != nil {
		return Baslik{}, err
	}
	bs.VeriOfseti = int64(onEk + bl)
	return bs, nil
}

func baslikDoldur(bs *Baslik, s map[string]any) error {
	for k := range s {
		switch k {
		case "descr", "fortran_order", "shape":
		default:
			return bicimHatasi("başlıkta beklenmeyen anahtar %q", k)
		}
	}
	d, ok := s["descr"]
	if !ok {
		return bicimHatasi("başlıkta 'descr' anahtarı yok")
	}
	ds, ok := d.(string)
	if !ok {
		return bicimHatasi("'descr' bir metin değil (yapılandırılmış türler desteklenmiyor)")
	}
	tur, err := turCoz(ds)
	if err != nil {
		return bicimHatasi("%v", err)
	}
	bs.Tur = tur

	f, ok := s["fortran_order"]
	if !ok {
		return bicimHatasi("başlıkta 'fortran_order' anahtarı yok")
	}
	fb, ok := f.(bool)
	if !ok {
		return bicimHatasi("'fortran_order' True/False değil")
	}
	if fb {
		return bicimHatasi("Fortran sıralı diziler desteklenmiyor (yalnızca C sıralı)")
	}

	sh, ok := s["shape"]
	if !ok {
		return bicimHatasi("başlıkta 'shape' anahtarı yok")
	}
	boyutlar, ok := sh.([]int64)
	if !ok {
		return bicimHatasi("'shape' bir tamsayı demeti değil")
	}
	if len(boyutlar) != 1 {
		return bicimHatasi("yalnızca tek boyutlu diziler destekleniyor, şekil %d boyutlu", len(boyutlar))
	}
	if boyutlar[0] < 0 {
		return bicimHatasi("negatif uzunluk %d", boyutlar[0])
	}
	if boyutlar[0] > (1<<62)/int64(tur.Boyut()) {
		return bicimHatasi("uzunluk çok büyük (%d)", boyutlar[0])
	}
	bs.Uzunluk = boyutlar[0]
	return nil
}

// ---------------------------------------------------------------------------
// Başlık sözlüğü için küçük bir sözdizimi çözümleyici. Biçim, tırnaklı
// anahtarlı bir sözlük değişmezidir:
// {'descr': '<u2', 'fortran_order': False, 'shape': (N,), }
// ---------------------------------------------------------------------------

type sozlukAyristirici struct {
	s string
	i int
}

func sozlukCoz(s string) (map[string]any, error) {
	a := &sozlukAyristirici{s: s}
	a.bosluk()
	if !a.al('{') {
		return nil, fmt.Errorf("'{' bekleniyordu")
	}
	sonuc := map[string]any{}
	for {
		a.bosluk()
		if a.al('}') {
			break
		}
		a.bosluk()
		anahtar, err := a.metin()
		if err != nil {
			return nil, fmt.Errorf("anahtar: %w", err)
		}
		a.bosluk()
		if !a.al(':') {
			return nil, fmt.Errorf("%q anahtarından sonra ':' bekleniyordu", anahtar)
		}
		a.bosluk()
		d, err := a.deger()
		if err != nil {
			return nil, fmt.Errorf("%q değeri: %w", anahtar, err)
		}
		if _, var_ := sonuc[anahtar]; var_ {
			return nil, fmt.Errorf("%q anahtarı iki kez geçiyor", anahtar)
		}
		sonuc[anahtar] = d
		a.bosluk()
		if a.al(',') {
			continue
		}
		a.bosluk()
		if a.al('}') {
			break
		}
		return nil, fmt.Errorf("konum %d: ',' ya da '}' bekleniyordu", a.i)
	}
	a.bosluk()
	if a.i != len(a.s) {
		return nil, fmt.Errorf("sözlükten sonra fazladan karakter")
	}
	return sonuc, nil
}

func (a *sozlukAyristirici) bosluk() {
	for a.i < len(a.s) {
		switch a.s[a.i] {
		case ' ', '\t', '\n', '\r', '\x00':
			a.i++
		default:
			return
		}
	}
}

func (a *sozlukAyristirici) al(c byte) bool {
	if a.i < len(a.s) && a.s[a.i] == c {
		a.i++
		return true
	}
	return false
}

func (a *sozlukAyristirici) metin() (string, error) {
	if a.i >= len(a.s) {
		return "", fmt.Errorf("metin bekleniyordu, başlık bitti")
	}
	tirnak := a.s[a.i]
	if tirnak != '\'' && tirnak != '"' {
		return "", fmt.Errorf("konum %d: tırnaklı metin bekleniyordu", a.i)
	}
	j := strings.IndexByte(a.s[a.i+1:], tirnak)
	if j < 0 {
		return "", fmt.Errorf("kapanmayan tırnak")
	}
	m := a.s[a.i+1 : a.i+1+j]
	if strings.IndexByte(m, '\\') >= 0 {
		return "", fmt.Errorf("kaçış dizili metin desteklenmiyor")
	}
	a.i += j + 2
	return m, nil
}

func (a *sozlukAyristirici) deger() (any, error) {
	if a.i >= len(a.s) {
		return nil, fmt.Errorf("değer bekleniyordu, başlık bitti")
	}
	switch c := a.s[a.i]; {
	case c == '\'' || c == '"':
		return a.metin()
	case c == '(':
		a.i++
		var demet []int64
		for {
			a.bosluk()
			if a.al(')') {
				return demet, nil
			}
			n, err := a.tamsayi()
			if err != nil {
				return nil, err
			}
			demet = append(demet, n)
			a.bosluk()
			if a.al(',') {
				continue
			}
			if a.al(')') {
				return demet, nil
			}
			return nil, fmt.Errorf("demette ',' ya da ')' bekleniyordu")
		}
	case c == '[' || c == '{':
		return nil, fmt.Errorf("iç içe liste/sözlük desteklenmiyor")
	case c >= '0' && c <= '9' || c == '-':
		return a.tamsayi()
	default:
		j := a.i
		for j < len(a.s) && (a.s[j] >= 'A' && a.s[j] <= 'Z' || a.s[j] >= 'a' && a.s[j] <= 'z') {
			j++
		}
		ad := a.s[a.i:j]
		a.i = j
		switch ad {
		case "True":
			return true, nil
		case "False":
			return false, nil
		case "None":
			return nil, nil
		}
		return nil, fmt.Errorf("tanınmayan değer %q", ad)
	}
}

func (a *sozlukAyristirici) tamsayi() (int64, error) {
	j := a.i
	if j < len(a.s) && a.s[j] == '-' {
		j++
	}
	for j < len(a.s) && a.s[j] >= '0' && a.s[j] <= '9' {
		j++
	}
	n, err := strconv.ParseInt(a.s[a.i:j], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("geçersiz tamsayı %q", a.s[a.i:j])
	}
	a.i = j
	// Eski yazıcıların uzun tamsayı soneki.
	if a.i < len(a.s) && (a.s[a.i] == 'L' || a.s[a.i] == 'l') {
		a.i++
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Yazma
// ---------------------------------------------------------------------------

// BaslikYaz, uzunluk elemanlı tek boyutlu bir dizi için .npy başlığını w'ye
// yazar. surum 1 ya da 2 olmalıdır. Başlık toplamı 64 bayta hizalanır.
func BaslikYaz(w io.Writer, tur Tur, uzunluk int64, surum int) error {
	if tur.Boyut() == 0 {
		return fmt.Errorf("veri: geçersiz tür")
	}
	if uzunluk < 0 {
		return fmt.Errorf("veri: negatif uzunluk")
	}
	sozluk := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': (%d,), }", tur, uzunluk)
	var onEk int
	switch surum {
	case 1:
		onEk = baslikOnEkV1
	case 2:
		onEk = baslikOnEkV2
	default:
		return fmt.Errorf("veri: yazma için desteklenmeyen sürüm %d", surum)
	}
	toplam := onEk + len(sozluk) + 1
	dolgu := (npyHizalama - toplam%npyHizalama) % npyHizalama
	bl := len(sozluk) + dolgu + 1
	if surum == 1 && bl > 0xFFFF {
		return fmt.Errorf("veri: başlık sürüm 1 için çok uzun")
	}
	var buf bytes.Buffer
	buf.WriteString(npyBuyu)
	buf.WriteByte(byte(surum))
	buf.WriteByte(0)
	if surum == 1 {
		_ = binary.Write(&buf, binary.LittleEndian, uint16(bl))
	} else {
		_ = binary.Write(&buf, binary.LittleEndian, uint32(bl))
	}
	buf.WriteString(sozluk)
	buf.WriteString(strings.Repeat(" ", dolgu))
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// NpyYazU16, veriyi '<u2' türünde sürüm 1 .npy olarak w'ye yazar.
func NpyYazU16(w io.Writer, veri []uint16) error {
	if err := BaslikYaz(w, TurU16, int64(len(veri)), 1); err != nil {
		return err
	}
	b := make([]byte, 2*len(veri))
	for i, v := range veri {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	_, err := w.Write(b)
	return err
}

// NpyYazI32, veriyi '<i4' türünde sürüm 1 .npy olarak w'ye yazar.
func NpyYazI32(w io.Writer, veri []int32) error {
	if err := BaslikYaz(w, TurI32, int64(len(veri)), 1); err != nil {
		return err
	}
	b := make([]byte, 4*len(veri))
	for i, v := range veri {
		binary.LittleEndian.PutUint32(b[4*i:], uint32(v))
	}
	_, err := w.Write(b)
	return err
}
