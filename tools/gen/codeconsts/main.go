// codeconsts, sözlükteki her kod için Go'da adlı bir sabit üretir
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
	trPath          = filepath.Join("internal", "lang", "codes", "tr.json")
	enPath          = filepath.Join("internal", "lang", "codes", "en.json")
	definitionsPath = filepath.Join("internal", "lang", "codes", "definitions.json")
	outputPath      = filepath.Join("internal", "lang", "codes", "codes_generated.go")
)

func main() {
	check := flag.Bool("denetle", false, "dosyayı yazma; güncel değilse 1 koduyla çık")
	root := flag.String("kok", "", "depo kökü (boşsa çalışma klasöründen yukarı doğru go.mod aranır)")
	flag.Parse()

	if *root == "" {
		*root = repoRoot()
	}
	fresh, err := Generate(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	path := filepath.Join(*root, outputPath)
	if *check {
		old, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), fresh) {
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./tools/gen/codeconsts ile yeniden üretin\n", path)
			os.Exit(1)
		}
		fmt.Printf("%s güncel\n", path)
		return
	}
	if err := os.WriteFile(path, fresh, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s yazıldı\n", path)
}

// repoRoot çalışma klasöründen yukarı doğru go.mod'un bulunduğu klasörü arar (go generate
// internal/lang/codes içinden çalıştırır).
func repoRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "."
		}
		d = parent
	}
}

// familyLetters internal/lang/codes'daki sırayla aynıdır (A=1000, G=2000, H=3000 ...).
const familyLetters = "AGHKTVWY"

type codeEntry struct {
	letter byte
	number int
	text   string
}

func parseCode(s string) (codeEntry, error) {
	if len(s) != 4 || bytes.IndexByte([]byte(familyLetters), s[0]) < 0 {
		return codeEntry{}, fmt.Errorf("kod %q geçersiz", s)
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n <= 0 {
		return codeEntry{}, fmt.Errorf("kod %q geçersiz", s)
	}
	return codeEntry{letter: s[0], number: n, text: s}, nil
}

// trEntry dil dosyasındaki bir satırın üreticinin kullandığı alanlarıdır.
type trEntry struct {
	Name string `json:"ad"`
}

// definitionEntry tanım dosyasındaki bir satırın üreticinin kullandığı alanlarıdır.
type definitionEntry struct {
	Kind    string `json:"tür"`
	Moved   string `json:"taşındı"`
	Removed bool   `json:"kaldırıldı"`
}

// readJSON decodes the JSON file at kok/path into v. A missing file is an error only when
// required is true; otherwise v is left as it is.
func readJSON(root, path string, required bool, v any) error {
	data, err := os.ReadFile(filepath.Join(root, path))
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

// Generate üretilen Go dosyasının içeriğini verir.
func Generate(root string) ([]byte, error) {
	var tr map[string]trEntry
	if err := readJSON(root, trPath, true, &tr); err != nil {
		return nil, err
	}
	// English names go next to the Turkish ones in the constants' comments, so a reader of the
	// (English) Go code knows what a code is without opening the language file.
	var en map[string]trEntry
	if err := readJSON(root, enPath, false, &en); err != nil {
		return nil, err
	}
	definitions := map[string]definitionEntry{}
	if err := readJSON(root, definitionsPath, false, &definitions); err != nil {
		return nil, err
	}

	var entries []codeEntry
	for s := range tr {
		k, err := parseCode(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", trPath, err)
		}
		entries = append(entries, k)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].letter != entries[j].letter {
			return bytes.IndexByte([]byte(familyLetters), entries[i].letter) < bytes.IndexByte([]byte(familyLetters), entries[j].letter)
		}
		return entries[i].number < entries[j].number
	})

	var b bytes.Buffer
	b.WriteString("// Code generated by go run ./tools/gen/codeconsts; DO NOT EDIT.\n\n")
	b.WriteString("package codes\n\n")
	b.WriteString("// A named constant for every live code in the dictionary (tr.json). The constant's name is\n")
	b.WriteString("// the code itself, not the user-visible name: renaming a name never changes the constant.\n")
	b.WriteString("// The name in the comment only helps the reader. Moved and removed codes have no constant;\n")
	b.WriteString("// see movedMap and removedSet.\n")
	b.WriteString("const (\n")
	var movedEntries, removedEntries []codeEntry
	for _, k := range entries {
		if definitions[k.text].Moved != "" {
			movedEntries = append(movedEntries, k)
			continue
		}
		if definitions[k.text].Removed {
			removedEntries = append(removedEntries, k)
			continue
		}
		name := tr[k.text].Name
		if e := en[k.text].Name; e != "" && e != name {
			name = e + " · " + name
		}
		fmt.Fprintf(&b, "\t%s Code = %c + %d // %s\n", k.text, k.letter, k.number, name)
	}
	b.WriteString(")\n\n")

	b.WriteString("// movedMap kaldırılıp işi başka bir koda (başka bir biçime: fonksiyondan metoda ya da\n")
	b.WriteString("// metottan fonksiyona) taşınan kodlardır: eski kod → yeni kod (definitions.json'daki\n")
	b.WriteString("// \"taşındı\" alanı). Eski kodun numarası bir daha kullanılmaz; adı yalnızca hata\n")
	b.WriteString("// mesajında yeni biçimi önermek için dil dosyasında kalır.\n")
	b.WriteString("var movedMap = map[Code]Code{\n")
	for _, k := range movedEntries {
		target, err := parseCode(definitions[k.text].Moved)
		if err != nil {
			return nil, fmt.Errorf("%s: %s taşındı: %v", definitionsPath, k.text, err)
		}
		fmt.Fprintf(&b, "\t%c + %d: %c + %d, // %s → %s\n", k.letter, k.number, target.letter, target.number, k.text, target.text)
	}
	b.WriteString("}\n\n")
	b.WriteString("// removedSet yerine geçen olmadan dilden kaldırılan kodlardır (definitions.json'daki\n")
	b.WriteString("// \"kaldırıldı\" alanı). Numaraları bir daha kullanılmaz; adları yalnızca \"TR'den\n")
	b.WriteString("// kaldırıldı\" hatası için dil dosyasında kalır.\n")
	b.WriteString("var removedSet = map[Code]bool{\n")
	for _, k := range removedEntries {
		fmt.Fprintf(&b, "\t%c + %d: true, // %s %s\n", k.letter, k.number, k.text, tr[k.text].Name)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
