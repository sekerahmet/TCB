package debugger

import "strings"

// boxInner başlık kutusunun iç genişliğidir (sütun).
const boxInner = 78

// boxRow kutunun bir satırıdır: içerik kenarlıkla aynı genişliğe boşlukla tamamlanır, sığmazsa "…"
// ile kırpılır. Genişlik görüntü sütunudur (displayWidth): iletinin dili ve yolun uzunluğu sağ kenarı kaydırmaz.
func boxRow(content string) string {
	room := boxInner - 2 // içeriğin iki yanındaki birer boşluk
	if displayWidth(content) > room {
		r := []rune(content)
		for len(r) > 0 && displayWidth(string(r))+1 > room {
			r = r[:len(r)-1]
		}
		content = string(r) + "…"
	}
	return "║ " + content + strings.Repeat(" ", room-displayWidth(content)) + " ║"
}

// displayWidth metnin terminaldeki sütun sayısıdır: emoji iki, değişken seçici (U+FE0F) sıfır,
// öbür harfler bir sütun.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r == 0xFE0F:
		case r >= 0x1F000:
			w += 2
		default:
			w++
		}
	}
	return w
}
