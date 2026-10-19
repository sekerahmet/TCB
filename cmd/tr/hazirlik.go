package main

import (
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"os"
	"strings"
)

// hazirlik bir kaynağın çalıştırılmadan önce hangi aşamalardan geçeceğini ve hata
// başlıklarını belirler. Komutlar arasındaki tek fark bu başlıklar ve tip denetiminin
// yapılıp yapılmamasıdır.
type hazirlik struct {
	// sozdizimiBasligi sözdizimi hatalarının üstüne yazılır. Boşsa hatalar başlıksız ve
	// kod çerçevesi olmadan, her biri tek satır olarak yazılır.
	sozdizimiBasligi string
	// tipBasligi boş değilse tip denetimi yapılır ve hataları bu başlıkla yazılır.
	tipBasligi string
}

// Dosya çalıştırmada ve etkileşimli kabukta (REPL) ortak kullanılan hata başlıkları.
const (
	sozdizimiBasligi = "❌ Sözdizimi Hataları:"
	tipBasligi       = "❌ Tip ve Güvenlik Denetimi Hataları:"
	calismaBasligi   = "❌ Çalışma Zamanı Hatası:"
	// uyariBasligi denetleyicinin uyarılarının (ör. yerleşik ad gölgeleme) başlığıdır.
	// Uyarılar programı durdurmaz ve çıkış kodunu etkilemez.
	uyariBasligi = "⚠️ Uyarılar:"
)

// Komutların kullandığı hazırlık biçimleri.
var (
	calistirmaHazirligi = hazirlik{
		sozdizimiBasligi: sozdizimiBasligi,
		tipBasligi:       tipBasligi,
	}
	hataAyiklamaHazirligi = hazirlik{
		sozdizimiBasligi: sozdizimiBasligi,
	}
	derlemeHazirligi = hazirlik{
		sozdizimiBasligi: "❌ Derleme Öncesi Sözdizimi Hataları:",
		tipBasligi:       "❌ Derleme Öncesi Tip Güvenliği Hataları:",
	}
	gomuluHazirlik = hazirlik{
		sozdizimiBasligi: "❌ Gömülü Uygulama Sözdizimi Hataları:",
	}
)

// kaynakOku dosyanın metnini okur; okunamazsa hatayı yazıp çıkar. BOM ve kodlu biçim
// başlığı internal/lang/source'ta çözülür.
func kaynakOku(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası: %v\n", turkish.FileError("", path, "okunamadı", err))
		os.Exit(1)
	}
	return string(data)
}

// dosyaHazirla dosyayı okuyup kaynakHazirla'dan geçirir; dosyanın metnini (paket ve modül
// denetimi için) ve hazırlığın sonucunu verir.
func dosyaHazirla(path string, h hazirlik) (string, *source.Result) {
	metin := kaynakOku(path)
	return metin, kaynakHazirla(metin, h)
}

// kaynakHazirla kodu ayrıştırır ve istenmişse tip denetiminden geçirir (kaynak.Hazirla). Hata
// varsa hepsini yazıp 1 koduyla çıkar. Hata çerçeveleri ve yorumlayıcının kaynak metni
// sonucun gövdesidir (kodlu dosyada başlıktan sonrası).
func kaynakHazirla(metin string, h hazirlik) *source.Result {
	s := source.Prepare(metin, source.Options{TypeCheck: h.tipBasligi != ""})
	uyarilariYaz(s.Warnings, s.Text)
	switch s.Stage {
	case source.StageSyntax:
		if h.sozdizimiBasligi == "" {
			for _, e := range s.Errors {
				fmt.Fprintln(os.Stderr, e)
			}
			os.Exit(1)
		}
		hatalarlaCik(h.sozdizimiBasligi, s.Errors, s.Text)
	case source.StageType:
		hatalarlaCik(h.tipBasligi, s.Errors, s.Text)
	}
	return s
}

// uyarilariYaz denetleyicinin uyarılarını (varsa) hatalardan ayrı bir başlıkla standart
// hataya yazar; çıkış kodunu etkilemez.
func uyarilariYaz(uyarilar []string, code string) {
	if len(uyarilar) > 0 {
		hatalariYaz(uyariBasligi, uyarilar, code)
	}
}

// hatalariYaz başlığı ve her hatayı kod çerçevesiyle standart hataya yazar.
func hatalariYaz(baslik string, errs []string, code string) {
	fmt.Fprintln(os.Stderr, baslik)
	for _, e := range errs {
		printError(e, code)
	}
}

