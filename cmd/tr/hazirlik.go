package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor"
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

// kaynakOku dosyayı okur ve baştaki UTF-8 BOM'u atar; okunamazsa hatayı yazıp çıkar.
func kaynakOku(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dosya okuma hatası: %v\n", turkce.DosyaHatasi("", path, "okunamadı", err))
		os.Exit(1)
	}
	return strings.TrimPrefix(string(data), "\xef\xbb\xbf")
}

// dosyaHazirla dosyayı okuyup kaynakHazirla'dan geçirir.
func dosyaHazirla(path string, h hazirlik) (string, *agac.Program) {
	code := kaynakOku(path)
	return code, kaynakHazirla(code, h)
}

// kaynakHazirla kodu ayrıştırır ve istenmişse tip denetiminden geçirir. Hata varsa
// hepsini yazıp 1 koduyla çıkar.
func kaynakHazirla(code string, h hazirlik) *agac.Program {
	p := ayristir.New(sozcuk.New(code))
	prog := p.ParseProgram()

	if errs := p.Errors(); len(errs) > 0 {
		if h.sozdizimiBasligi == "" {
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, e)
			}
			os.Exit(1)
		}
		hatalarlaCik(h.sozdizimiBasligi, errs, code)
	}

	if h.tipBasligi != "" {
		chk := denetci.New()
		errs := chk.CheckProgram(prog)
		uyarilariYaz(chk.Warnings(), code)
		if len(errs) > 0 {
			hatalarlaCik(h.tipBasligi, errs, code)
		}
	}
	return prog
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
func programCalistir(machine *yurut.VM, prog *agac.Program, code string) {
	kesmeyiYakala(machine)
	if _, err := machine.Run(prog); err != nil {
		if errors.Is(err, yurut.ErrKesildi) {
			machine.WaitAsync() // eşzamanlı görevler de kesildi; ertelenen işleri bitsin
			motor.Kapat()
			kesilmeyiBildir(machine, err, code)
		}
		motor.Kapat()
		calismaHatasiYaz(machine, err, code)
		cik(1)
	}
	machine.WaitAsync()
	motor.Kapat()
	if machine.Kesildi() {
		// Kesme isteği programın son beklemesinde (ör. bekle) geldi; program yine de durduruldu.
		kesilmeyiBildir(machine, nil, code)
	}
	if u := machine.KullanilmayanArgumanUyarisi(); u != "" {
		machine.CiktiYaz(os.Stderr, u+"\n")
	}
}

// calismaHatasiYaz bir çalışma zamanı hatasını başlığıyla standart hataya yazar. Bir modüle
// ait hatanın (ya da modülün denetim hatalarının) çerçevesi o modülün kaynağından çizilir.
// Metin VM'nin çıktı noktasından geçer (canlı satır, gizli değerler).
func calismaHatasiYaz(machine *yurut.VM, err error, code string) {
	var sb strings.Builder
	sb.WriteString(calismaBasligi + "\n")
	for _, m := range yurut.HataMaddeleri(err, code) {
		fmt.Fprintf(&sb, "  • %s\n", m.Mesaj)
		sb.WriteString(indentFrame(m.Cerceve))
	}
	machine.CiktiYaz(os.Stderr, sb.String())
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
