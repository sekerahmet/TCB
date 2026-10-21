package process

import (
	"slices"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// komut_satırı(seçenekler, açıklama: "", açıklamalar: {}) betiğin seçeneklerini
// varsayılanlarıyla tanımlar ve komut satırı argümanlarıyla doldurulmuş YENİ bir sözlük
// döndürür (verilen sözlük değişmez). Seçenek adları sözlüğün anahtarlarıdır (veri; kodlanmaz),
// tipi varsayılandan çıkar. Kurulum hataları (desteklenmeyen varsayılan, ayrılmış ad, ikinci
// çağrı) sıradan çalışma zamanı hatalarıdır; kullanıcının yazdığı argümandaki hatalar ve
// --yardım ise programı bitirir (interp.ExitRequest: 2 ya da 0 koduyla).

// optKind bir seçeneğin değer tipidir.
type optKind int

const (
	kindInt optKind = iota
	kindFloat
	kindText
	kindBool
	kindList
	kindNone // varsayılan yok: verilirse Metin
)

// option komut satırının bir seçeneğidir.
type option struct {
	name        string
	optKind     optKind
	itemKind    optKind // kindList'de öğelerin tipi
	defaultVal  interp.Value
	description string
	secret      bool
}

// typeCode seçenek tipinin (yardımda ve hatalarda gösterilen) tip kodudur.
func (t optKind) typeCode() codes.Code {
	switch t {
	case kindInt:
		return codes.A200
	case kindFloat:
		return codes.A201
	case kindBool:
		return codes.A203
	case kindList:
		return codes.A205
	}
	return codes.A202
}

func (d *State) commandLine(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G801)
	if err := bind.CheckNamed(fn, named, nameDesc, nameDescs); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G801, "1", len(args), messages.Text(messages.H2329))
	}
	dict, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.G801, 1, args[0], codes.A206)
	}
	// Kurulum önce denetlenir: tanımdaki hata, ikinci çağrıda da adıyla bildirilir.
	options, err := buildOptions(m, fn, dict)
	if err != nil {
		return nil, err
	}
	description, err := readDescriptions(fn, named, options)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	called := d.called
	d.mu.Unlock()
	if called {
		return nil, messages.New(messages.H7218, fn)
	}
	result, err := applyArguments(m, options, description)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.called, d.settings = true, result
	d.mu.Unlock()
	return result, nil
}

// buildOptions sözlüğün anahtarlarını ve varsayılanlarını seçeneklere çevirir.
func buildOptions(m *interp.VM, fn string, dict *interp.DictVal) ([]*option, error) {
	var out []*option
	for _, k := range dict.Order {
		value, ok := dict.Pairs[k]
		if !ok {
			continue
		}
		if key := dict.KeyValue(k); key != nil {
			if _, text := key.(*interp.StringVal); !text {
				return nil, messages.New(messages.H7219, fn, codes.DisplayName(codes.A202), interp.TypeAndSummary(key))
			}
		}
		if err := checkOptionName(fn, k); err != nil {
			return nil, err
		}
		s := &option{name: k, defaultVal: value, secret: IsSecretName(k)}
		var err error
		if s.optKind, s.itemKind, err = defaultKind(fn, k, value); err != nil {
			return nil, err
		}
		if s.secret && m != nil {
			registerSecret(m, value)
		}
		out = append(out, s)
	}
	return out, nil
}

// checkOptionName seçenek adının komut satırında yazılabilir ve sonuç sözlüğünde
// okunabilir olduğunu denetler. Sözlüğün üye adları (uzunluk, anahtarlar ...) seçenek adı
// olamaz: ayar.uzunluk anahtarı değil eleman sayısını verirdi.
func checkOptionName(fn, name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsFunc(name, func(r rune) bool { return r == '=' || unicode.IsSpace(r) }) {
		return messages.New(messages.H7220, fn, name)
	}
	if name == "h" || isHelpName(name) { // --yardım'ın bir dildeki adı ya da -h
		return messages.New(messages.H7221, fn, name, helpOption())
	}
	if k, ok := codes.Find(codes.NamespaceMember, name); ok {
		if tn, ok := codes.DefinitionOf(k); ok && slices.Contains(tn.Types(), codes.A206) {
			return messages.New(messages.H7222, fn, name, codes.DisplayName(codes.A206))
		}
	}
	return nil
}

