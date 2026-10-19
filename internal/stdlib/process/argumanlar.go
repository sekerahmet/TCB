package process

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Komut satırı yazımı: --ad değer ya da --ad=değer; Mantık seçenek değersiz yazılınca doğru
// olur (--hızlı), yanlış yalnızca --hızlı=yanlış ile verilir. Konumlu argüman, tek tireli
// seçenek (-h dışında), bilinmeyen ya da iki kez verilen seçenek ve tipine uymayan değer
// programı 2 koduyla bitirir; mesaj en yakın seçeneği önerir. Türkçe harfsiz (--ogrenme_orani)
// ve tireli (--öğrenme-oranı) yazımlar kabul edilmez (tek yazım), doğru ad önerilir.

// argumanSiniri bir argümanın en uzun boyudur (bayt).
const argumanSiniri = 1 << 20

// Kullanıcının yazdığı argümanlardaki hatanın çıkış kodu ("yanlış kullanım").
const kullanimHatasiKodu = 2

// yardimSecenekleri seçenek listesini isteyen argümanlardır.
var yardimSecenekleri = []string{"--yardım", "-h"}

// argumanlariUygula programın argümanlarını okur (okunmuş sayılırlar) ve seçeneklerin
// değerleriyle yeni bir sözlük kurar. --yardım ya da hatalı argüman bir program çıkışı
// isteği (yurut.CikisIstegi) döndürür.
func argumanlariUygula(m *interp.VM, secenekler []*secenek, aciklama string) (*interp.DictVal, error) {
	var argv []string
	komut := "tr"
	if m != nil {
		argv = m.Argumanlar()
		komut = m.ProgramKomutu()
	}
	for _, a := range argv {
		if a == yardimSecenekleri[0] || a == yardimSecenekleri[1] {
			return nil, &interp.CikisIstegi{Kod: 0, Mesaj: yardimMetni(komut, aciklama, secenekler)}
		}
	}
	verilen, err := ayristir(m, argv, secenekler)
	if err != nil {
		mesaj := err.Error()
		if !strings.HasSuffix(mesaj, "?") {
			mesaj += "."
		}
		return nil, &interp.CikisIstegi{Kod: kullanimHatasiKodu,
			Mesaj: fmt.Sprintf("Hata: %s Seçenekler için: %s %s", mesaj, komut, yardimSecenekleri[0])}
	}
	sonuc := interp.NewDictVal()
	for _, s := range secenekler {
		v, ok := verilen[s.ad]
		if !ok {
			v = s.varsayilan
		}
		sonuc.Set(s.ad, v)
	}
	return sonuc, nil
}

// ayristir argümanları seçeneklere göre çözer: seçenek adı → değer.
func ayristir(m *interp.VM, argv []string, secenekler []*secenek) (map[string]interp.Value, error) {
	verilen := map[string]interp.Value{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if len(a) > argumanSiniri {
			return nil, fmt.Errorf("%d. argüman çok uzun (%d bayt; en çok 1 MB)", i+1, len(a))
		}
		ad, deger, degerli, err := seceneginAdi(a, secenekler)
		if err != nil {
			return nil, err
		}
		if degerli && GizliAdMi(ad) && m != nil {
			gizliEkle(m, deger) // hata mesajında da görünmesin
		}
		s := bul(secenekler, ad)
		if s == nil {
			return nil, bilinmeyenSecenek("--"+ad, secenekler)
		}
		if _, once := verilen[s.ad]; once {
			return nil, fmt.Errorf("'--%s' iki kez verildi", s.ad)
		}
		if !degerli && s.tur != turMantik {
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "--") {
				return nil, fmt.Errorf("'--%s' bir değer bekler (ör. --%s %s ya da --%s=%s)", s.ad, s.ad, ornekDeger(s), s.ad, ornekDeger(s))
			}
			i++
			deger, degerli = argv[i], true
			if len(deger) > argumanSiniri {
				return nil, fmt.Errorf("'--%s' değeri çok uzun (%d bayt; en çok 1 MB)", s.ad, len(deger))
			}
			if s.gizli && m != nil {
				gizliEkle(m, deger)
			}
		}
		v, err := degerCoz(s, deger, degerli)
		if err != nil {
			return nil, err
		}
		verilen[s.ad] = v
	}
	return verilen, nil
}

// seceneginAdi bir argümanı seçenek adına ve (--ad=değer biçiminde) değerine ayırır;
// argüman bir seçenek değilse kullanıcıya yol gösteren hatayı verir.
func seceneginAdi(a string, secenekler []*secenek) (ad, deger string, degerli bool, err error) {
	switch {
	case a == "--":
		return "", "", false, fmt.Errorf("'--' tek başına kullanılmaz; seçenekler --ad değer biçiminde yazılır")
	case strings.HasPrefix(a, "--"):
		ad = a[2:]
		if i := strings.IndexByte(ad, '='); i >= 0 {
			ad, deger, degerli = ad[:i], ad[i+1:], true
		}
		return ad, deger, degerli, nil
	case strings.HasPrefix(a, "-") && len(a) > 1:
		e := bilinmeyenSecenek(a, secenekler)
		return "", "", false, fmt.Errorf("%v; seçenekler iki tireyle yazılır (--ad değer)", e)
	}
	return "", "", false, fmt.Errorf("'%s' bir seçenek değil: argümanlar --ad değer biçiminde verilir (konumlu argüman yoktur)", kisalt(a))
}

