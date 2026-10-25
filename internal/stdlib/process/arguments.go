package process

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// Yardım seçeneği bir ad kodudur (K106): her yazım dilinin adı kabul edilir (--yardım de
// --help de), yardımda ve iletilerde etkin dilin adı yazılır; -h simgedir.

// helpOption yardım seçeneğinin etkin dildeki yazımıdır (--yardım).
func helpOption() string { return codes.OptionFlag(codes.K106) }

// isHelpArgument argümanın yardımı istediğidir: -h ya da herhangi bir dilde --yardım.
func isHelpArgument(a string) bool {
	if a == "-h" {
		return true
	}
	name, ok := strings.CutPrefix(a, "--")
	return ok && isHelpName(name)
}

// isHelpName adın (tiresiz) yardım seçeneğinin bir dildeki adı olduğudur.
func isHelpName(name string) bool {
	k, ok := codes.FindOption(name)
	return ok && k == codes.K106
}

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
		if isHelpArgument(a) {
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
			Message: messages.Text(messages.H7200, msg, command, helpOption())}
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
			return nil, messages.New(messages.H7201, i+1, len(a))
		}
		name, value, valued, err := optionName(a, options)
		if err != nil {
			return nil, err
		}
		if valued && IsSecretName(name) && m != nil {
			addSecretItems(m, value) // hata mesajında da görünmesin
		}
		s := find(options, name)
		if s == nil {
			return nil, unknownOption("--"+name, options)
		}
		if _, before := given[s.name]; before {
			return nil, messages.New(messages.H7202, s.name)
		}
		if !valued && s.optKind != kindBool {
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "--") {
				return nil, messages.New(messages.H7203, s.name, exampleValue(s))
			}
			i++
			value, valued = argv[i], true
			if len(value) > argLimit {
				return nil, messages.New(messages.H7204, s.name, len(value))
			}
			if s.secret && m != nil {
				addSecretItems(m, value)
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
		return "", "", false, messages.New(messages.H7205)
	case strings.HasPrefix(a, "--"):
		name = a[2:]
		if i := strings.IndexByte(name, '='); i >= 0 {
			name, value, valued = name[:i], name[i+1:], true
		}
		return name, value, valued, nil
	case strings.HasPrefix(a, "-") && len(a) > 1:
		e := unknownOption(a, options)
		return "", "", false, messages.New(messages.H7206, e)
	}
	return "", "", false, messages.New(messages.H7207, shorten(a))
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
	// Değer iletide gösterilmez: yanlış yazılmış bir gizli seçeneğin (-api_key=...) değeri
	// gizli değer süzgecine henüz verilmemiştir.
	if i := strings.IndexByte(given, '='); i >= 0 {
		given = given[:i] + "=…"
	}
	candidates := []string{codes.DisplayName(codes.K106)}
	for _, s := range options {
		candidates = append(candidates, s.name)
	}
	o := turkish.Closest(name, candidates)
	if o == "" && find(options, name) != nil {
		o = name // -adım → --adım
	}
	if o == "" && name == "h" {
		o = codes.DisplayName(codes.K106)
	}
	if o == "" {
		return messages.New(messages.H7208, shorten(given))
	}
	return messages.New(messages.H7209, shorten(given), o)
}

// parseValue bir argüman değerini seçeneğin tipine çevirir.
func parseValue(s *option, value string, valued bool) (interp.Value, error) {
	if s.optKind == kindBool {
		if !valued {
			return interp.True, nil
		}
		if b, ok := boolName(value); ok {
			return interp.BoolToVal(b), nil
		}
		return nil, messages.New(messages.H7210, s.name, codes.NameRef(codes.A203), codes.NameRef(codes.A040), codes.NameRef(codes.A041), shorten(value))
	}
	if s.optKind != kindList {
		v, err := parsePlainValue(s.optKind, value)
		if err != nil {
			return nil, messages.New(messages.H7211, s.name, codes.NameRef(s.optKind.typeCode()), shorten(value), err)
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
			return nil, messages.New(messages.H7212, s.name, codes.NameRef(s.itemKind.typeCode()), i+1, shorten(p), err)
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
		if b, ok := boolName(s); ok {
			return interp.BoolToVal(b), nil
		}
		return nil, messages.New(messages.H7213, codes.NameRef(codes.A040), codes.NameRef(codes.A041))
	}
	return &interp.StringVal{Val: s}, nil
}

// Komut satırı kişinin girdisidir: değer adları her gömülü yazım dilinde kabul edilir (17.8;
// komutlar ve seçenek adları gibi). Yardım varsayılanı kişinin dilinde gösterir (TR_DIL), betik
// kendi dilinde yazılmıştır; ikisinin yazımı da okunur.

// boolName s bir gömülü yazım dilinde doğru ya da yanlışın adı mı (A040, A041)?
func boolName(s string) (value, ok bool) {
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			switch s {
			case l.Name(codes.A040):
				return true, true
			case l.Name(codes.A041):
				return false, true
			}
		}
	}
	return false, false
}

// nonFiniteName s bir gömülü yazım dilinde sonsuz ya da tanımsız sayının adı mı (A110, A111)?
func nonFiniteName(s string) (float64, bool) {
	for _, code := range codes.Languages() {
		if l, err := codes.LoadLanguage(code); err == nil {
			if f, ok := interp.ParseNonFinite(s, l); ok {
				return f, true
			}
		}
	}
	return 0, false
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
		return codes.DisplayName(codes.A040) // iletide örnek: kişinin dilinde
	}
	return messages.Text(messages.H7214) // değer
}

// shorten hata mesajlarındaki argümanı (uzunsa) kısaltır.
func shorten(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
