package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
)

// 'tr derle' ile üretilen uygulama, yorumlayıcı ikilisinin sonuna eklenmiş bir paketten
// oluşur. İki biçim vardır:
//
//	Sürüm 1 (eski, yalnızca okunur): [kaynak kod][kod uzunluğu u64][eskiPaketImzasi]
//	Sürüm 2 ve sonrası:            [JSON paket][paket uzunluğu u64][sürüm u32][paketImzasi]
//
// Tüm sayılar küçük uçludur (little endian). Sürüm 2 paketinde ana programın yanında
// onun 'aktar' ile aldığı .tr modülleri de (özyinelemeli olarak) bulunur.
const (
	eskiPaketImzasi        = "TR_EXE_1"
	paketImzasi            = "TR_PAKET"
	paketSurumu     uint32 = 2

	eskiAltbilgiBoyu = 16 // uzunluk + imza
	altbilgiBoyu     = 20 // uzunluk + sürüm + imza
)

// gomuluKaynakAdi gömülü uygulamanın ana programına verilen addır. VM bu adı görünce
// modülleri ana programın klasörüne göre aramaz; gömülü uygulamada her 'aktar' zaten
// gömülü modülün mutlak yoluna yönlendirilir.
const gomuluKaynakAdi = "<derlenmiş-uygulama>"

// paketDosyasi pakete gömülü tek bir .tr kaynağıdır.
type paketDosyasi struct {
	// Ad derleme sırasında dosyanın çözüldüğü yoldur (yalnızca bilgi ve geçici dosya adı için).
	Ad  string `json:"ad"`
	Kod string `json:"kod"`
	// Aktarimlar dosyadaki 'aktar' deyimlerinin, kaynaktaki sırayla, hangi gömülü modüle
	// (Moduller dizisindeki sıra) gittiğini söyler.
	Aktarimlar []int `json:"aktarimlar,omitempty"`
}

// paket gömülü uygulamanın tamamıdır.
type paket struct {
	Ana      paketDosyasi   `json:"ana"`
	Moduller []paketDosyasi `json:"moduller,omitempty"`
}

// paketiKodla paketi altbilgisiyle birlikte ikilinin sonuna eklenecek baytlara çevirir.
func paketiKodla(pk *paket) ([]byte, error) {
	veri, err := json.Marshal(pk)
	if err != nil {
		return nil, err
	}
	altbilgi := make([]byte, altbilgiBoyu)
	binary.LittleEndian.PutUint64(altbilgi[0:8], uint64(len(veri)))
	binary.LittleEndian.PutUint32(altbilgi[8:12], paketSurumu)
	copy(altbilgi[12:20], paketImzasi)
	return append(veri, altbilgi...), nil
}

// paketiAyikla ikilinin sonundaki paketi çözer. bulundu, sonda bir paket imzası olup
// olmadığını; sablonBoyu, paket atıldığında geriye kalan yorumlayıcının bayt sayısını
// verir. İmza var ama paket bozuksa ya da desteklenmeyen bir sürümse hata döner.
func paketiAyikla(ikili []byte) (pk *paket, sablonBoyu int, bulundu bool, err error) {
	n := len(ikili)
	switch {
	case n >= altbilgiBoyu && string(ikili[n-8:]) == paketImzasi:
		uzunluk := binary.LittleEndian.Uint64(ikili[n-20 : n-12])
		surum := binary.LittleEndian.Uint32(ikili[n-12 : n-8])
		if uzunluk > uint64(n-altbilgiBoyu) {
			return nil, 0, true, errors.New("gömülü paketin uzunluk alanı bozuk")
		}
		bas := n - altbilgiBoyu - int(uzunluk)
		if surum < 2 || surum > paketSurumu {
			return nil, bas, true, fmt.Errorf("gömülü paket sürümü %d desteklenmiyor (bu yorumlayıcı en çok %d sürümünü okur; uygulamayı daha yeni bir 'tr' ile derlenmiş olabilir)", surum, paketSurumu)
		}
		pk = &paket{}
		if err := json.Unmarshal(ikili[bas:n-altbilgiBoyu], pk); err != nil {
			return nil, bas, true, fmt.Errorf("gömülü paket çözülemedi: %v", err)
		}
		return pk, bas, true, nil
	case n >= eskiAltbilgiBoyu && string(ikili[n-8:]) == eskiPaketImzasi:
		// Sürüm 1: modülsüz, yalnızca ana programın kaynağı.
		uzunluk := binary.LittleEndian.Uint64(ikili[n-16 : n-8])
		if uzunluk == 0 || uzunluk > uint64(n-eskiAltbilgiBoyu) {
			return nil, 0, true, errors.New("gömülü kodun uzunluk alanı bozuk")
		}
		bas := n - eskiAltbilgiBoyu - int(uzunluk)
		kod := string(ikili[bas : n-eskiAltbilgiBoyu])
		return &paket{Ana: paketDosyasi{Kod: kod}}, bas, true, nil
	}
	return nil, n, false, nil
}

