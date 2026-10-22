// grammar, VS Code eklentisinin söz dizimi renklendirmesini
// (editors/vscode/syntaxes/tr.tmLanguage.json) dilin kod listesinden (internal/lang/codes, tr.json)
// üretir. Anahtar kelime, tip adı ve yerleşik fonksiyon listeleri elle yazılmaz; sözlükteki
// adlardan gelir. Böylece bir ad değiştiğinde ya da kaldırıldığında eklenti kendiliğinden
// güncel kalır ve kaldırılmış adlar (eski adlar, ASCII yazımlar) renklendirilmez: her işin
// tek yazımı vardır.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/grammar           // dosyayı yeniden yazar
//	go run ./tools/gen/grammar -check    // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// grammarPath depo köküne göre üretilen dosyadır.
var grammarPath = filepath.Join("editors", "vscode", "syntaxes", "tr.tmLanguage.json")

func main() {
	genmain.RunMain("grammar", grammarPath, func(string) ([]byte, error) { return Generate() })
}

// --- sözlükten listeler ---

// spellingLanguages gömülü bütün yazım dilleridir (Türkçe önce): kodlu dosya düzenleyicide seçilen
// yazım dilinde (tr.yazimDili) gösterilir, renklendirme her dilin adlarını tanır.
var spellingLanguages = func() []*codes.Language {
	d := []*codes.Language{codes.Active()}
	for _, langCode := range codes.Languages() {
		if langCode != codes.Active().Code {
			y, err := codes.LoadLanguage(langCode)
			if err != nil {
				panic(err)
			}
			d = append(d, y)
		}
	}
	return d
}()

// namesInAllLanguages kodun bütün yazım dillerindeki adlarıdır (yinelenmeden, Türkçe önce).
func namesInAllLanguages(k codes.Code) []string {
	var out []string
	for _, d := range spellingLanguages {
		if g, ok := d.Entry(k); ok && !slices.Contains(out, g.Name) {
			out = append(out, g.Name)
		}
	}
	return out
}

// names kodların bütün yazım dillerindeki adlarıdır (önce Türkçeler sözlük sırasıyla, sonra
// öbür dillerdekiler).
func names(codeList ...codes.Code) []string {
	var out []string
	for _, d := range spellingLanguages {
		for _, k := range codeList {
			if g, ok := d.Entry(k); ok && !slices.Contains(out, g.Name) {
				out = append(out, g.Name)
			}
		}
	}
	return out
}

// namesInGroup harf ailesinin grup numaralı grubundaki (grup < 0 ise bütün ailedeki) ve
// uygun(kod) doğru olan sözlük kodlarının adlarıdır.
func namesInGroup(letter byte, group int, pred func(codes.Code) bool) []string {
	var selected []codes.Code
	for _, k := range codes.Codes() {
		if k.Letter() == letter && (group < 0 || k.Group() == group) && (pred == nil || pred(k)) {
			selected = append(selected, k)
		}
	}
	return names(selected...)
}

// wordsPattern bir ad listesini \b(a|b|c)\b düzenli ifadesine çevirir. Adlar düzenli ifade
// karakterleri içermez; yine de kaçışlanır.
func wordsPattern(names []string) string {
	parts := make([]string, len(names))
	for i, a := range names {
		parts[i] = regexp.QuoteMeta(a)
	}
	return `\b(` + strings.Join(parts, "|") + `)\b`
}

// --- sıralı JSON ---

// jsonField bir JSON nesnesinin anahtar-değer çiftidir; nesne alanlarını yazıldığı sırayla tutar.
type jsonField struct {
	name  string
	value any
}

type jsonObject []jsonField

