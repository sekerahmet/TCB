// kod_sabitleri, sözlükteki her kod için Go'da adlı bir sabit üretir
// (internal/lang/codes/codes_generated.go): codes.G100, codes.A001, codes.T503 ... Sabitin adı
// kodun kendisidir, kullanıcıya görünen ad değildir; ad sonradan değişse de Go kodu
// değişmez. Go kodu kodları yalnızca bu sabitlerle yazar (codes.G + 100 biçimi
// kullanılmaz); sözlükte olmayan bir kod yazılırsa derleme düşer.
//
// Üretici kod paketini içe aktarmaz: dil dosyasını (internal/lang/codes/tr.json) ve dilden
// bağımsız tanımları (internal/lang/codes/definitions.json) kendisi okur. Böylece üretilen dosya
// eskimiş olsa da üretici çalışır.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/codeconsts           // dosyayı yeniden yazar
//	go run ./tools/gen/codeconsts -denetle  // güncel değilse 1 koduyla çıkar
//	go generate ./internal/lang/codes                 // aynısı
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Depo köküne göre yollar.
var (
	trYolu    = filepath.Join("internal", "lang", "codes", "tr.json")
	enYolu    = filepath.Join("internal", "lang", "codes", "en.json")
	tanimYolu = filepath.Join("internal", "lang", "codes", "definitions.json")
	ciktiYolu = filepath.Join("internal", "lang", "codes", "codes_generated.go")
)

func main() {
	denetle := flag.Bool("denetle", false, "dosyayı yazma; güncel değilse 1 koduyla çık")
	kok := flag.String("kok", "", "depo kökü (boşsa çalışma klasöründen yukarı doğru go.mod aranır)")
	flag.Parse()

	if *kok == "" {
		*kok = depoKoku()
	}
	yeni, err := Uret(*kok)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	yol := filepath.Join(*kok, ciktiYolu)
	if *denetle {
		eski, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./tools/gen/codeconsts ile yeniden üretin\n", yol)
			os.Exit(1)
		}
		fmt.Printf("%s güncel\n", yol)
		return
	}
	if err := os.WriteFile(yol, yeni, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s yazıldı\n", yol)
}

// depoKoku çalışma klasöründen yukarı doğru go.mod'un bulunduğu klasörü arar (go generate
// internal/lang/codes içinden çalıştırır).
func depoKoku() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		ust := filepath.Dir(d)
		if ust == d {
			return "."
		}
		d = ust
	}
}

// aileHarfleri internal/lang/codes'daki sırayla aynıdır (A=1000, G=2000, H=3000 ...).
const aileHarfleri = "AGHTVWY"

type kod struct {
	harf  byte
	sayi  int
	metin string
}