// gomuluPaketiBul çalışan ikilinin sonunda bir uygulama paketi arar.
func gomuluPaketiBul() (*paket, bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, false, nil
	}
	f, err := os.Open(exePath)
	if err != nil {
		return nil, false, nil
	}
	defer f.Close()
	bilgi, err := f.Stat()
	if err != nil {
		return nil, false, nil
	}
	// Önce yalnızca son 8 bayta (imza) bakılır.
	boy := bilgi.Size()
	if boy < eskiAltbilgiBoyu {
		return nil, false, nil
	}
	imza := make([]byte, 8)
	if _, err := f.ReadAt(imza, boy-8); err != nil {
		return nil, false, nil
	}
	// İmza varsa yalnızca altbilgi ve paket okunur (yorumlayıcının kendisi değil).
	var altbilgi int64
	switch string(imza) {
	case paketImzasi:
		altbilgi = altbilgiBoyu
	case eskiPaketImzasi:
		altbilgi = eskiAltbilgiBoyu
	default:
		return nil, false, nil
	}
	if boy < altbilgi {
		return nil, true, errors.New("gömülü paketin altbilgisi eksik")
	}
	uzunlukBaytlari := make([]byte, 8)
	if _, err := f.ReadAt(uzunlukBaytlari, boy-altbilgi); err != nil {
		return nil, true, err
	}
	uzunluk := binary.LittleEndian.Uint64(uzunlukBaytlari)
	if uzunluk > uint64(boy-altbilgi) {
		return nil, true, errors.New("gömülü paketin uzunluk alanı bozuk")
	}
	kuyruk := make([]byte, int64(uzunluk)+altbilgi)
	if _, err := f.ReadAt(kuyruk, boy-int64(len(kuyruk))); err != nil {
		return nil, true, err
	}
	pk, _, bulundu, err := paketiAyikla(kuyruk)
	return pk, bulundu, err
}

// aktarYeri bir 'aktar' deyiminin modül yolunu yazan sözcük biriminin kaynaktaki
// karakter (rune) aralığıdır.
type aktarYeri struct {
	bas, son int
	yol      string
	satir    int
}

// aktarYerleri koddaki tüm 'aktar' deyimlerini kaynaktaki sırayla bulur. Ayrıştırıcı da
// 'aktar' sözcüğünden sonraki ilk sözcük biriminin metnini modül yolu sayar; burada da
// aynısı yapılır, böylece iç bloklardaki aktarımlar da kaçmaz. Aralıklar kaynağın gövdesine
// göredir (source.Parts; kodlu dosyada başlıktan sonrası).
func aktarYerleri(kod string) []aktarYeri {
	b, _ := source.Split(kod)
	l := b.Lexer()
	var yerler []aktarYeri
	for {
		tok := l.NextToken()
		if tok.Type == lexer.TOKEN_EOF {
			return yerler
		}
		if tok.Type != lexer.TOKEN_IMPORT {
			continue
		}
		yolTok := l.NextToken()
		if yolTok.Type == lexer.TOKEN_EOF {
			return yerler
		}
		bas, son := l.LastRange()
		yerler = append(yerler, aktarYeri{bas: bas, son: son, yol: yolTok.Literal, satir: tok.Line})
	}
}

