package turkce

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var konumRe = regexp.MustCompile(`Satır (\d+)(?:, Sütun (\d+))?`)
var tirnakliAdRe = regexp.MustCompile(`'([^'\s]+)'`)

// KodCercevesi hata mesajındaki "Satır N, Sütun M" konumunu kaynak kodda gösterir
// (Rust/Elm tarzı). Sütun biliniyorsa hatalı kısmın altını ^^^ ile çizer; mesajda
// tırnak içinde geçen ad o konumdaysa onun uzunluğu kadar. Konum bulunamazsa "" döner.
//
//	12 | yazdır(toplm + 1)
//	   |        ^^^^^
func KodCercevesi(kaynak, mesaj string) string {
	m := konumRe.FindStringSubmatch(mesaj)
	if m == nil || kaynak == "" {
		return ""
	}
	satirNo, _ := strconv.Atoi(m[1])
	satirlar := strings.Split(strings.ReplaceAll(kaynak, "\r\n", "\n"), "\n")
	if satirNo < 1 || satirNo > len(satirlar) {
		return ""
	}
	satir := strings.TrimRight(satirlar[satirNo-1], "\r")
	if strings.TrimSpace(satir) == "" {
		return ""
	}
	numara := strconv.Itoa(satirNo)
	bosluk := strings.Repeat(" ", len(numara))
	var sb strings.Builder
	fmt.Fprintf(&sb, " %s | %s\n", numara, satir)

	if m[2] == "" {
		return sb.String()
	}
	sutun, _ := strconv.Atoi(m[2])
	runes := []rune(satir)
	if sutun < 1 || sutun > len(runes)+1 {
		return sb.String()
	}
	uzunluk := 1
	if t := tirnakliAdRe.FindStringSubmatch(mesaj); t != nil {
		ad := []rune(t[1])
		if sutun-1+len(ad) <= len(runes) && string(runes[sutun-1:sutun-1+len(ad)]) == t[1] {
			uzunluk = len(ad)
		}
	}
	// Sekmeler hizayı bozmasın diye işaretçinin önündeki boşluk kaynak satırdan kopyalanır
	var onek strings.Builder
	for _, r := range runes[:sutun-1] {
		if r == '\t' {
			onek.WriteRune('\t')
		} else {
			onek.WriteRune(' ')
		}
	}
	fmt.Fprintf(&sb, " %s | %s%s\n", bosluk, onek.String(), strings.Repeat("^", uzunluk))
	return sb.String()
}
