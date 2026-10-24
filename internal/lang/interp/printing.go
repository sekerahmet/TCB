package interp

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Değerin metni (3, 17.1): metin(), yazdır ve metin yerleştirmesi bir değeri işlemin yazıldığı
// dosyanın dilinde metne çevirir (valueText). Dile bağlı olan yalnızca dilin değerlerinin adlarıdır
// (doğru, yanlış, yok ve tip adları; LanguageText), iç içe kapların içindekiler dahil. Inspect
// kanonik (Türkçe) kalır: veri biçimleri (JSON, günlük) ve iletiler onu kullanır.

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

// valueText v'nin lang dilinde yazılmış bir dosyadaki metnidir; lang nil kanonik dildir. Dilin
// sözcüğü olan değerler (LanguageText) her zaman bir dilde çevrilir: fonksiyon, yerleşik ve yapı
// değerlerinin Inspect'i görüntüdür (TR_DIL), program metni ondan alınmaz. Aynı dili birden çok değer için bir kez soran çağıranlar (yazdır, birleştir,
// yerleştirme) bunu kullanır.
func valueText(v Value, lang *codes.Language) string {
	return writeItem(v, lang, nil)
}

// writeContainer bir kabı (Liste, Sözlük, yapı nesnesi) lang dilinde (nil: kanonik) yazdırır.
// filePath, şu an yazdırılmakta olan dış kaplardır: kendini (doğrudan ya da dolaylı) içeren bir
// kap yeniden görülünce sonsuz özyineleme yerine "[...]" / "{...}" / "Ad{...}" yazılır.
func writeContainer(v Value, lang *codes.Language, filePath []Value) string {
	for _, u := range filePath {
		if u == v {
			switch x := v.(type) {
			case *ListVal:
				return "[...]"
			case *StructInstanceVal:
				return x.Def.Name + "{...}"
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
			if el.Type() == ValString {
				fmt.Fprintf(&sb, "%q", el.Inspect())
			} else {
				sb.WriteString(writeItem(el, lang, filePath))
			}
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
				fmt.Fprintf(&sb, "%q", k)
			}
			sb.WriteString(": ")
			val := x.Pairs[k]
			if val.Type() == ValString {
				fmt.Fprintf(&sb, "%q", val.Inspect())
			} else {
				sb.WriteString(writeItem(val, lang, filePath))
			}
		}
		sb.WriteByte('}')
	case *StructInstanceVal:
		sb.WriteString(x.Def.Name)
		sb.WriteByte('{')
		first := true
		for _, f := range x.Def.Fields {
			if val, ok := x.Fields[f.Name]; ok {
				if !first {
					sb.WriteString(", ")
				}
				first = false
				sb.WriteString(f.Name)
				sb.WriteString(": ")
				sb.WriteString(writeItem(val, lang, filePath))
			}
		}
		sb.WriteByte('}')
	}
	return sb.String()
}

// writeItem bir kabın elemanını lang dilinde (nil: kanonik) yazar; eleman da bir kapsa döngü
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
		if lang == nil {
			lang = codes.Active()
		}
		return x.Text(lang)
	}
	return v.Inspect()
}