// modulYolunuCoz bir 'aktar' yolunu yorumlayıcıyla aynı kuralla (yurut.ModulYolunuCoz) çözer:
// önce aktaran dosyanın klasörüne, bulunamazsa ana programın klasörüne göre (11.1). aranan,
// hata iletisinde gösterilecek denenmiş yollardır. Dilin kitaplığındaki (tr/ önekli) modüller
// diskte aranmaz; onları çağıran yurut.KitaplikModulu ile ayırır.
func modulYolunuCoz(yol, aktaran, ana string) (cozulen string, aranan []string, ok bool) {
	if !strings.HasSuffix(yol, ".tr") {
		yol += ".tr"
	}
	return interp.ResolveModulePath(yol, interp.FileDirs(aktaran, ana))
}

// kitaplikAktarimi pakette dilin gömülü kitaplığına (11.6) giden aktarımın işaretidir: bu
// modüller pakete gömülmez (uygulama zaten kitaplığı içeren yorumlayıcının kopyasıdır) ve
// yolları yönlendirilmez.
const kitaplikAktarimi = -1

// modulToplayici derleme sırasında programın aktardığı modülleri özyinelemeli toplar.
type modulToplayici struct {
	sira     map[string]int // mutlak yol → Moduller içindeki sıra
	moduller []paketDosyasi
	ana      string // ana programın yolu
	// genelAdlar ana programın genel kapsama tanımladığı adlardır: modüller genel kapsamın
	// altında çalışır ve bu adları görür (11.1). genelDinamik, ana programda takma adsız
	// 'aktar' olduğu için adların tam bilinmediğini söyler.
	genelAdlar   []string
	genelDinamik bool
}

// topla dosyanın 'aktar' deyimlerini çözer, henüz gömülmemiş modülleri okuyup pakete
// ekler ve dosyanın Aktarimlar alanını doldurur. kaynakYolu, dosyanın yorumlayıcıda
// alacağı SourceFile değeridir (modül yolları buna göre çözülür).
func (t *modulToplayici) topla(dosya *paketDosyasi, kaynakYolu string) error {
	yerler := aktarYerleri(dosya.Kod)
	dosya.Aktarimlar = make([]int, 0, len(yerler))
	for _, y := range yerler {
		if ad, ok := interp.LibraryModule(y.yol); ok {
			if _, err := interp.ReadLibrary(ad); err != nil {
				return fmt.Errorf("%s, satır %d: %v", kaynakYolu, y.satir, err)
			}
			dosya.Aktarimlar = append(dosya.Aktarimlar, kitaplikAktarimi)
			continue
		}
		cozulen, aranan, ok := modulYolunuCoz(y.yol, kaynakYolu, t.ana)
		if !ok {
			return fmt.Errorf("%s, satır %d: %q modülü bulunamadı (aranan: %s)", kaynakYolu, y.satir, y.yol, strings.Join(aranan, ", "))
		}
		mutlak, err := filepath.Abs(cozulen)
		if err != nil {
			mutlak = cozulen
		}
		if i, varMi := t.sira[mutlak]; varMi {
			dosya.Aktarimlar = append(dosya.Aktarimlar, i)
			continue
		}
		veri, err := os.ReadFile(cozulen)
		if err != nil {
			return fmt.Errorf("%s, satır %d: %q modülü okunamadı: %v", kaynakYolu, y.satir, y.yol, err)
		}
		modul := paketDosyasi{Ad: filepath.ToSlash(cozulen), Kod: string(veri)}
		// Modül yorumlayıcıdaki 'aktar' ile aynı denetimden geçer (sözdizimi, üst düzey
		// 'sonuç', tip denetimi).
		if s := interp.CheckModule(modul.Kod, t.genelAdlar, t.genelDinamik); s.Stage != "" {
			return &modulDenetimHatasi{yol: filepath.ToSlash(cozulen), kod: s.Text, tur: s.Stage, hatalar: s.Errors}
		}
		// Sıra özyinelemeden önce ayrılır; döngüsel aktarımlar böylece aynı sırayı bulur
		// (döngünün kendisini yorumlayıcı çalışma anında bildirir).
		i := len(t.moduller)
		t.sira[mutlak] = i
		t.moduller = append(t.moduller, paketDosyasi{})
		if err := t.topla(&modul, cozulen); err != nil {
			return err
		}
		t.moduller[i] = modul
		dosya.Aktarimlar = append(dosya.Aktarimlar, i)
	}
	return nil
}

