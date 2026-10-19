// eklenti_listeleri, VS Code eklentisinin söz dizimi renklendirmesini
// (editors/vscode/syntaxes/tr.tmLanguage.json) dilin kod listesinden (internal/lang/codes, tr.json)
// üretir. Anahtar kelime, tip adı ve yerleşik fonksiyon listeleri elle yazılmaz; sözlükteki
// adlardan gelir. Böylece bir ad değiştiğinde ya da kaldırıldığında eklenti kendiliğinden
// güncel kalır ve kaldırılmış adlar (eski adlar, ASCII yazımlar) renklendirilmez: her işin
// tek yazımı vardır.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/grammar           // dosyayı yeniden yazar
//	go run ./tools/gen/grammar -denetle  // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Katman: araçlar (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// dilbilgisiYolu depo köküne göre üretilen dosyadır.
var dilbilgisiYolu = filepath.Join("editors", "vscode", "syntaxes", "tr.tmLanguage.json")

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
			fmt.Fprintf(os.Stderr, "%s güncel değil; go run ./tools/gen/grammar ile yeniden üretin\n", yol)
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

// yazimDilleri gömülü bütün yazım dilleridir (Türkçe önce): kodlu dosya düzenleyicide seçilen
// yazım dilinde (tr.yazimDili) gösterilir, renklendirme her dilin adlarını tanır.
var yazimDilleri = func() []*codes.Dil {
	d := []*codes.Dil{codes.Etkin()}
	for _, kodu := range codes.Diller() {
		if kodu != codes.Etkin().Kodu {
			y, err := codes.DilYukle(kodu)
			if err != nil {
				panic(err)
			}
			d = append(d, y)
		}
	}
	return d
}()

// dillerdekiAdlar kodun bütün yazım dillerindeki adlarıdır (yinelenmeden, Türkçe önce).
func dillerdekiAdlar(k codes.Kod) []string {
	var out []string
	for _, d := range yazimDilleri {
		if g, ok := d.Girdisi(k); ok && !slices.Contains(out, g.Ad) {
			out = append(out, g.Ad)
		}
	}
	return out
}

// adlar kodların bütün yazım dillerindeki adlarıdır (önce Türkçeler sözlük sırasıyla, sonra
// öbür dillerdekiler).
func adlar(kodlar ...codes.Kod) []string {
	var out []string
	for _, d := range yazimDilleri {
		for _, k := range kodlar {
			if g, ok := d.Girdisi(k); ok && !slices.Contains(out, g.Ad) {
				out = append(out, g.Ad)
			}
		}
	}
	return out
}

// grubundakiler harf ailesinin grup numaralı grubundaki (grup < 0 ise bütün ailedeki) ve
// uygun(kod) doğru olan sözlük kodlarının adlarıdır.
func grubundakiler(harf byte, grup int, uygun func(codes.Kod) bool) []string {
	var secilen []codes.Kod
	for _, k := range codes.Kodlar() {
		if k.Harf() == harf && (grup < 0 || k.Grup() == grup) && (uygun == nil || uygun(k)) {
			secilen = append(secilen, k)
		}
	}
	return adlar(secilen...)
}

// yerlesik kodun genel kapsamdaki bir yerleşik fonksiyon olup olmadığıdır.
func yerlesik(k codes.Kod) bool { return k.Yerlesik() }

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
	// ad kodun bütün dillerdeki adlarının düzenli ifade seçeneğidir (değişken|var).
	ad := func(k codes.Kod) string { return strings.Join(dillerdekiAdlar(k), "|") }

	kontrol := adlar(codes.A001, codes.A002, codes.A003, codes.A004, codes.A010, codes.A011, codes.A012, codes.A013, codes.A014, codes.A021, codes.A060, codes.A061, codes.A062, codes.A080, codes.A081)
	bildirim := adlar(codes.A030, codes.A031, codes.A020, codes.A050)
	tipler := grubundakiler('A', 2, nil)
	genel := grubundakiler('G', -1, yerlesik)
	yz := append(grubundakiler('T', -1, yerlesik), grubundakiler('V', -1, yerlesik)...)

	ic := func(secenek string) string { return secenek } // adlar düzenli ifade karakteri içermez

	dilbilgisi := nesne{
		{"$schema", "https://raw.githubusercontent.com/martinring/tmlanguage/master/tmlanguage.json"},
		{"comment", "Bu dosya kod listesinden (internal/lang/codes/tr.json) üretilir; elle düzenlemeyin: go run ./tools/gen/grammar"},
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
				// Ham metin (2.6a): kaçış ve yerleştirme yok, çok satırlı olabilir.
				{{"name", "string.quoted.other.raw.tr"}, {"begin", "`"}, {"end", "`"},
					{"beginCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", nesne{{"0", nesne{{"name", "punctuation.definition.string.end.tr"}}}}}},
			}}}},
			{"string-content", nesne{{"patterns", []nesne{
				kural("constant.character.escape.brace.tr", `\\[{}]`),
				kural("constant.character.escape.tr", `\\[ntr"'\\]`),
				// Bilinmeyen kaçış dizisi sözdizimi hatasıdır (2.6).
				kural("invalid.illegal.escape.tr", `\\.`),
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
				{{"match", `\b(` + ic(ad(codes.A070)) + `)\b\s*("[^"]*"|'[^']*'|[^\W\d][\w.]*)\s+(` + ic(ad(codes.A100)) + `)\b\s*([^\W\d]\w*)?`},
					{"captures", yakala("keyword.control.import.tr", "string.other.module.tr", "keyword.control.import.as.tr", "entity.name.namespace.tr")}},
				kural("keyword.control.import.tr", kelimeler(adlar(codes.A070))),
			}}}},
			{"declarations", nesne{{"patterns", []nesne{
				{{"match", `\b(` + ic(ad(codes.A020)) + `)\s+([^\W\d]\w*)`}, {"captures", yakala("storage.type.function.tr", "entity.name.function.tr")}},
				{{"match", `\b(` + ic(ad(codes.A050)) + `)\s+([^\W\d]\w*)`}, {"captures", yakala("storage.type.struct.tr", "entity.name.type.tr")}},
			}}}},
			{"keywords", nesne{{"patterns", []nesne{
				kural("keyword.control.tr", kelimeler(kontrol)),
				kural("keyword.other.debugger.tr", kelimeler(adlar(codes.A090))),
				kural("storage.type.tr", kelimeler(bildirim)),
				kural("variable.language.self.tr", kelimeler(adlar(codes.A051))),
				kural("constant.language.boolean.tr", kelimeler(adlar(codes.A040, codes.A041))),
				kural("constant.language.null.tr", kelimeler(adlar(codes.A042))),
				kural("keyword.operator.logical.tr", kelimeler(adlar(codes.A043, codes.A044, codes.A045))),
			}}}},
			{"pronoun", nesne{
				{"comment", "Sondaki blok (o-lambda) parametresi: liste.filtrele { o > 2 }"},
				{"patterns", []nesne{kural("variable.language.pronoun.tr", `(?<![\w.])o(?!\w)`)}},
			}},
			{"types", nesne{{"patterns", []nesne{kural("entity.name.type.tr", kelimeler(tipler))}}}},
			{"builtins", nesne{{"patterns", []nesne{
				kural("support.function.builtin.tr", kelimeler(genel)),
				kural("support.function.ai.tr", kelimeler(yz)),
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
