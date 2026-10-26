// codesdoc, dilin bütün kodlarının açıklamalı başvuru belgesini (docs/codes.md) kod
// listesinden üretir: dilden bağımsız tanımlar (internal/lang/codes/definitions.json: tür, tipler, imza,
// ilişkili kodlar) ve Türkçe dil dosyası (internal/lang/codes/tr.json: ad, yabancı adlar,
// açıklama, örnek). Belge elle düzenlenmez; bir kod eklendiğinde ya da değiştiğinde
// yeniden üretilir.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/gen/codesdoc           // dosyayı yeniden yazar
//	go run ./tools/gen/codesdoc -check    // güncel değilse 1 koduyla çıkar
//
// Aynı denetim `go test ./tools/gen/...` ile de yapılır.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// docPath depo köküne göre üretilen dosyadır.
var docPath = filepath.Join("docs", "codes.md")

func main() {
	genmain.RunMain("codesdoc", docPath, func(string) ([]byte, error) { return Generate() })
}

// section bir bölümün kısmıdır: kimliklerinin sınıfı Class olan kodlar (TR_fn_... için fn).
type section struct {
	Class codes.Class `json:"class"`
	Title string      `json:"title"`
}

// family belgenin bir bölümüdür; bölümler ve kısımlar kod listesinin sınıf sırasındadır.
type family struct {
	Title    string    `json:"title"`
	Sections []section `json:"sections"`
}

// docText belgenin Türkçe metnidir (text.json): başlıklar, giriş, etiketler. Go metninde
// Türkçe bulunmaz; üreteç yalnızca yer tutucuları doldurur.
type docText struct {
	Title         string            `json:"title"`
	GeneratedNote string            `json:"generatedNote"`
	Intro         []string          `json:"intro"`
	CountsFormat  string            `json:"countsFormat"`
	Contents      string            `json:"contents"`
	PipelineTitle string            `json:"pipelineTitle"`
	PipelineIntro []string          `json:"pipelineIntro"`
	Labels        map[string]string `json:"labels"`
	Families      []family          `json:"families"`
}

//go:embed text.json
var textJSON []byte

// text belgenin metnidir; text.json bozuksa üreteç çalışamaz.
var text = func() docText {
	var t docText
	if err := json.Unmarshal(textJSON, &t); err != nil {
		panic("text.json: " + err.Error())
	}
	return t
}()

// typeName bir tip ifadesini adlarla yazar: "Tam | Ondalık".
func typeName(t codes.TypeExpr) string {
	if t.Empty() {
		return text.Labels["anyType"]
	}
	parts := make([]string, len(t.Types))
	for i, k := range t.Types {
		parts[i] = codes.Source().Name(k)
	}
	return strings.Join(parts, " | ")
}

// defaultValue bir adlı argümanın varsayılanını TR yazımıyla verir.
func defaultValue(v any) string {
	switch x := v.(type) {
	case nil:
		return codes.Source().Name(codes.KwNone) // yok
	case bool:
		if x {
			return codes.Source().Name(codes.KwTrue)
		}
		return codes.Source().Name(codes.KwFalse)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return strconv.Quote(x)
	}
	return fmt.Sprint(v)
}