// defaultKind varsayılan değerden seçeneğin tipini çıkarır.
func defaultKind(fn, name string, v interp.Value) (optKind, optKind, error) {
	if t, ok := plainKind(v); ok {
		return t, kindText, nil
	}
	switch x := v.(type) {
	case *interp.NullVal:
		return kindNone, kindText, nil
	case *interp.ListVal:
		item := kindText
		for i, e := range x.Elements {
			t, ok := plainKind(e)
			if !ok {
				return 0, 0, messages.New(messages.H7223,
					fn, name, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), i+1, interp.TypeAndSummary(e))
			}
			switch {
			case i == 0:
				item = t
			case item == t:
			case (item == kindInt && t == kindFloat) || (item == kindFloat && t == kindInt):
				item = kindFloat
			default:
				return 0, 0, messages.New(messages.H7224, fn, name, i+1, interp.TypeAndSummary(e))
			}
		}
		return kindList, item, nil
	}
	return 0, 0, messages.New(messages.H7225,
		fn, name, interp.TypeNames(codes.A200, codes.A201, codes.A202, codes.A203), codes.Name(codes.A205), codes.Name(codes.A042), interp.TypeAndSummary(v))
}

// plainKind Tam, Ondalık, Metin ya da Mantık değerin tipidir.
func plainKind(v interp.Value) (optKind, bool) {
	switch v.(type) {
	case *interp.IntVal:
		return kindInt, true
	case *interp.FloatVal:
		return kindFloat, true
	case *interp.StringVal:
		return kindText, true
	case *interp.BoolVal:
		return kindBool, true
	}
	return 0, false
}

// readDescriptions açıklama: ve açıklamalar: adlı argümanlarını okur; açıklamalar
// seçeneklere yazılır, programın açıklaması döner.
func readDescriptions(fn string, named map[string]interp.Value, options []*option) (string, error) {
	description := ""
	if v, ok := bind.GetNamed(named, nameDesc); ok {
		s, ok := v.(*interp.StringVal)
		if !ok {
			return "", messages.New(messages.H7226, fn, nameDesc.TR, codes.DisplayName(codes.A202), interp.TypeAndSummary(v))
		}
		description = s.Val
	}
	v, ok := bind.GetNamed(named, nameDescs)
	if !ok {
		return description, nil
	}
	dict, ok := v.(*interp.DictVal)
	if !ok {
		return "", messages.New(messages.H7227, fn, nameDescs.TR, codes.DisplayName(codes.A206), interp.TypeAndSummary(v))
	}
	names := make([]string, len(options))
	for i, s := range options {
		names[i] = s.name
	}
	for _, k := range dict.Order {
		value, ok := dict.Pairs[k]
		if !ok {
			continue
		}
		i := slices.Index(names, k)
		if i < 0 {
			if o := turkish.Closest(k, names); o != "" {
				return "", messages.New(messages.H7229, fn, nameDescs.TR, dict.DisplayKey(k), o)
			}
			return "", messages.New(messages.H7228, fn, nameDescs.TR, dict.DisplayKey(k))
		}
		s, ok := value.(*interp.StringVal)
		if !ok {
			return "", messages.New(messages.H7230, fn, nameDescs.TR, k, codes.DisplayName(codes.A202), interp.TypeAndSummary(value))
		}
		options[i].description = s.Val
	}
	return description, nil
}

// registerSecret gizli bir seçeneğin metin değerini (liste seçeneğinde her metin öğesini)
// gizli değer süzgecine verir.
func registerSecret(m *interp.VM, v interp.Value) {
	switch x := v.(type) {
	case *interp.StringVal:
		addSecret(m, x.Val)
	case *interp.ListVal:
		for _, el := range x.Elements {
			registerSecret(m, el)
		}
	}
}
