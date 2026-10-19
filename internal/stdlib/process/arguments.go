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

// argLimit bir argümanın en uzun boyudur (bayt).
const argLimit = 1 << 20

// Kullanıcının yazdığı argümanlardaki hatanın çıkış kodu ("yanlış kullanım").
const usageErrorCode = 2

// helpOptions seçenek listesini isteyen argümanlardır.
var helpOptions = []string{"--yardım", "-h"}

// applyArguments programın argümanlarını okur (okunmuş sayılırlar) ve seçeneklerin
// değerleriyle yeni bir sözlük kurar. --yardım ya da hatalı argüman bir program çıkışı
// isteği (interp.ExitRequest) döndürür.
func applyArguments(m *interp.VM, options []*option, description string) (*interp.DictVal, error) {
	var argv []string
	command := "tr"
	if m != nil {
		argv = m.Args()
		command = m.ProgramCommand()
	}
	for _, a := range argv {
		if a == helpOptions[0] || a == helpOptions[1] {
			return nil, &interp.ExitRequest{Code: 0, Message: helpText(command, description, options)}
		}
	}
	given, err := parse(m, argv, options)
	if err != nil {
		msg := err.Error()
		if !strings.HasSuffix(msg, "?") {
			msg += "."
		}
		return nil, &interp.ExitRequest{Code: usageErrorCode,
			Message: fmt.Sprintf("Hata: %s Seçenekler için: %s %s", msg, command, helpOptions[0])}
	}
	result := interp.NewDictVal()
	for _, s := range options {
		v, ok := given[s.name]
		if !ok {
			v = s.defaultVal
		}
		result.Set(s.name, v)
	}
	return result, nil
}

// parse argümanları seçeneklere göre çözer: seçenek adı → değer.
func parse(m *interp.VM, argv []string, options []*option) (map[string]interp.Value, error) {
	given := map[string]interp.Value{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if len(a) > argLimit {
			return nil, fmt.Errorf("%d. argüman çok uzun (%d bayt; en çok 1 MB)", i+1, len(a))
		}
		name, value, valued, err := optionName(a, options)
		if err != nil {
			return nil, err
		}
		if valued && IsSecretName(name) && m != nil {
			addSecret(m, value) // hata mesajında da görünmesin
		}
		s := find(options, name)
		if s == nil {
			return nil, unknownOption("--"+name, options)
		}
		if _, before := given[s.name]; before {
			return nil, fmt.Errorf("'--%s' iki kez verildi", s.name)
		}
		if !valued && s.optKind != kindBool {
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "--") {
				return nil, fmt.Errorf("'--%s' bir değer bekler (ör. --%s %s ya da --%s=%s)", s.name, s.name, exampleValue(s), s.name, exampleValue(s))
			}
			i++
			value, valued = argv[i], true
			if len(value) > argLimit {
				return nil, fmt.Errorf("'--%s' değeri çok uzun (%d bayt; en çok 1 MB)", s.name, len(value))
			}
			if s.secret && m != nil {
				addSecret(m, value)
			}
		}
		v, err := parseValue(s, value, valued)
		if err != nil {
			return nil, err
		}
		given[s.name] = v
	}
	return given, nil
}

// optionName bir argümanı seçenek adına ve (--ad=değer biçiminde) değerine ayırır;
// argüman bir seçenek değilse kullanıcıya yol gösteren hatayı verir.
func optionName(a string, options []*option) (name, value string, valued bool, err error) {
	switch {
	case a == "--":
		return "", "", false, fmt.Errorf("'--' tek başına kullanılmaz; seçenekler --ad değer biçiminde yazılır")
	case strings.HasPrefix(a, "--"):
		name = a[2:]
		if i := strings.IndexByte(name, '='); i >= 0 {
			name, value, valued = name[:i], name[i+1:], true
		}
		return name, value, valued, nil
	case strings.HasPrefix(a, "-") && len(a) > 1:
		e := unknownOption(a, options)
		return "", "", false, fmt.Errorf("%v; seçenekler iki tireyle yazılır (--ad değer)", e)
	}
	return "", "", false, fmt.Errorf("'%s' bir seçenek değil: argümanlar --ad değer biçiminde verilir (konumlu argüman yoktur)", shorten(a))
}