// modulDenetimHatasi bir modülde sözdizimi ya da tip denetimi hatası bulunduğunu bildirir.
type modulDenetimHatasi struct {
	yol     string
	kod     string
	tur     string // "sözdizimi" ya da "tip denetimi"
	hatalar []string
}

func (e *modulDenetimHatasi) Error() string {
	return fmt.Sprintf("%s modülünde %s hatası: %s", e.yol, e.tur, e.hatalar[0])
}

// paketHazirla ana programı ve aktardığı tüm modülleri içeren paketi kurar.
func paketHazirla(srcPath, kod string) (*paket, error) {
	t := &modulToplayici{sira: map[string]int{}, ana: srcPath}
	t.genelAdlar, t.genelDinamik = checker.TopLevelNames(source.Prepare(kod, source.Options{}).Program)
	pk := &paket{Ana: paketDosyasi{Ad: filepath.ToSlash(srcPath), Kod: kod}}
	if err := t.topla(&pk.Ana, srcPath); err != nil {
		return nil, err
	}
	pk.Moduller = t.moduller
	return pk, nil
}

// islemciNotu, yorumlayıcı belirli bir x86-64 düzeyi için derlenmişse (GOAMD64=v3/v4)
// üretilen uygulamanın da aynı işlemci özelliklerini isteyeceğini söyleyen satırı verir.
func islemciNotu() string {
	bilgi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var mimari, duzey string
	for _, s := range bilgi.Settings {
		switch s.Key {
		case "GOARCH":
			mimari = s.Value
		case "GOAMD64":
			duzey = s.Value
		}
	}
	return islemciNotuMetni(mimari, duzey)
}

// islemciNotuMetni islemciNotu'nun derleme ayarlarından bağımsız (sınanabilir) bölümüdür.
func islemciNotuMetni(mimari, duzey string) string {
	if mimari != "amd64" {
		return ""
	}
	switch duzey {
	case "v3":
		return "⚠️  Not: Yorumlayıcı GOAMD64=v3 ile derlendiği için bu uygulama AVX2 destekli (x86-64-v3) bir işlemci ister."
	case "v4":
		return "⚠️  Not: Yorumlayıcı GOAMD64=v4 ile derlendiği için bu uygulama AVX-512 destekli (x86-64-v4) bir işlemci ister."
	}
	return ""
}

