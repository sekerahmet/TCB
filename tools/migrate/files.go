package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Dosya türlerinin çevirisi: .tr dosyası, belgedeki ```tr bloğu ve dil dosyasının örnekleri.

const (
	// oldHeader eski kodlu biçimin başlığıdır (ardından dosyanın dili gelir); oldHeaderPrefix
	// başka sürümleri de tanır (bilerek bozuk fikstür).
	oldHeader       = "//! tr kodlu 3 dil="
	oldHeaderPrefix = "//! tr kodlu"
	// newHeader yeni başlıktır (2.1); newHeaderPrefix başka sürümleri de tanır.
	newHeader       = "# TR 4"
	newHeaderPrefix = "# TR "
	bom             = "\ufeff"
)

// removedJSON kalkan anahtar kelimelerin eski gösterim yazımlarıdır (dil → kimlik → yazım); kod
// listesinden silindikleri için burada durur.
//
//go:embed removed.json
var removedJSON []byte

// viewOf dilin gösterim sözcükleridir (Türkçe ya da İngilizce gösterimde yazılmış eski metin için).
func viewOf(langCode string) *view {
	lang, err := codes.LoadLanguage(langCode)
	if err != nil {
		panic(err)
	}
	var removed map[string]map[string]string
	if err := json.Unmarshal(removedJSON, &removed); err != nil {
		panic(err)
	}
	v := &view{lang: lang, removed: map[string]string{}, constWord: lang.Name(codes.KwConst)}
	for id, spelling := range removed[langCode] {
		to := "_"
		if id == "TR_kw_while" {
			to = lang.Name(codes.KwFor)
		}
		v.removed[spelling] = to
	}
	return v
}

// convertTRFile bir .tr dosyasının (ya da belgedeki bloğun) bütün metnini çevirir.
func convertTRFile(rel string, in string) (string, []problem) {
	text, hasBOM := strings.CutPrefix(in, bom)
	prefix := ""
	if hasBOM {
		prefix = bom
	}
	lineBase := 0
	if strings.HasPrefix(text, "#!") {
		first, rest, eol := firstLine(text)
		prefix += first + eol
		text = rest
		lineBase = 1
	}
	first, rest, eol := firstLine(text)
	switch {
	case strings.HasPrefix(first, newHeaderPrefix):
		return convertValues(rel, in) // yeni biçim: yalnız değer kuralları (values.go)
	case strings.HasPrefix(first, oldHeader):
		if _, err := codes.LoadLanguage(strings.TrimPrefix(first, oldHeader)); err != nil {
			return in, []problem{{Line: lineBase + 1, Reason: "unknown language in the old header"}}
		}
		body, probs := convertCode(rest, nil, lineBase+1)
		return prefix + newHeader + eol + body, probs
	case strings.HasPrefix(first, oldHeaderPrefix):
		return in, []problem{{Line: lineBase + 1, Reason: "old header of another format version"}}
	}
	// Başlıksız metin Türkçe gösterimdir (eski 17.7).
	body, probs := convertCode(text, viewOf(codes.SourceLanguage), lineBase)
	coded, lexical, err := encodeView(body, codes.SourceLanguage)
	if err != nil {
		return in, append(probs, problem{Line: lineBase + 1, Reason: "cannot encode the Turkish text: " + err.Error()})
	}
	if lexical {
		probs = append(probs, problem{Line: lineBase + 1, Reason: "syntax error: encoded word by word, review it"})
	}
	return prefix + newHeader + findEOL(body) + coded, probs
}

// convertMarkdown belgedeki ```tr bloklarını convertTRFile ile çevirir; öbür satırlar olduğu gibi kalır.
func convertMarkdown(rel, in string) (string, []problem) {
	lines := strings.SplitAfter(in, "\n")
	var out strings.Builder
	var probs []problem
	for i := 0; i < len(lines); i++ {
		out.WriteString(lines[i])
		if !strings.HasPrefix(lines[i], "```tr") {
			continue
		}
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], "```") {
			j++
		}
		block := strings.Join(lines[i+1:j], "")
		conv, ps := convertTRFile(rel, block)
		for _, p := range ps {
			probs = append(probs, problem{Line: i + 1 + p.Line, Reason: p.Reason})
		}
		out.WriteString(conv)
		i = j - 1
	}
	return out.String(), probs
}

// exampleKey dil dosyasında örnek alanının başıdır.
const exampleKey = `"example": `

// convertExamples dil dosyasındaki her "example" değerinin yazımını çevirir; örnek gösterim metnidir,
// gösterim olarak kalır (dosyanın dili: tr.json Türkçe, en.json İngilizce).
func convertExamples(rel, in string) (string, []problem) {
	v := viewOf(strings.TrimSuffix(rel[strings.LastIndexByte(rel, '/')+1:], ".json"))
	lines := strings.SplitAfter(in, "\n")
	var probs []problem
	for i, l := range lines {
		k := strings.Index(l, exampleKey)
		if k < 0 {
			continue
		}
		start := k + len(exampleKey)
		end := jsonStringEnd(l, start)
		var ex string
		if end < 0 || json.Unmarshal([]byte(l[start:end]), &ex) != nil {
			probs = append(probs, problem{Line: i + 1, Reason: "example is not a JSON string"})
			continue
		}
		if looksNew(ex) {
			continue
		}
		conv, ps := convertCode(ex, v, i)
		probs = append(probs, ps...)
		if conv == ex {
			continue
		}
		lines[i] = l[:start] + jsonQuote(conv) + l[end:]
	}
	return strings.Join(lines, ""), probs
}

// templateKey kod şablonlarında (tools/gen/snippets/data.json) gövde alanının başıdır.
const templateKey = `"source": `

// convertTemplates kod şablonlarının kodlu gövdelerini (tam dosya metni) çevirir.
func convertTemplates(rel, in string) (string, []problem) {
	lines := strings.SplitAfter(in, "\n")
	var probs []problem
	for i, l := range lines {
		k := strings.Index(l, templateKey)
		if k < 0 {
			continue
		}
		start := k + len(templateKey)
		end := jsonStringEnd(l, start)
		var src string
		if end < 0 || json.Unmarshal([]byte(l[start:end]), &src) != nil {
			probs = append(probs, problem{Line: i + 1, Reason: "source is not a JSON string"})
			continue
		}
		conv, ps := convertTRFile(rel, src)
		probs = append(probs, ps...)
		if conv != src {
			lines[i] = l[:start] + jsonQuote(conv) + l[end:]
		}
	}
	return strings.Join(lines, ""), probs
}

// jsonStringEnd s[start]'taki JSON metninin kapanış tırnağından sonraki indekstir (yoksa -1).
func jsonStringEnd(s string, start int) int {
	if start >= len(s) || s[start] != '"' {
		return -1
	}
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

// jsonQuote metni dil dosyalarının yazdığı gibi tırnaklar (HTML kaçışı yok).
func jsonQuote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// firstLine metnin ilk satırını, ondan sonrasını ve aradaki satır sonunu verir.
func firstLine(text string) (first, rest, eol string) {
	i := strings.IndexByte(text, '\n')
	if i < 0 {
		return text, "", ""
	}
	first, eol = text[:i], "\n"
	if strings.HasSuffix(first, "\r") {
		first, eol = first[:len(first)-1], "\r\n"
	}
	return first, text[i+1:], eol
}

// findEOL metnin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
func findEOL(body string) string {
	if i := strings.IndexByte(body, '\n'); i > 0 && body[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}
