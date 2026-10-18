// Package testkosucu 'tr test' komutunun çekirdeğidir: .tr dosyalarındaki
// test "ad" { ... } bloklarını bulur, her birini yalıtılmış bir kapsamda çalıştırır
// ve sonuçları Türkçe olarak raporlar.
//
// Katman: araç (internal/arac); dil, motor ve hesap katmanlarını içe aktarabilir.
package testkosucu

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "tr_dili/internal/arac/ayikla" // koddaki kesme_noktası hata ayıklayıcıyı açar
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	_ "tr_dili/internal/motor" // testler dilin tamamıyla (motor yerleşikleri dahil) çalışır
)

// Ozet bir test çalıştırmasının sayısal sonucudur.
type Ozet struct {
	Toplam     int // çalıştırılan test sayısı
	Gecen      int
	Kalan      int
	Yuklenemez int // sözdizimi/tip/üst düzey hata nedeniyle testleri çalıştırılamayan dosya sayısı
	Dosya      int // incelenen dosya sayısı
}

// Basarili hiç test kalmadıysa ve tüm dosyalar yüklenebildiyse doğrudur.
func (o Ozet) Basarili() bool { return o.Kalan == 0 && o.Yuklenemez == 0 }

func (o *Ozet) ekle(b Ozet) {
	o.Toplam += b.Toplam
	o.Gecen += b.Gecen
	o.Kalan += b.Kalan
	o.Yuklenemez += b.Yuklenemez
	o.Dosya += b.Dosya
}

// String özet satırını üretir: "12 test, 11 geçti, 1 kaldı".
func (o Ozet) String() string {
	s := fmt.Sprintf("%d test, %d geçti, %d kaldı", o.Toplam, o.Gecen, o.Kalan)
	if o.Yuklenemez > 0 {
		s += fmt.Sprintf(", %d dosya yüklenemedi", o.Yuklenemez)
	}
	return s
}

// TestDosyalari hedefe göre çalıştırılacak dosyaları bulur. Hedef bir dosyaysa yalnızca
// odur; klasörse altındaki (alt klasörler dahil) tüm *_test.tr dosyaları sıralı döner.
func TestDosyalari(hedef string) ([]string, error) {
	bilgi, err := os.Stat(hedef)
	if err != nil {
		return nil, fmt.Errorf("%s bulunamadı", hedef)
	}
	if !bilgi.IsDir() {
		return []string{hedef}, nil
	}
	var dosyalar []string
	err = filepath.WalkDir(hedef, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Gizli klasörler (.git, .claude) ve bağımlılık klasörleri taranmaz
			ad := d.Name()
			if yol != hedef && (strings.HasPrefix(ad, ".") || ad == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.tr") {
			dosyalar = append(dosyalar, yol)
		}
		return nil
	})
	sort.Strings(dosyalar)
	return dosyalar, err
}

// Calistir hedefteki tüm test dosyalarını çalıştırır, raporu cikti'ya yazar ve özeti döndürür.
func Calistir(hedef string, cikti io.Writer) (Ozet, error) {
	dosyalar, err := TestDosyalari(hedef)
	if err != nil {
		return Ozet{}, err
	}
	if len(dosyalar) == 0 {
		return Ozet{}, fmt.Errorf("%s içinde *_test.tr dosyası bulunamadı", hedef)
	}
	var toplam Ozet
	for _, d := range dosyalar {
		toplam.ekle(DosyaCalistir(d, cikti))
	}
	fmt.Fprintln(cikti)
	if toplam.Basarili() {
		fmt.Fprintf(cikti, "✅ %s\n", toplam)
	} else {
		fmt.Fprintf(cikti, "❌ %s\n", toplam)
	}
	return toplam, nil
}

