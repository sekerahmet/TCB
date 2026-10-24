package interp

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Değerin metni (3, 17.1): metin(), yazdır ve metin yerleştirmesi bir değeri işlemin yazıldığı
// dosyanın dilinde metne çevirir (valueText). Dile bağlı olan yalnızca dilin değerlerinin adlarıdır
// (doğru, yanlış, yok ve tip adları; languageText), iç içe kapların içindekiler dahil. Inspect
// kanonik (Türkçe) kalır: veri biçimleri (JSON, günlük) ve iletiler onu kullanır.

// languageText adı dilin bir sözcüğü olan değerlerdir: metinleri bir yazım dilinde verilir.
type languageText interface {
	text(lang *codes.Language) string
}

// textOf val'ın şu an çalışan koddaki metnidir: program bir değeri işlemin yazıldığı dosyanın
// dilinde metne çevirir (3, 17.1). Programın değerden metin yaptığı her yer bunu kullanır
// (metin(), yazdır, yerleştirme, Metin ile '+', birleştir, dosya_yaz, girdi'nin istemi, metin
// metotlarının argümanları). Veri biçimleri (JSON, günlük, kontrol noktası) ve iletiler
// kullanmaz: onlar Inspect'tedir.
func (v *VM) textOf(val Value) string { return valueText(val, v.textLanguage()) }

// valueText v'nin lang dilinde yazılmış bir dosyadaki metnidir; lang nil kanonik dildir. Dilin
// sözcüğü olan değerler (languageText) her zaman bir dilde çevrilir: fonksiyon, yerleşik ve yapı
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
	case *ListVal, *DictVal, *StructInstanceVal:
		return writeContainer(v, lang, filePath)
	case languageText:
		if lang == nil {
			lang = codes.Active()
		}
		return x.text(lang)
	}
	return v.Inspect()
}