// arguments imzanın parantez içini yazar: "Tam, [Tam], adlı: Tip = varsayılan".
func arguments(im codes.Signature) string {
	var parts []string
	for _, p := range im.Params {
		s := typeName(p)
		switch {
		case p.Optional:
			s = "[" + s + "]"
		case p.Many:
			s = "[" + s + "...]"
		case p.AtLeastOne:
			s = s + "..."
		}
		parts = append(parts, s)
	}
	for _, a := range im.Named {
		s := codes.Source().Name(a.Code) + ": " + typeName(a.Type)
		if a.HasDefault {
			s += " = " + defaultValue(a.Default)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func returns(im codes.Signature) string {
	if im.Return.Empty() {
		return ""
	}
	return " → " + typeName(im.Return)
}

// signatureLines bir kodun imzasını adlarla yazar (üyelerde tip başına bir satır).
func signatureLines(t codes.Definition) []string {
	name := codes.Source().Name(t.Code)
	switch t.Kind {
	case codes.KindFunction:
		return []string{name + "(" + arguments(t.Signature) + ")" + returns(t.Signature)}
	case codes.KindConstant:
		return []string{name + returns(t.Signature)}
	case codes.KindMethod, codes.KindProperty:
		var out []string
		for _, u := range t.Members {
			s := codes.Source().Name(u.Type) + "." + name
			if t.Kind == codes.KindMethod {
				s += "(" + arguments(u.Signature) + ")"
			}
			out = append(out, s+returns(u.Signature))
		}
		return out
	}
	return nil
}

// codeLabel "TR_fn_sum `toplam`" biçimidir.
func codeLabel(k codes.Code) string { return fmt.Sprintf("%s `%s`", k, codes.Source().Name(k)) }

// Generate belgenin içeriğini üretir.
func Generate() ([]byte, error) {
	if errs := codes.CheckDefinitions(codes.Source()); len(errs) > 0 {
		return nil, fmt.Errorf("definitions are inconsistent: %v", errs[0])
	}
	d := codes.Source()
	allCodes := d.Codes()

	// Ters bağlar: adlı argümanı kullananlar, tipin üyeleri.
	usedBy := map[codes.Code][]codes.Code{}
	typeMembers := map[codes.Code][]codes.Code{}
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		signatures := []codes.Signature{t.Signature}
		for _, u := range t.Members {
			signatures = append(signatures, u.Signature)
			typeMembers[u.Type] = append(typeMembers[u.Type], k)
		}
		seen := map[codes.Code]bool{}
		for _, im := range signatures {
			for _, a := range im.Named {
				if !seen[a.Code] {
					seen[a.Code] = true
					usedBy[a.Code] = append(usedBy[a.Code], k)
				}
			}
		}
	}
	list := func(ks []codes.Code) string {
		parts := make([]string, len(ks))
		for i, k := range ks {
			parts[i] = codeLabel(k)
		}
		return strings.Join(parts, ", ")
	}

	var b bytes.Buffer
	b.WriteString("# " + text.Title + "\n\n")
	b.WriteString("<!-- " + text.GeneratedNote + " -->\n\n")
	for _, para := range text.Intro {
		b.WriteString(para + "\n\n")
	}
	counts := map[codes.Kind]int{}
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		counts[t.Kind]++
	}
	fmt.Fprintf(&b, text.CountsFormat+"\n\n",
		len(d.Codes()), counts[codes.KindKeyword], counts[codes.KindContextual], counts[codes.KindValueName], counts[codes.KindType], counts[codes.KindFunction],
		counts[codes.KindConstant], counts[codes.KindMethod], counts[codes.KindProperty], counts[codes.KindNamedArg])
	b.WriteString("## " + text.Contents + "\n\n")
	for _, a := range text.Families {
		fmt.Fprintf(&b, "- %s: ", a.Title)
		var parts []string
		for _, sec := range a.Sections {
			parts = append(parts, sec.Title)
		}
		b.WriteString(strings.Join(parts, " · ") + "\n")
	}
	b.WriteString("- " + text.PipelineTitle + "\n\n")

	for _, a := range text.Families {
		fmt.Fprintf(&b, "## %s\n\n", a.Title)
		for _, sec := range a.Sections {
			var ks []codes.Code
			for _, k := range allCodes {
				if k.Class() == sec.Class {
					ks = append(ks, k)
				}
			}
			if len(ks) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", sec.Title)
			for _, k := range ks {
				g, _ := d.Entry(k)
				t, _ := codes.DefinitionOf(k)
				fmt.Fprintf(&b, "#### %s `%s`\n\n", k, g.Name)
				if im := signatureLines(t); len(im) > 0 {
					for _, s := range im {
						fmt.Fprintf(&b, "    %s\n", s)
					}
					b.WriteString("\n")
				}
				b.WriteString(g.Description + "\n\n")
				if g.Example != "" {
					fmt.Fprintf(&b, "```tr\n%s\n```\n\n", g.Example)
				}
				var extras []string
				if t.Kind == codes.KindNamedArg && len(usedBy[k]) > 0 {
					extras = append(extras, text.Labels["usedBy"]+": "+list(usedBy[k]))
				}
				if t.Kind == codes.KindType && len(typeMembers[k]) > 0 {
					extras = append(extras, text.Labels["members"]+": "+list(typeMembers[k]))
				}
				if len(t.Related) > 0 {
					extras = append(extras, text.Labels["related"]+": "+list(t.Related))
				}
				if len(extras) > 0 {
					b.WriteString(strings.Join(extras, " · ") + "\n\n")
				}
			}
		}
	}
	// Boru hattı (x |> f, docs/spec/04_operatorler.md 4.9): ilk konumlu parametresi borudaki değeri alan
	// fonksiyonlar. Metotlar boruyla değil '.' ile zincirlenir.
	b.WriteString("## " + text.PipelineTitle + "\n\n")
	for _, para := range text.PipelineIntro {
		b.WriteString(para + "\n\n")
	}
	var pipeable []string
	for _, k := range d.Codes() {
		t, _ := codes.DefinitionOf(k)
		if t.Kind == codes.KindFunction && len(t.Signature.Params) > 0 {
			pipeable = append(pipeable, "`"+codes.Source().Name(k)+"`")
		}
	}
	b.WriteString(strings.Join(pipeable, ", ") + "\n")
	return append(bytes.TrimRight(b.Bytes(), "\n"), '\n'), nil
}
