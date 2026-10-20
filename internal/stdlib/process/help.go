package process

import (
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// helpText --yardım çıktısıdır: kullanım satırı, programın açıklaması ve her seçeneğin
// tipi, açıklaması ve varsayılanı.
//
//	Kullanım: tr egit.tr [seçenekler]
//
//	Küçük dil modelini eğitir
//
//	Seçenekler:
//	  --öğrenme_oranı <Ondalık>   AdamW öğrenme oranı (varsayılan: 0.001)
//	  --hızlı                     (varsayılan: yanlış)
//	  --yardım, -h                bu yardımı gösterir
func helpText(command, description string, options []*option) string {
	var b strings.Builder
	b.WriteString(messages.Text(messages.H7232, command) + "\n")
	if description != "" {
		b.WriteString("\n" + description + "\n")
	}
	b.WriteString("\n" + messages.Text(messages.H7233) + "\n")
	leftText := make([]string, len(options)+1)
	width := 0
	for i, s := range options {
		leftText[i] = "--" + s.name
		if s.optKind != kindBool {
			leftText[i] += " <" + typeText(s) + ">"
		}
		width = max(width, utf8.RuneCountInString(leftText[i]))
	}
	helpLeft := strings.Join(helpOptions, ", ")
	leftText[len(options)] = helpLeft
	width = max(width, utf8.RuneCountInString(helpLeft))
	line := func(left, right string) {
		b.WriteString("  " + left + strings.Repeat(" ", width-utf8.RuneCountInString(left)+3) + right + "\n")
	}
	for i, s := range options {
		right := messages.Text(messages.H7234, defaultText(s.defaultVal))
		if s.description != "" {
			right = s.description + " " + right
		}
		line(leftText[i], right)
	}
	line(helpLeft, messages.Text(messages.H7235))
	return strings.TrimSuffix(b.String(), "\n")
}

// typeText seçeneğin yardımdaki tipidir: <Tam>, <Metin>, <Tam,Tam,...>.
func typeText(s *option) string {
	if s.optKind == kindList {
		t := codes.Name(s.itemKind.typeCode())
		return t + "," + t + ",..."
	}
	return codes.Name(s.optKind.typeCode())
}

// defaultText varsayılan değerin komut satırında yazılacağı biçimidir.
func defaultText(v interp.Value) string {
	switch x := v.(type) {
	case *interp.StringVal:
		if x.Val == "" {
			return `""`
		}
		return x.Val
	case *interp.ListVal:
		parts := make([]string, len(x.Elements))
		for i, e := range x.Elements {
			parts[i] = defaultText(e)
		}
		if len(parts) == 0 {
			return messages.Text(messages.H7236)
		}
		return strings.Join(parts, ",")
	}
	return v.Inspect()
}
