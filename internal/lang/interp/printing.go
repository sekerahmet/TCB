package interp

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Değerin metni (3.14): TR_String(x), TR_print, metin yerleştirmesi ve Inspect aynı metni verir;
// hiçbir dosyanın ve kişinin diline bağlı değildir. Dilin değerleri ve tipleri kimliğin adıyla
// (codes.ValueNames: true, none, inf, Int), yerleşiklerin ve kitaplık nesnelerinin metni sabit
// İngilizce katalogla (messages.ValueLanguage: <builtin print>, Progress(step 0/2)) yazılır.
// Kişinin dilinde yalnız iletiler, kabuk ve hata ayıklayıcı görünür (DisplayText, Shown).

// LanguageText is a value whose text depends on a writing language: the language's value and
// type names (doğru, yok, Tam, sonsuz), names of builtins, messages inside the text. Text(lang)
// is its text in a file written in lang (3). The program's text of a value (valueText, textOf)
// asks for it in the file's language; Inspect is the canonical (core values) or display (engine
// values, functions) text. Engine values (tensor, Günlük ...) implement it too.
type LanguageText interface {
	Text(lang *codes.Language) string
}

var (
	_ LanguageText = (*BoolVal)(nil)
	_ LanguageText = (*NullVal)(nil)
	_ LanguageText = (*FloatVal)(nil)
	_ LanguageText = (*TypeVal)(nil)
	_ LanguageText = (*FuncVal)(nil)
	_ LanguageText = (*BuiltinVal)(nil)
	_ LanguageText = (*StructDefVal)(nil)
)

// textOf val'ın şu an çalışan koddaki metnidir: program bir değeri işlemin yazıldığı dosyanın
// dilinde metne çevirir (3, 17.1). Programın değerden metin yaptığı her yer bunu kullanır
// (metin(), yazdır, yerleştirme, Metin ile '+', birleştir, dosya_yaz, girdi'nin istemi, metin
// metotlarının argümanları). Veri biçimleri (JSON, günlük, kontrol noktası) ve iletiler
// kullanmaz: onlar Inspect'tedir.
func (v *VM) textOf(val Value) string { return valueText(val, v.textLanguage()) }

// DisplayText is v's display text, in the person's writing language (TR_DIL): the REPL's echo,
// the debugger, the İzleme Masası and help text show it. The program's own text of a value
// follows the language of its file instead (textOf); Inspect is canonical.
func DisplayText(v Value) string { return valueText(v, codes.Display()) }

// DisplayType is the name of v's type in the person's writing language (TR_DIL); for a struct
// instance, the struct's name.
func DisplayType(v Value) string {
	if t := TypeOfValue(v); t != nil {
		return t.Text(codes.Display())
	}
	return string(v.Type())
}

// valueText v'nin lang listesiyle yazılmış metnidir: program codes.ValueNames ile (değerin metni,
// Inspect'le aynı), kişiye gösterilen yerler (DisplayText, iletiler) kişinin diliyle yazar; lang nil
// ise codes.ValueNames.
func valueText(v Value, lang *codes.Language) string {
	return writeItem(v, lang, nil)
}

// orValueNames lang'dir; nil ise değerin metninin listesi (codes.ValueNames).
func orValueNames(lang *codes.Language) *codes.Language {
	if lang == nil {
		return codes.ValueNames()
	}
	return lang
}

// writeContainer bir kabı (Liste, Sözlük, yapı nesnesi) lang dilinde (nil: değerin metni) yazdırır.
// filePath, şu an yazdırılmakta olan dış kaplardır: kendini (doğrudan ya da dolaylı) içeren bir
// kap yeniden görülünce sonsuz özyineleme yerine "[...]" / "{...}" / "Ad{...}" yazılır.
func writeContainer(v Value, lang *codes.Language, filePath []Value) string {
	for _, u := range filePath {
		if u == v {
			switch x := v.(type) {
			case *ListVal:
				return "[...]"
			case *StructInstanceVal:
				return structCycleText(x, lang)
			}
			return "{...}"
		}
	}
	filePath = append(filePath, v)
	var sb strings.Builder
	switch x := v.(type) {
	case *ListVal:
		sb.WriteByte('[')
		for i, el := range x.Elements {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(writeElement(el, lang, filePath))
		}
		sb.WriteByte(']')
	case *DictVal:
		sb.WriteByte('{')
		for i, k := range x.Order {
			if i > 0 {
				sb.WriteString(", ")
			}
			if kv, ok := x.KeyVals[k]; ok {
				sb.WriteString(writeItem(kv, lang, filePath))
			} else {
				sb.WriteString(QuoteText(k))
			}
			sb.WriteString(": ")
			sb.WriteString(writeElement(x.Pairs[k], lang, filePath))
		}
		sb.WriteByte('}')
	case *StructInstanceVal:
		writeStruct(&sb, x, lang, filePath)
	}
	return sb.String()
}

// shownName içerideki bir adın (kimlikse: TR_mem_length) lang'deki yazımıdır; kullanıcının adı
// olduğu gibi.
func shownName(name string, lang *codes.Language) string {
	if !codes.IsReservedName(name) {
		return name
	}
	k, err := codes.Parse(name)
	if err != nil {
		return name
	}
	if e, ok := orValueNames(lang).Entry(k); ok {
		return e.Name
	}
	return name
}

// writeElement liste ve sözlükteki bir değerin metnidir (3.14): metin tırnaklı ve kaçışlı, JSON değeri
// JSON metniyle (metin türü de tırnaklı: [1, "a"] ile [1, a] ayrılsın), öbürleri writeItem.
func writeElement(v Value, lang *codes.Language, filePath []Value) string {
	switch x := v.(type) {
	case *StringVal:
		return QuoteText(x.Val)
	case *JSONVal:
		return x.JSONText()
	}
	return writeItem(v, lang, filePath)
}

// QuoteText s'nin programdaki metin yazımıdır (2.6, 3.14): çift tırnak içinde, kaçışlar \n \t \r \"
// \\; öbür denetim karakterleri ve yön değiştiren karakterler (2.1a) \u{...} ile. Çıktı programa
// kopyalanınca aynı metni verir.
func QuoteText(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\n':
			sb.WriteString(`\n`)
		case '\t':
			sb.WriteString(`\t`)
		case '\r':
			sb.WriteString(`\r`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			if unicode.IsControl(r) || lexer.IsBidiControl(r) {
				fmt.Fprintf(&sb, `\u{%x}`, r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// writeItem bir kabın elemanını lang dilinde (nil: değerin metni) yazar; eleman da bir kapsa döngü
// denetimi sürer.
func writeItem(v Value, lang *codes.Language, filePath []Value) string {
	switch x := v.(type) {
	case *StringVal:
		return x.Val // sık tipler arayüz denetiminden önce (yerleştirmenin sıcak yolu)
	case *IntVal:
		return x.Inspect()
	case *BoolVal:
		if lang == nil {
			return x.Inspect()
		}
		return x.Text(lang)
	case *FloatVal:
		if lang == nil {
			return x.Inspect()
		}
		return x.Text(lang)
	case *ListVal, *DictVal, *StructInstanceVal:
		return writeContainer(v, lang, filePath)
	case LanguageText:
		return x.Text(orValueNames(lang))
	}
	return v.Inspect()
}