func find(options []*option, name string) *option {
	for _, s := range options {
		if s.name == name {
			return s
		}
	}
	return nil
}

// unknownOption en yakın seçeneği öneren hatayı verir; verilen, argümanın yazıldığı
// biçimdir (--ogrenme_orani, -adım).
func unknownOption(given string, options []*option) error {
	name := strings.TrimLeft(given, "-")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	candidates := []string{"yardım"}
	for _, s := range options {
		candidates = append(candidates, s.name)
	}
	o := turkish.Closest(name, candidates)
	if o == "" && find(options, name) != nil {
		o = name // -adım → --adım
	}
	if o == "" && (name == "help" || name == "h") {
		o = "yardım"
	}
	if o == "" {
		return fmt.Errorf("bilinmeyen seçenek '%s'", shorten(given))
	}
	return fmt.Errorf("bilinmeyen seçenek '%s'. Bunu mu demek istediniz: '--%s'?", shorten(given), o)
}

// parseValue bir argüman değerini seçeneğin tipine çevirir.
func parseValue(s *option, value string, valued bool) (interp.Value, error) {
	if s.optKind == kindBool {
		if !valued {
			return interp.TRUE, nil
		}
		switch value {
		case codes.Name(codes.A040):
			return interp.TRUE, nil
		case codes.Name(codes.A041):
			return interp.FALSE, nil
		}
		return nil, fmt.Errorf("'--%s' %s bir seçenek; %s ya da %s verin (yalnız --%s yazmak %s demektir), %q verildi",
			s.name, codes.Name(codes.A203), codes.Name(codes.A040), codes.Name(codes.A041), s.name, codes.Name(codes.A040), shorten(value))
	}
	if s.optKind != kindList {
		v, err := parsePlainValue(s.optKind, value)
		if err != nil {
			return nil, fmt.Errorf("'--%s' %s bekler, %q verildi%s", s.name, codes.Name(s.optKind.typeCode()), shorten(value), err.Error())
		}
		return v, nil
	}
	list := &interp.ListVal{}
	if value == "" {
		return list, nil
	}
	for i, p := range strings.Split(value, ",") {
		v, err := parsePlainValue(s.itemKind, strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("'--%s' virgülle ayrılmış %s listesi bekler; %d. öğe %q%s", s.name, codes.Name(s.itemKind.typeCode()), i+1, shorten(p), err.Error())
		}
		list.Elements = append(list.Elements, v)
	}
	return list, nil
}

// parsePlainValue Tam, Ondalık, Mantık ya da Metin bir değeri çözer. Hata metni, mesaja eklenecek
// açıklamadır ("; ondalık ayırıcı nokta olmalı: 0.001" gibi; boş olabilir).
func parsePlainValue(t optKind, s string) (interp.Value, error) {
	switch t {
	case kindInt:
		n, err := parseInt(s)
		if err != nil {
			return nil, err
		}
		return interp.NewInt(n), nil
	case kindFloat:
		f, err := parseFloat(s)
		if err != nil {
			return nil, err
		}
		return &interp.FloatVal{Val: f}, nil
	case kindBool:
		switch s {
		case codes.Name(codes.A040):
			return interp.TRUE, nil
		case codes.Name(codes.A041):
			return interp.FALSE, nil
		}
		return nil, fmt.Errorf(" (%s ya da %s)", codes.Name(codes.A040), codes.Name(codes.A041))
	}
	return &interp.StringVal{Val: s}, nil
}

// exampleValue hata mesajlarında seçeneğe uygun örnek bir değerdir.
func exampleValue(s *option) string {
	t := s.optKind
	if t == kindList {
		return plainExample(s.itemKind) + "," + plainExample(s.itemKind)
	}
	return plainExample(t)
}

func plainExample(t optKind) string {
	switch t {
	case kindInt:
		return "5000"
	case kindFloat:
		return "0.001"
	case kindBool:
		return codes.Name(codes.A040)
	}
	return "değer"
}

// shorten hata mesajlarındaki argümanı (uzunsa) kısaltır.
func shorten(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