func bul(secenekler []*secenek, ad string) *secenek {
	for _, s := range secenekler {
		if s.ad == ad {
			return s
		}
	}
	return nil
}

// bilinmeyenSecenek en yakın seçeneği öneren hatayı verir; verilen, argümanın yazıldığı
// biçimdir (--ogrenme_orani, -adım).
func bilinmeyenSecenek(verilen string, secenekler []*secenek) error {
	ad := strings.TrimLeft(verilen, "-")
	if i := strings.IndexByte(ad, '='); i >= 0 {
		ad = ad[:i]
	}
	adaylar := []string{"yardım"}
	for _, s := range secenekler {
		adaylar = append(adaylar, s.ad)
	}
	o := turkish.EnYakin(ad, adaylar)
	if o == "" && bul(secenekler, ad) != nil {
		o = ad // -adım → --adım
	}
	if o == "" && (ad == "help" || ad == "h") {
		o = "yardım"
	}
	if o == "" {
		return fmt.Errorf("bilinmeyen seçenek '%s'", kisalt(verilen))
	}
	return fmt.Errorf("bilinmeyen seçenek '%s'. Bunu mu demek istediniz: '--%s'?", kisalt(verilen), o)
}

// degerCoz bir argüman değerini seçeneğin tipine çevirir.
func degerCoz(s *secenek, deger string, degerli bool) (interp.Value, error) {
	if s.tur == turMantik {
		if !degerli {
			return interp.TRUE, nil
		}
		switch deger {
		case codes.Ad(codes.A040):
			return interp.TRUE, nil
		case codes.Ad(codes.A041):
			return interp.FALSE, nil
		}
		return nil, fmt.Errorf("'--%s' %s bir seçenek; %s ya da %s verin (yalnız --%s yazmak %s demektir), %q verildi",
			s.ad, codes.Ad(codes.A203), codes.Ad(codes.A040), codes.Ad(codes.A041), s.ad, codes.Ad(codes.A040), kisalt(deger))
	}
	if s.tur != turListe {
		v, err := duzDegerCoz(s.tur, deger)
		if err != nil {
			return nil, fmt.Errorf("'--%s' %s bekler, %q verildi%s", s.ad, codes.Ad(s.tur.tipKodu()), kisalt(deger), err.Error())
		}
		return v, nil
	}
	liste := &interp.ListVal{}
	if deger == "" {
		return liste, nil
	}
	for i, p := range strings.Split(deger, ",") {
		v, err := duzDegerCoz(s.ogeTuru, strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("'--%s' virgülle ayrılmış %s listesi bekler; %d. öğe %q%s", s.ad, codes.Ad(s.ogeTuru.tipKodu()), i+1, kisalt(p), err.Error())
		}
		liste.Elements = append(liste.Elements, v)
	}
	return liste, nil
}

// duzDegerCoz Tam, Ondalık, Mantık ya da Metin bir değeri çözer. Hata metni, mesaja eklenecek
// açıklamadır ("; ondalık ayırıcı nokta olmalı: 0.001" gibi; boş olabilir).
func duzDegerCoz(t tur, s string) (interp.Value, error) {
	switch t {
	case turTam:
		n, err := tamCoz(s)
		if err != nil {
			return nil, err
		}
		return interp.NewInt(n), nil
	case turOndalik:
		f, err := ondalikCoz(s)
		if err != nil {
			return nil, err
		}
		return &interp.FloatVal{Val: f}, nil
	case turMantik:
		switch s {
		case codes.Ad(codes.A040):
			return interp.TRUE, nil
		case codes.Ad(codes.A041):
			return interp.FALSE, nil
		}
		return nil, fmt.Errorf(" (%s ya da %s)", codes.Ad(codes.A040), codes.Ad(codes.A041))
	}
	return &interp.StringVal{Val: s}, nil
}

// ornekDeger hata mesajlarında seçeneğe uygun örnek bir değerdir.
func ornekDeger(s *secenek) string {
	t := s.tur
	if t == turListe {
		return duzOrnek(s.ogeTuru) + "," + duzOrnek(s.ogeTuru)
	}
	return duzOrnek(t)
}

func duzOrnek(t tur) string {
	switch t {
	case turTam:
		return "5000"
	case turOndalik:
		return "0.001"
	case turMantik:
		return codes.Ad(codes.A040)
	}
	return "değer"
}

// kisalt hata mesajlarındaki argümanı (uzunsa) kısaltır.
func kisalt(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