func kodCoz(s string) (kod, error) {
	if len(s) != 4 || bytes.IndexByte([]byte(aileHarfleri), s[0]) < 0 {
		return kod{}, fmt.Errorf("kod %q geçersiz", s)
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n <= 0 {
		return kod{}, fmt.Errorf("kod %q geçersiz", s)
	}
	return kod{harf: s[0], sayi: n, metin: s}, nil
}

// trGirdisi dil dosyasındaki bir satırın üreticinin kullandığı alanlarıdır.
type trGirdisi struct {
	Ad string `json:"ad"`
}

// tanimGirdisi tanım dosyasındaki bir satırın üreticinin kullandığı alanlarıdır.
type tanimGirdisi struct {
	Tur     string `json:"tür"`
	Tasindi string `json:"taşındı"`
	Kaldir  bool   `json:"kaldırıldı"`
}

// readJSON decodes the JSON file at kok/path into v. A missing file is an error only when
// required is true; otherwise v is left as it is.
func readJSON(kok, path string, required bool, v any) error {
	data, err := os.ReadFile(filepath.Join(kok, path))
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

// Uret üretilen Go dosyasının içeriğini verir.
func Uret(kok string) ([]byte, error) {
	var tr map[string]trGirdisi
	if err := readJSON(kok, trYolu, true, &tr); err != nil {
		return nil, err
	}
	// English names go next to the Turkish ones in the constants' comments, so a reader of the
	// (English) Go code knows what a code is without opening the language file.
	var en map[string]trGirdisi
	if err := readJSON(kok, enYolu, false, &en); err != nil {
		return nil, err
	}
	tanimlar := map[string]tanimGirdisi{}
	if err := readJSON(kok, tanimYolu, false, &tanimlar); err != nil {
		return nil, err
	}

	var kodlar []kod
	for s := range tr {
		k, err := kodCoz(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", trYolu, err)
		}
		kodlar = append(kodlar, k)
	}
	sort.Slice(kodlar, func(i, j int) bool {
		if kodlar[i].harf != kodlar[j].harf {
			return bytes.IndexByte([]byte(aileHarfleri), kodlar[i].harf) < bytes.IndexByte([]byte(aileHarfleri), kodlar[j].harf)
		}
		return kodlar[i].sayi < kodlar[j].sayi
	})

	var b bytes.Buffer
	b.WriteString("// Code generated by go run ./tools/gen/codeconsts; DO NOT EDIT.\n\n")
	b.WriteString("package codes\n\n")
	b.WriteString("// A named constant for every live code in the dictionary (tr.json). The constant's name is\n")
	b.WriteString("// the code itself, not the user-visible name: renaming a name never changes the constant.\n")
	b.WriteString("// The name in the comment only helps the reader. Moved and removed codes have no constant;\n")
	b.WriteString("// see movedMap and removedSet.\n")
	b.WriteString("const (\n")
	var tasinan, kaldirilan []kod
	for _, k := range kodlar {
		if tanimlar[k.metin].Tasindi != "" {
			tasinan = append(tasinan, k)
			continue
		}
		if tanimlar[k.metin].Kaldir {
			kaldirilan = append(kaldirilan, k)
			continue
		}
		ad := tr[k.metin].Ad
		if e := en[k.metin].Ad; e != "" && e != ad {
			ad = e + " · " + ad
		}
		fmt.Fprintf(&b, "\t%s Code = %c + %d // %s\n", k.metin, k.harf, k.sayi, ad)
	}
	b.WriteString(")\n\n")

	b.WriteString("// movedMap kaldırılıp işi başka bir koda (başka bir biçime: fonksiyondan metoda ya da\n")
	b.WriteString("// metottan fonksiyona) taşınan kodlardır: eski kod → yeni kod (definitions.json'daki\n")
	b.WriteString("// \"taşındı\" alanı). Eski kodun numarası bir daha kullanılmaz; adı yalnızca hata\n")
	b.WriteString("// mesajında yeni biçimi önermek için dil dosyasında kalır.\n")
	b.WriteString("var movedMap = map[Code]Code{\n")
	for _, k := range tasinan {
		hedef, err := kodCoz(tanimlar[k.metin].Tasindi)
		if err != nil {
			return nil, fmt.Errorf("%s: %s taşındı: %v", tanimYolu, k.metin, err)
		}
		fmt.Fprintf(&b, "\t%c + %d: %c + %d, // %s → %s\n", k.harf, k.sayi, hedef.harf, hedef.sayi, k.metin, hedef.metin)
	}
	b.WriteString("}\n\n")
	b.WriteString("// removedSet yerine geçen olmadan dilden kaldırılan kodlardır (definitions.json'daki\n")
	b.WriteString("// \"kaldırıldı\" alanı). Numaraları bir daha kullanılmaz; adları yalnızca \"TR'den\n")
	b.WriteString("// kaldırıldı\" hatası için dil dosyasında kalır.\n")
	b.WriteString("var removedSet = map[Code]bool{\n")
	for _, k := range kaldirilan {
		fmt.Fprintf(&b, "\t%c + %d: true, // %s %s\n", k.harf, k.sayi, k.metin, tr[k.metin].Ad)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