// DosyaCalistir tek bir dosyanın test bloklarını çalıştırır. Önce dosya normal biçimde
// yüklenir (test blokları atlanarak üst düzey tanımlar oluşur); ardından her test, bu
// tanımların üzerinde açılan yeni bir kapsamda tek tek çalışır. Bir testin kalması ya da
// hata vermesi diğerlerini durdurmaz.
func DosyaCalistir(yol string, cikti io.Writer) Ozet {
	ozet := Ozet{Dosya: 1}
	fmt.Fprintf(cikti, "📄 %s\n", yol)

	veri, err := os.ReadFile(yol)
	if err != nil {
		fmt.Fprintf(cikti, "  ✗ %v\n", turkce.DosyaHatasi("", yol, "okunamadı", err))
		ozet.Yuklenemez++
		return ozet
	}
	kaynak := strings.TrimPrefix(string(veri), "\xef\xbb\xbf")

	p := ayristir.New(sozcuk.New(kaynak))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Fprintln(cikti, "  ✗ sözdizimi hataları nedeniyle testler çalıştırılamadı:")
		for _, e := range p.Errors() {
			hataYaz(cikti, e, kaynak)
		}
		ozet.Yuklenemez++
		return ozet
	}
	denetleyici := denetci.New()
	hatalar := denetleyici.CheckProgram(prog)
	if uyarilar := denetleyici.Warnings(); len(uyarilar) > 0 {
		// Uyarılar testleri durdurmaz ve sonucu etkilemez.
		fmt.Fprintln(cikti, "  ⚠️ uyarılar:")
		for _, u := range uyarilar {
			hataYaz(cikti, u, kaynak)
		}
	}
	if len(hatalar) > 0 {
		fmt.Fprintln(cikti, "  ✗ tip denetimi hataları nedeniyle testler çalıştırılamadı:")
		for _, e := range hatalar {
			hataYaz(cikti, e, kaynak)
		}
		ozet.Yuklenemez++
		return ozet
	}

	var testler []*agac.TestStatement
	for _, st := range prog.Statements {
		if t, ok := st.(*agac.TestStatement); ok {
			testler = append(testler, t)
		}
	}
	if len(testler) == 0 {
		fmt.Fprintln(cikti, "  (bu dosyada test bloğu yok)")
		return ozet
	}

	makine := yurut.New()
	makine.Out = cikti
	makine.Err = cikti // eşzamanlı görev hataları da raporda görünür
	makine.SourceFile = yol
	makine.SourceCode = kaynak
	if _, err := makine.Run(prog); err != nil {
		fmt.Fprintln(cikti, "  ✗ dosyanın üst düzey kodu hata verdi, testler çalıştırılamadı:")
		for _, m := range yurut.HataMaddeleri(err, kaynak) {
			fmt.Fprintf(cikti, "    • %s\n", m.Mesaj)
			fmt.Fprint(cikti, girintile(m.Cerceve, "      "))
		}
		ozet.Yuklenemez++
		return ozet
	}

	for _, t := range testler {
		ozet.Toplam++
		if err := makine.RunTest(t); err != nil {
			ozet.Kalan++
			mesaj := err.Error()
			if !strings.Contains(mesaj, "Satır ") {
				// Konumsuz hatalarda (ör. fırlat) testin satırını göster
				err = fmt.Errorf("Satır %d: %s", t.Line(), mesaj)
			}
			// Modüle ait hatanın çerçevesi modülün kaynağından çizilir.
			for i, m := range yurut.HataMaddeleri(err, kaynak) {
				if i == 0 {
					fmt.Fprintf(cikti, "  ✗ %s — %s\n", t.Name, m.Mesaj)
				} else {
					fmt.Fprintf(cikti, "      • %s\n", m.Mesaj)
				}
				fmt.Fprint(cikti, girintile(m.Cerceve, "      "))
			}
			continue
		}
		ozet.Gecen++
		fmt.Fprintf(cikti, "  ✓ %s\n", t.Name)
	}
	return ozet
}

func hataYaz(cikti io.Writer, mesaj, kaynak string) {
	fmt.Fprintf(cikti, "    • %s\n", mesaj)
	fmt.Fprint(cikti, girintile(turkce.KodCercevesi(kaynak, mesaj), "      "))
}

func girintile(metin, onek string) string {
	if metin == "" {
		return ""
	}
	var sb strings.Builder
	for _, s := range strings.Split(strings.TrimRight(metin, "\n"), "\n") {
		sb.WriteString(onek + s + "\n")
	}
	return sb.String()
}