// compileToExe 'tr derle' komutudur: kaynağı ve aktardığı modülleri denetleyip çalışan
// yorumlayıcının bir kopyasının sonuna paket olarak ekler.
func compileToExe(srcPath string, outPath string) {
	code, _ := dosyaHazirla(srcPath, derlemeHazirligi)
	pk := modulleriDenetle(srcPath, code, derlemeHazirligi, "❌ Derleme hatası:")

	if outPath == "" {
		base := strings.TrimSuffix(srcPath, filepath.Ext(srcPath))
		outPath = base + ".exe"
	}
	if err := ciktiYoluDenetle(srcPath, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Derleme hatası: %v\n", err)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Derleyici ikili dosyası okunamadı: %v\n", err)
		os.Exit(1)
	}
	baseExeBytes, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Derleyici şablonu okunamadı: %v\n", err)
		os.Exit(1)
	}
	// Şablonun sonunda zaten bir paket varsa (her iki biçim) atılır.
	if _, sablonBoyu, bulundu, _ := paketiAyikla(baseExeBytes); bulundu && sablonBoyu > 0 {
		baseExeBytes = baseExeBytes[:sablonBoyu]
	}

	paketBaytlari, err := paketiKodla(pk)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Paket oluşturma hatası: %v\n", err)
		os.Exit(1)
	}
	outBuf := make([]byte, 0, len(baseExeBytes)+len(paketBaytlari))
	outBuf = append(outBuf, baseExeBytes...)
	outBuf = append(outBuf, paketBaytlari...)

	// Önce geçici dosyaya yazılır, sonra yeniden adlandırılır: yazma yarıda kalırsa var
	// olan bir dosya bozulmaz.
	gecici := outPath + ".yaziliyor"
	if err := os.WriteFile(gecici, outBuf, 0755); err != nil {
		os.Remove(gecici)
		fmt.Fprintf(os.Stderr, "❌ .exe oluşturma hatası: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(gecici, outPath); err != nil {
		os.Remove(gecici)
		fmt.Fprintf(os.Stderr, "❌ .exe oluşturma hatası: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 Başarıyla derlendi! Bağımsız ve taşınabilir .exe hazır: %s\n", outPath)
	if len(pk.Moduller) > 0 {
		adlar := make([]string, len(pk.Moduller))
		for i, m := range pk.Moduller {
			adlar[i] = m.Ad
		}
		fmt.Printf("📦 Gömülen modüller (%d): %s\n", len(adlar), strings.Join(adlar, ", "))
	}
	if not := islemciNotu(); not != "" {
		fmt.Println(not)
	}
}

// ciktiYoluDenetle 'tr derle'nin çıktı yolunun bir kaynak dosyayı ezmeyeceğini denetler:
// çıktı kaynağın kendisi (aynı dosya) ya da .tr uzantılı bir dosya olamaz.
func ciktiYoluDenetle(srcPath, outPath string) error {
	if strings.EqualFold(filepath.Ext(outPath), ".tr") {
		return fmt.Errorf("çıktı yolu %q bir .tr kaynak dosyası; uygulama .tr dosyasının üzerine yazılmaz (ör. %s yazın)", outPath, strings.TrimSuffix(outPath, filepath.Ext(outPath))+".exe")
	}
	ayni := false
	if a, err := filepath.Abs(srcPath); err == nil {
		if b, err := filepath.Abs(outPath); err == nil && strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
			ayni = true
		}
	}
	if si, err := os.Stat(srcPath); err == nil {
		if oi, err := os.Stat(outPath); err == nil && os.SameFile(si, oi) {
			ayni = true
		}
	}
	if ayni {
		return fmt.Errorf("çıktı yolu %q kaynak dosyanın kendisi; uygulama kaynağın üzerine yazılmaz", outPath)
	}
	return nil
}