// hatalarlaCik hataları hatalariYaz ile yazıp (temizlik işlerinden sonra) 1 koduyla çıkar.
func hatalarlaCik(baslik string, errs []string, code string) {
	hatalariYaz(baslik, errs, code)
	cik(1)
}

// programCalistir programı çalıştırır, çalışma zamanı hatasını yazıp çıkar; başarılıysa
// eşzamansız işlerin bitmesini bekler. Her iki durumda da motorların açık kaynakları
// (veri okuyucuları, önbellekler) çıkmadan önce kapatılır.
//
// Ctrl+C ve sonlandırma istekleri yakalanır (kesme.go): program nazikçe durur, ertelenen
// işler çalışır ve 3 koduyla çıkılır.
func programCalistir(machine *interp.VM, prog *ast.Program, code string) {
	kesmeyiYakala(machine)
	if _, err := machine.Run(prog); err != nil {
		if c, ok := interp.IsExitRequest(err); ok {
			machine.WaitAsync()
			motoruKapat()
			programCikisi(machine, c)
		}
		if errors.Is(err, interp.ErrInterrupted) {
			machine.WaitAsync() // eşzamanlı görevler de kesildi; ertelenen işleri bitsin
			motoruKapat()
			kesilmeyiBildir(machine, err, code)
		}
		motoruKapat()
		calismaHatasiYaz(machine, err, code)
		cik(1)
	}
	machine.WaitAsync()
	motoruKapat()
	if machine.Interrupted() {
		// Kesme isteği programın son beklemesinde (ör. bekle) geldi; program yine de durduruldu.
		kesilmeyiBildir(machine, nil, code)
	}
	if u := machine.UnusedArgsWarning(); u != "" {
		machine.WriteOutput(os.Stderr, u+"\n")
	}
}

// kapanmadanOnce program bittiğinde, motorlar kapanmadan önce çalışacak işlerdir (ör. tr izle
// ve tr profil GPU'nun son ölçümlerini arka uç kapanmadan alır).
var kapanmadanOnce []func()

// motoruKapat kapanmadanOnce işlerini (bir kez) çalıştırır ve motorları kapatır.
func motoruKapat() {
	isler := kapanmadanOnce
	kapanmadanOnce = nil
	for _, f := range isler {
		f()
	}
	stdlib.Kapat()
}

// programCikisi bir yerleşiğin istediği program çıkışını yapar (ör. komut satırı
// seçeneklerinde --yardım: 0, hatalı argüman: 2): mesaj 0 kodunda standart çıktıya, öbür
// kodlarda standart hataya yazılır.
func programCikisi(machine *interp.VM, c *interp.ExitRequest) {
	if c.Message != "" {
		w := os.Stderr
		if c.Code == 0 {
			w = os.Stdout
		}
		machine.WriteOutput(w, c.Message+"\n")
	}
	cik(c.Code)
}

// calismaHatasiYaz bir çalışma zamanı hatasını başlığıyla standart hataya yazar. Bir modüle
// ait hatanın (ya da modülün denetim hatalarının) çerçevesi o modülün kaynağından çizilir.
// Metin VM'nin çıktı noktasından geçer (canlı satır, gizli değerler).
func calismaHatasiYaz(machine *interp.VM, err error, code string) {
	var sb strings.Builder
	sb.WriteString(calismaBasligi + "\n")
	for _, m := range interp.ErrorItems(err, code) {
		fmt.Fprintf(&sb, "  • %s\n", m.Message)
		sb.WriteString(indentFrame(m.Frame))
	}
	machine.WriteOutput(os.Stderr, sb.String())
}

// modulBasligi bir hata başlığına modülün yolunu ekler: "❌ X:" → "❌ X (yol modülünde):".
func modulBasligi(baslik, yol string) string {
	return strings.TrimSuffix(baslik, ":") + " (" + yol + " modülünde):"
}

// modulleriDenetle programın 'aktar' ile aldığı bütün modülleri (özyinelemeli) bulur ve
// çalıştırmadan denetler; 'tr kontrol' ve 'tr derle' kullanır. Hata varsa yazıp 1 koduyla
// çıkar. genelBaslik bulunamayan/okunamayan modül hatalarının başlığıdır.
func modulleriDenetle(path, code string, h hazirlik, genelBaslik string) *paket {
	pk, err := paketHazirla(path, code)
	if err == nil {
		return pk
	}
	var mh *modulDenetimHatasi
	if errors.As(err, &mh) {
		baslik := h.tipBasligi
		if mh.tur == "sözdizimi" {
			baslik = h.sozdizimiBasligi
		}
		hatalarlaCik(modulBasligi(baslik, mh.yol), mh.hatalar, mh.kod)
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", genelBaslik, err)
	cik(1)
	return nil
}