func (n jsonObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, a := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshalJSON(a.name)
		if err != nil {
			return nil, err
		}
		v, err := marshalJSON(a.value)
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
func marshalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// rule adı ve eşleşmesi olan tek satırlık bir kuraldır.
func rule(name, match string) jsonObject {
	return jsonObject{{"name", name}, {"match", match}}
}

func captures(names ...string) jsonObject {
	var n jsonObject
	for i, a := range names {
		n = append(n, jsonField{fmt.Sprint(i + 1), jsonObject{{"name", a}}})
	}
	return n
}

func includes(patterns ...string) []jsonObject {
	out := make([]jsonObject, len(patterns))
	for i, d := range patterns {
		out[i] = jsonObject{{"include", d}}
	}
	return out
}

// Generate dil bilgisi dosyasının içeriğini sözlükten üretir.
func Generate() ([]byte, error) {
	// ad kodun bütün dillerdeki adlarının düzenli ifade seçeneğidir (değişken|var).
	name := func(k codes.Code) string { return strings.Join(namesInAllLanguages(k), "|") }

	control := names(codes.A001, codes.A002, codes.A003, codes.A004, codes.A010, codes.A011, codes.A012, codes.A013, codes.A014, codes.A021, codes.A060, codes.A061, codes.A062, codes.A080, codes.A081)
	declaration := names(codes.A030, codes.A031, codes.A020, codes.A050)
	types := namesInGroup('A', 2, nil)
	general := namesInGroup('G', -1, codes.Code.IsBuiltin)
	ai := append(namesInGroup('T', -1, codes.Code.IsBuiltin), namesInGroup('V', -1, codes.Code.IsBuiltin)...)

	inner := func(option string) string { return option } // adlar düzenli ifade karakteri içermez

	grammar := jsonObject{
		{"$schema", "https://raw.githubusercontent.com/martinring/tmlanguage/master/tmlanguage.json"},
		{"comment", "This file is generated from the code list (internal/lang/codes/tr.json); do not edit it by hand: go run ./tools/gen/grammar"},
		{"name", "TR Dili"},
		{"scopeName", "source.tr"},
		{"patterns", includes("#comments", "#strings", "#import", "#declarations", "#keywords", "#pronoun", "#types", "#builtins", "#numbers", "#operators")},
		{"repository", jsonObject{
			{"comments", jsonObject{{"patterns", []jsonObject{
				rule("comment.line.double-slash.tr", "//.*$"),
				{{"name", "comment.block.tr"}, {"begin", `/\*`}, {"end", `\*/`}},
			}}}},
			{"strings", jsonObject{{"patterns", []jsonObject{
				{{"name", "string.quoted.double.tr"}, {"begin", `"`}, {"end", `"`},
					{"beginCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.end.tr"}}}}},
					{"patterns", includes("#string-content")}},
				{{"name", "string.quoted.single.tr"}, {"begin", `'`}, {"end", `'`},
					{"beginCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.end.tr"}}}}},
					{"patterns", includes("#string-content")}},
				// Ham metin (2.6a): kaçış ve yerleştirme yok, çok satırlı olabilir.
				{{"name", "string.quoted.other.raw.tr"}, {"begin", "`"}, {"end", "`"},
					{"beginCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.begin.tr"}}}}},
					{"endCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.definition.string.end.tr"}}}}}},
			}}}},
			{"string-content", jsonObject{{"patterns", []jsonObject{
				rule("constant.character.escape.brace.tr", `\\[{}]`),
				rule("constant.character.escape.tr", `\\[ntr"'\\]`),
				// Bilinmeyen kaçış dizisi sözdizimi hatasıdır (2.6).
				rule("invalid.illegal.escape.tr", `\\.`),
				{{"include", "#interpolation"}},
			}}}},
			{"interpolation", jsonObject{
				{"name", "meta.interpolation.tr"}, {"begin", `\{`}, {"end", `\}`},
				{"beginCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.section.interpolation.begin.tr"}}}}},
				{"endCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.section.interpolation.end.tr"}}}}},
				{"contentName", "meta.embedded.expression.tr"},
				{"patterns", includes("#nested-braces", "$self")},
			}},
			{"nested-braces", jsonObject{
				{"begin", `\{`}, {"end", `\}`},
				{"beginCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.section.braces.begin.tr"}}}}},
				{"endCaptures", jsonObject{{"0", jsonObject{{"name", "punctuation.section.braces.end.tr"}}}}},
				{"patterns", includes("#nested-braces", "$self")},
			}},
			{"import", jsonObject{{"patterns", []jsonObject{
				{{"match", `\b(` + inner(name(codes.A070)) + `)\b\s*("[^"]*"|'[^']*'|[^\W\d][\w.]*)\s+(` + inner(name(codes.A100)) + `)\b\s*([^\W\d]\w*)?`},
					{"captures", captures("keyword.control.import.tr", "string.other.module.tr", "keyword.control.import.as.tr", "entity.name.namespace.tr")}},
				rule("keyword.control.import.tr", wordsPattern(names(codes.A070))),
			}}}},
			{"declarations", jsonObject{{"patterns", []jsonObject{
				{{"match", `\b(` + inner(name(codes.A020)) + `)\s+([^\W\d]\w*)`}, {"captures", captures("storage.type.function.tr", "entity.name.function.tr")}},
				{{"match", `\b(` + inner(name(codes.A050)) + `)\s+([^\W\d]\w*)`}, {"captures", captures("storage.type.struct.tr", "entity.name.type.tr")}},
			}}}},
			{"keywords", jsonObject{{"patterns", []jsonObject{
				rule("keyword.control.tr", wordsPattern(control)),
				rule("keyword.other.debugger.tr", wordsPattern(names(codes.A090))),
				rule("storage.type.tr", wordsPattern(declaration)),
				rule("variable.language.self.tr", wordsPattern(names(codes.A051))),
				rule("constant.language.boolean.tr", wordsPattern(names(codes.A040, codes.A041))),
				rule("constant.language.null.tr", wordsPattern(names(codes.A042))),
				rule("keyword.operator.logical.tr", wordsPattern(names(codes.A043, codes.A044, codes.A045))),
			}}}},
			{"pronoun", jsonObject{
				{"comment", "Trailing-block (o-lambda) parameter: liste.filtrele { o > 2 }"},
				{"patterns", []jsonObject{rule("variable.language.pronoun.tr", `(?<![\w.])o(?!\w)`)}},
			}},
			{"types", jsonObject{{"patterns", []jsonObject{rule("entity.name.type.tr", wordsPattern(types))}}}},
			{"builtins", jsonObject{{"patterns", []jsonObject{
				rule("support.function.builtin.tr", wordsPattern(general)),
				rule("support.function.ai.tr", wordsPattern(ai)),
			}}}},
			{"numbers", jsonObject{{"patterns", []jsonObject{
				rule("constant.numeric.float.tr", `\b\d[\d_]*(?:\.\d[\d_]*)?[eE][+-]?\d+\b`),
				rule("constant.numeric.float.tr", `\b\d[\d_]*\.\d[\d_]*\b`),
				rule("constant.numeric.integer.tr", `\b\d[\d_]*\b`),
			}}}},
			{"operators", jsonObject{{"patterns", []jsonObject{
				rule("keyword.operator.range.tr", `\.\.=?`),
				rule("keyword.operator.pipe.tr", `\|>`),
				rule("keyword.operator.null-coalescing.tr", `\?\?`),
				rule("punctuation.accessor.optional.tr", `\?\.`),
				rule("keyword.operator.arrow.tr", `=>`),
				rule("keyword.operator.comparison.tr", `==|!=|<=|>=|<|>`),
				rule("keyword.operator.logical.tr", `&&|\|\||!`),
				rule("keyword.operator.assignment.compound.tr", `\+=|-=|\*=|/=`),
				rule("keyword.operator.assignment.tr", `:=|=`),
				rule("keyword.operator.arithmetic.tr", `\*\*|@|[+\-*/%]`),
				rule("keyword.operator.ternary.tr", `\?`),
			}}}},
		}},
	}

	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(grammar); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