// metinSabiti bir yolu TR metin sabiti olarak yazar.
func metinSabiti(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// aktarlariYonlendir dosyadaki her 'aktar' yolunu, gömülü modülün geçici klasöre yazılmış
// kopyasının mutlak yoluyla değiştirir. Değişiklik satır içinde kaldığı için satır
// numaraları (hata iletileri) aynı kalır. Mutlak yol, yorumlayıcının bulunulan klasördeki
// aynı adlı bir dosyayı gömülü modülün önüne koymasını da engeller.
func aktarlariYonlendir(d paketDosyasi, yollar []string) (string, error) {
	yerler := aktarYerleri(d.Kod)
	if len(yerler) != len(d.Aktarimlar) {
		return "", fmt.Errorf("gömülü paket bozuk: %q dosyasında %d aktarım var, pakette %d kayıtlı", d.Ad, len(yerler), len(d.Aktarimlar))
	}
	if len(yerler) == 0 {
		return d.Kod, nil
	}
	b, _ := source.Split(d.Kod)
	girdi := []rune(b.Body)
	var sb strings.Builder
	onceki := 0
	for i, y := range yerler {
		hedef := d.Aktarimlar[i]
		if hedef == kitaplikAktarimi {
			continue // dilin kitaplığı: yol olduğu gibi kalır
		}
		if hedef < 0 || hedef >= len(yollar) {
			return "", fmt.Errorf("gömülü paket bozuk: %q dosyasında geçersiz modül sırası %d", d.Ad, hedef)
		}
		sb.WriteString(string(girdi[onceki:y.bas]))
		sb.WriteString(metinSabiti(filepath.ToSlash(yollar[hedef])))
		onceki = y.son
	}
	sb.WriteString(string(girdi[onceki:]))
	b.Body = sb.String()
	return b.Join(), nil
}

// gomuluModulleriYaz modülleri geçici klasöre (aktarımları yönlendirilmiş olarak) yazar
// ve her modülün yolunu döndürür.
func gomuluModulleriYaz(pk *paket, klasor string) ([]string, error) {
	yollar := make([]string, len(pk.Moduller))
	for i, m := range pk.Moduller {
		ad := filepath.Base(filepath.FromSlash(m.Ad))
		if ad == "." || ad == string(filepath.Separator) || ad == "" {
			ad = "modul.tr"
		}
		yollar[i] = filepath.Join(klasor, strconv.Itoa(i), ad)
	}
	for i, m := range pk.Moduller {
		kod, err := aktarlariYonlendir(m, yollar)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(yollar[i]), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(yollar[i], []byte(kod), 0o600); err != nil {
			return nil, err
		}
	}
	return yollar, nil
}

// gomuluUygulamayiCalistir 'tr derle' ile üretilmiş uygulamanın paketini çalıştırır.
// Modüller özel bir geçici klasöre açılır ve program bitince (hata ile bitse de) silinir.
func gomuluUygulamayiCalistir(pk *paket) {
	kod := pk.Ana.Kod
	if len(pk.Moduller) > 0 {
		klasor, err := os.MkdirTemp("", "tr-uygulama-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Gömülü modüller açılamadı: %v\n", err)
			os.Exit(1)
		}
		cikisTemizligi = append(cikisTemizligi, func() { os.RemoveAll(klasor) })
		yollar, err := gomuluModulleriYaz(pk, klasor)
		if err == nil {
			kod, err = aktarlariYonlendir(pk.Ana, yollar)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Gömülü modüller açılamadı: %v\n", err)
			cik(1)
		}
	} else if slices.ContainsFunc(pk.Ana.Aktarimlar, func(i int) bool { return i != kitaplikAktarimi }) {
		fmt.Fprintln(os.Stderr, "❌ Gömülü paket bozuk: aktarılan modüller pakette yok")
		os.Exit(1)
	}

	s := kaynakHazirla(kod, gomuluHazirlik)
	machine := interp.New()
	machine.SourceFile = gomuluKaynakAdi
	machine.CommandName = filepath.Base(os.Args[0])
	// Hata çerçeveleri kullanıcının yazdığı (yönlendirilmemiş) kaynağı göstersin; satırlar aynıdır.
	ozgun := source.Prepare(pk.Ana.Kod, source.Options{}).Text
	machine.SourceCode = ozgun
	// Üretilen uygulamanın bütün argümanları programındır (uygulama.exe --adım 5).
	machine.SetArgs(os.Args[1:])
	programCalistir(machine, s.Program, ozgun)
	temizle()
}

// cikisTemizligi çıkmadan önce çalışacak işlerdir (ör. gömülü modüllerin geçici klasörü).
var cikisTemizligi []func()

// temizle bekleyen temizlik işlerini bir kez çalıştırır.
func temizle() {
	for i := len(cikisTemizligi) - 1; i >= 0; i-- {
		cikisTemizligi[i]()
	}
	cikisTemizligi = nil
}

// cikisKodu cik'e verilen koddur; temizlik işleri (ör. tr profil'in son raporu) okur.
var cikisKodu int

// cik temizlik işlerini yapıp verilen kodla çıkar.
func cik(kod int) {
	cikisKodu = kod
	temizle()
	os.Exit(kod)
}
