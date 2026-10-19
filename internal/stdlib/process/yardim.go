package process

import (
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// yardimMetni --yardım çıktısıdır: kullanım satırı, programın açıklaması ve her seçeneğin
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
func yardimMetni(komut, aciklama string, secenekler []*secenek) string {
	var b strings.Builder
	b.WriteString("Kullanım: " + komut + " [seçenekler]\n")
	if aciklama != "" {
		b.WriteString("\n" + aciklama + "\n")
	}
	b.WriteString("\nSeçenekler:\n")
	solMetin := make([]string, len(secenekler)+1)
	genislik := 0
	for i, s := range secenekler {
		solMetin[i] = "--" + s.ad
		if s.tur != turMantik {
			solMetin[i] += " <" + tipYazimi(s) + ">"
		}
		genislik = max(genislik, utf8.RuneCountInString(solMetin[i]))
	}
	yardimSol := strings.Join(yardimSecenekleri, ", ")
	solMetin[len(secenekler)] = yardimSol
	genislik = max(genislik, utf8.RuneCountInString(yardimSol))
	satir := func(sol, sag string) {
		b.WriteString("  " + sol + strings.Repeat(" ", genislik-utf8.RuneCountInString(sol)+3) + sag + "\n")
	}
	for i, s := range secenekler {
		sag := "(varsayılan: " + varsayilanYazimi(s.varsayilan) + ")"
		if s.aciklama != "" {
			sag = s.aciklama + " " + sag
		}
		satir(solMetin[i], sag)
	}
	satir(yardimSol, "bu yardımı gösterir")
	return strings.TrimSuffix(b.String(), "\n")
}

// tipYazimi seçeneğin yardımdaki tipidir: <Tam>, <Metin>, <Tam,Tam,...>.
func tipYazimi(s *secenek) string {
	if s.tur == turListe {
		t := codes.Name(s.ogeTuru.tipKodu())
		return t + "," + t + ",..."
	}
	return codes.Name(s.tur.tipKodu())
}

// varsayilanYazimi varsayılan değerin komut satırında yazılacağı biçimidir.
func varsayilanYazimi(v interp.Value) string {
	switch x := v.(type) {
	case *interp.StringVal:
		if x.Val == "" {
			return `""`
		}
		return x.Val
	case *interp.ListVal:
		parcalar := make([]string, len(x.Elements))
		for i, e := range x.Elements {
			parcalar[i] = varsayilanYazimi(e)
		}
		if len(parcalar) == 0 {
			return "boş liste"
		}
		return strings.Join(parcalar, ",")
	}
	return v.Inspect()
}
