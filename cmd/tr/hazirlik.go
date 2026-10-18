package main

import (
	"fmt"
	"os"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/checker"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/motor"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
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
func dosyaHazirla(path string, h hazirlik) (string, *ast.Program) {
	code := kaynakOku(path)
	return code, kaynakHazirla(code, h)
}

// kaynakHazirla kodu ayrıştırır ve istenmişse tip denetiminden geçirir. Hata varsa
// hepsini yazıp 1 koduyla çıkar.
func kaynakHazirla(code string, h hazirlik) *ast.Program {
	p := parser.New(lexer.New(code))
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
		chk := checker.New()
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
func programCalistir(machine *vm.VM, prog *ast.Program, code string) {
	if _, err := machine.Run(prog); err != nil {
		motor.Kapat()
		hatalarlaCik(calismaBasligi, []string{err.Error()}, code)
	}
	machine.WaitAsync()
	motor.Kapat()
}
