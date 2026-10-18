// eklenti_listeleri, VS Code eklentisinin söz dizimi renklendirmesini
// (vscode_eklentisi/syntaxes/tr.tmLanguage.json) dilin sözlüğünden (pkg/sozluk, tr.json)
// üretir. Anahtar kelime, tip adı ve yerleşik fonksiyon listeleri elle yazılmaz; sözlükteki
// adlardan gelir. Böylece bir ad değiştiğinde ya da kaldırıldığında eklenti kendiliğinden
// güncel kalır ve kaldırılmış adlar (eski adlar, ASCII yazımlar) renklendirilmez: her işin
// tek yazımı vardır.
//
// Kullanım (depo kökünden):
//
//	go run ./araclar/eklenti_listeleri           // dosyayı yeniden yazar
//	go run ./araclar/eklenti_listeleri -denetle  // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./araclar/...` ile de yapılır.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"tr_dili/pkg/sozluk"
)

// dilbilgisiYolu depo köküne göre üretilen dosyadır.
var dilbilgisiYolu = filepath.Join("vscode_eklentisi", "syntaxes", "tr.tmLanguage.json")

func main() {
	denetle := flag.Bool("denetle", false, "dosyayı yazma; güncel değilse 1 koduyla çık")
	kok := flag.String("kok", ".", "depo kökü")
	flag.Parse()

	yol := filepath.Join(*kok, dilbilgisiYolu)
	yeni, err := Uret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	if *denetle {
		eski, err := os.ReadFile(yol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(eski, yeni) {
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./araclar/eklenti_listeleri ile yeniden üretin\n", yol)
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

// --- sözlükten listeler ---

// adlar kodların adlarıdır (sözlük sırasıyla).
func adlar(kodlar ...sozluk.Kod) []string { return sozluk.Adlar(kodlar...) }

// grubundakiler harf ailesinin grup numaralı grubundaki (grup < 0 ise bütün ailedeki) ve
// uygun(kod) doğru olan sözlük kodlarının adlarıdır.
func grubundakiler(harf byte, grup int, uygun func(sozluk.Kod) bool) []string {
	var out []string
	for _, k := range sozluk.Kodlar() {
		if k.Harf() == harf && (grup < 0 || k.Grup() == grup) && (uygun == nil || uygun(k)) {
			out = append(out, sozluk.Ad(k))
		}
	}
	return out
}

// yerlesik kodun genel kapsamdaki bir yerleşik fonksiyon olup olmadığıdır.
func yerlesik(k sozluk.Kod) bool { return k.Yerlesik() }

// kelimeler bir ad listesini \b(a|b|c)\b düzenli ifadesine çevirir. Adlar düzenli ifade
// karakterleri içermez; yine de kaçışlanır.
func kelimeler(adlar []string) string {
	parca := make([]string, len(adlar))
	for i, a := range adlar {
		parca[i] = regexp.QuoteMeta(a)
	}
	return `\b(` + strings.Join(parca, "|") + `)\b`
}

// --- sıralı JSON ---

// alan bir JSON nesnesinin anahtar-değer çiftidir; nesne alanlarını yazıldığı sırayla tutar.
type alan struct {
	ad    string
	deger any
}

type nesne []alan

func (n nesne) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, a := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := jsonYaz(a.ad)
		if err != nil {
			return nil, err
		}
		v, err := jsonYaz(a.deger)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// jsonYaz değeri HTML kaçışı yapmadan (düzenli ifadelerdeki < > & olduğu gibi) yazar.
func jsonYaz(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// kural adı ve eşleşmesi olan tek satırlık bir kuraldır.
func kural(ad, eslesme string) nesne {
	return nesne{{"name", ad}, {"match", eslesme}}
}

func yakala(adlar ...string) nesne {
	var n nesne
	for i, a := range adlar {
		n = append(n, alan{fmt.Sprint(i + 1), nesne{{"name", a}}})
	}
	return n
}

func ekle(desenler ...string) []nesne {
	out := make([]nesne, len(desenler))
	for i, d := range desenler {
		out[i] = nesne{{"include", d}}
	}
	return out
}

// Uret dil bilgisi dosyasının içeriğini sözlükten üretir.
func Uret() ([]byte, error) {
	ad := sozluk.Ad

	kontrol := adlar(sozluk.A001, sozluk.A002, sozluk.A003, sozluk.A004, sozluk.A010, sozluk.A011, sozluk.A012, sozluk.A013, sozluk.A014, sozluk.A021, sozluk.A060, sozluk.A061, sozluk.A062, sozluk.A080, sozluk.A081)
	bildirim := adlar(sozluk.A030, sozluk.A031, sozluk.A020, sozluk.A050)
	tipler := grubundakiler('A', 2, nil)
	genel := grubundakiler('G', -1, yerlesik)
	yz := append(grubundakiler('T', -1, yerlesik), grubundakiler('V', -1, yerlesik)...)
	veriTabani := grubundakiler('W', 2, yerlesik)
	web := grubundakiler('W', 1, yerlesik)
	arayuz := grubundakiler('W', 3, yerlesik)

	ic := func(ad string) string { return regexp.QuoteMeta(ad) }

	dilbilgisi := nesne{
		{"$schema", "https://raw.githubusercontent.com/martinring/tmlanguage/master/tmlanguage.json"},
		{"comment", "Bu dosya sözlükten (pkg/sozluk/tr.json) üretilir; elle düzenlemeyin: go run ./araclar/eklenti_listeleri"},
		{"name", "TR Dili"},
		{"scopeName", "source.tr"},
		{"patterns", ekle("#comments", "#strings", "#import", "#declarations", "#keywords", "#pronoun", "#types", "#builtins", "#numbers", "#operators")},
		{"repository", nesne{
			{"comments", nesne{{"patterns", []nesne{
				kural("comment.line.double-slash.tr", "//.*$"),
				{{"name", "comment.block.tr"}, {"begin", `/\*`}, {"end", `\*/`}},
			}}}},
			{"strings", nesne{{"patterns", []nesne{
				{{"name", "string.quoted.double.tr"}, {"begin", `"`}, {"end", `"`},
					{"beginCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.end.tr"}}}}},
					{"patterns", ekle("#string-content")}},
				{{"name", "string.quoted.single.tr"}, {"begin", `'`}, {"end", `'`},
					{"beginCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.end.tr"}}}}},
					{"patterns", ekle("#string-content")}},
			}}}},
			{"string-content", nesne{{"patterns", []nesne{
				kural("constant.character.escape.brace.tr", `\\[{}]`),
				kural("constant.character.escape.tr", `\\.`),
				{{"include", "#interpolation"}},
			}}}},
			{"interpolation", nesne{
				{"name", "meta.interpolation.tr"}, {"begin", `\{`}, {"end", `\}`},
				{"beginCaptures", nesne{{"0", nesne{{"name", "punctuation.section.interpolation.begin.tr"}}}}},
				{"endCaptures", nesne{{"0", nesne{{"name", "punctuation.section.interpolation.end.tr"}}}}},
				{"contentName", "meta.embedded.expression.tr"},
				{"patterns", ekle("#nested-braces", "$self")},
			}},
			{"nested-braces", nesne{
				{"begin", `\{`}, {"end", `\}`},
				{"beginCaptures", nesne{{"0", nesne{{"name", "punctuation.section.braces.begin.tr"}}}}},
				{"endCaptures", nesne{{"0", nesne{{"name", "punctuation.section.braces.end.tr"}}}}},
				{"patterns", ekle("#nested-braces", "$self")},
			}},
			{"import", nesne{{"patterns", []nesne{
				{{"match", `\b(` + ic(ad(sozluk.A070)) + `)\b\s*("[^"]*"|'[^']*'|[^\W\d][\w.]*)\s+(` + ic(ad(sozluk.A100)) + `)\b\s*([^\W\d]\w*)?`},
					{"captures", yakala("keyword.control.import.tr", "string.other.module.tr", "keyword.control.import.as.tr", "entity.name.namespace.tr")}},
				kural("keyword.control.import.tr", kelimeler(adlar(sozluk.A070))),
			}}}},
			{"declarations", nesne{{"patterns", []nesne{
				{{"match", `\b(` + ic(ad(sozluk.A020)) + `)\s+([^\W\d]\w*)`}, {"captures", yakala("storage.type.function.tr", "entity.name.function.tr")}},
				{{"match", `\b(` + ic(ad(sozluk.A050)) + `)\s+([^\W\d]\w*)`}, {"captures", yakala("storage.type.struct.tr", "entity.name.type.tr")}},
			}}}},
			{"keywords", nesne{{"patterns", []nesne{
				kural("keyword.control.tr", kelimeler(kontrol)),
				kural("keyword.other.debugger.tr", kelimeler(adlar(sozluk.A090))),
				kural("storage.type.tr", kelimeler(bildirim)),
				kural("variable.language.self.tr", kelimeler(adlar(sozluk.A051))),
				kural("constant.language.boolean.tr", kelimeler(adlar(sozluk.A040, sozluk.A041))),
				kural("constant.language.null.tr", kelimeler(adlar(sozluk.A042))),
				kural("keyword.operator.logical.tr", kelimeler(adlar(sozluk.A043, sozluk.A044, sozluk.A045))),
			}}}},
			{"pronoun", nesne{
				{"comment", "Sondaki blok (o-lambda) parametresi: liste.filtrele { o > 2 }"},
				{"patterns", []nesne{kural("variable.language.pronoun.tr", `(?<![\w.])o(?!\w)`)}},
			}},
			{"types", nesne{{"patterns", []nesne{kural("entity.name.type.tr", kelimeler(tipler))}}}},
			{"builtins", nesne{{"patterns", []nesne{
				kural("support.function.builtin.tr", kelimeler(genel)),
				kural("support.function.ai.tr", kelimeler(yz)),
				kural("support.function.data.tr", kelimeler(veriTabani)),
				kural("support.function.web.tr", kelimeler(web)),
				kural("support.function.gui.tr", kelimeler(arayuz)),
			}}}},
			{"numbers", nesne{{"patterns", []nesne{
				kural("constant.numeric.float.tr", `\b\d[\d_]*(?:\.\d[\d_]*)?[eE][+-]?\d+\b`),
				kural("constant.numeric.float.tr", `\b\d[\d_]*\.\d[\d_]*\b`),
				kural("constant.numeric.integer.tr", `\b\d[\d_]*\b`),
			}}}},
			{"operators", nesne{{"patterns", []nesne{
				kural("keyword.operator.range.tr", `\.\.=?`),
				kural("keyword.operator.pipe.tr", `\|>`),
				kural("keyword.operator.null-coalescing.tr", `\?\?`),
				kural("punctuation.accessor.optional.tr", `\?\.`),
				kural("keyword.operator.arrow.tr", `=>`),
				kural("keyword.operator.comparison.tr", `==|!=|<=|>=|<|>`),
				kural("keyword.operator.logical.tr", `&&|\|\||!`),
				kural("keyword.operator.assignment.compound.tr", `\+=|-=|\*=|/=`),
				kural("keyword.operator.assignment.tr", `:=|=`),
				kural("keyword.operator.arithmetic.tr", `\*\*|@|[+\-*/%]`),
				kural("keyword.operator.ternary.tr", `\?`),
			}}}},
		}},
	}

	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(dilbilgisi); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
