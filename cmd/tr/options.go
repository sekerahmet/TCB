package main

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/hint"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Komut satırı seçenekleri ad kodlarıdır (opt): komutlar gibi her yazım dilinin adı kabul
// edilir (--dil de --lang da; betikler TR_DIL'den bağımsız çalışır), çıktı etkin dildedir.
// Seçenek iki tireyle, sözcükleri tireyle ayrılarak yazılır: kodun adındaki alt çizgi komut
// satırında tiredir (go_profili → --go-profili; GNU ve Go araçlarının yazımı). Kısa bayraklar
// (-k, -h, -v, -w) simgedir, kod almaz.
//
// '-' ile başlayan argüman (tek başına '-' standart giriştir) dosya adı sanılmaz: komutun
// seçeneği değilse bilinmeyen seçenektir (Python argparse 'unrecognized arguments', Go flag
// 'flag provided but not defined'). Adı '-' ile başlayan dosya '--'dan sonra verilir.

// optionCode bir argümanın seçenek kodudur; uzun seçenek değilse ok false'tur.
func optionCode(arg string) (codes.Code, bool) {
	name, ok := strings.CutPrefix(arg, "--")
	if !ok || name == "" || strings.Contains(name, "_") {
		return codes.None, false
	}
	return codes.FindOption(strings.ReplaceAll(name, "-", "_"))
}

// isOption argümanın k seçeneği olduğudur.
func isOption(arg string, k codes.Code) bool {
	c, ok := optionCode(arg)
	return ok && c == k
}

// looksLikeOption argümanın seçenek biçiminde ('-' ile başlayan) olduğudur; tek başına '-'
// standart giriştir, '--' seçeneklerin sonudur.
func looksLikeOption(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg != "--"
}

// unknownOption komutun almadığı bir seçenekle (H8674) çıkışa götürür: ileti komutun seçeneklerini
// (allowed) sayar ve yazımı en yakın olanı önerir.
func unknownOption(command any, arg string, allowed []codes.Code) {
	name, _, _ := strings.Cut(arg, "=")
	list := make([]string, len(allowed))
	for i, a := range allowed {
		list[i] = codes.OptionFlag(a)
	}
	exitUsage(messages.Text(messages.H8674, command, name, strings.Join(list, ", "), hint.ClosestSuggestion(name, list, nil)))
}

// unknownTopLevelOption tr'nin kendisine verilen seçenek biçimindeki ilk argümanla çıkışa götürür:
// '--yardım', '--help' gibi bir komutun adıysa (ya da yazımı ona en yakınsa) asıl komut (H8602),
// değilse bilinmeyen seçenek (H8604) iletisiyle.
func unknownTopLevelOption(arg string) {
	name := strings.ReplaceAll(strings.TrimLeft(arg, "-"), "-", "_")
	k, ok := codes.FindCommand(name)
	if !ok {
		var names []string
		for _, c := range codes.Codes() {
			if c.Class() == codes.ClassCommand {
				names = append(names, codes.DisplayName(c))
			}
		}
		if closest := turkish.Closest(name, names); closest != "" {
			k, ok = codes.FindCommand(closest)
		}
	}
	if ok {
		exitUsage(messages.Text(messages.H8602, arg, codes.NameRef(k)))
	}
	exitUsage(messages.Text(messages.H8604, arg, codes.NameRef(codes.CmdHelp)))
}

// fileArguments komutun dosya argümanlarını verir: '--'dan önce seçenek biçimindeki her argüman
// accept'in tanıdığı bir seçenek olmalıdır (değilse unknownOption; accept nil ise komutun seçeneği
// yoktur) ve sonuca girmez. İlk '--' atlanır, ondan sonraki her argüman dosyadır.
func fileArguments(command any, args []string, allowed []codes.Code, accept func(string) bool) []string {
	var files []string
	for i, a := range args {
		switch {
		case a == "--":
			return append(files, args[i+1:]...)
		case looksLikeOption(a):
			if accept == nil || !accept(a) {
				unknownOption(command, a, allowed)
			}
		default:
			files = append(files, a)
		}
	}
	return files
}

// programFile ilk argümanı (program dosyası) ve ondan sonrakileri (programın argümanları) ayırır:
// '--' ise atlanır ve sonraki argüman dosyadır; seçenek biçimindeyse komutun seçeneği olmadığından
// unknownOption. Dosya yoksa ok false'tur.
func programFile(command any, args []string) (file string, rest []string, ok bool) {
	switch {
	case len(args) > 0 && args[0] == "--":
		args = args[1:]
	case len(args) > 0 && looksLikeOption(args[0]):
		unknownOption(command, args[0], nil)
	}
	if len(args) == 0 {
		return "", nil, false
	}
	return args[0], args[1:], true
}

// noOptions argüman almayan bir komutun seçenek biçimindeki argümanını reddeder.
func noOptions(command codes.Code, args []string) {
	fileArguments(codes.NameRef(command), args, nil, nil)
}

// parseLongOption '--ad' ya da '--ad=değer' biçimindeki bir seçeneği çözer: izin verilmeyen ya da
// bilinmeyen seçenek H8674 ile, değer gereken seçenekte eksik değer H8675 ile çıkışa götürür.
// rest, seçenekten sonraki argümanlardır; değer ayrı bir argümansa ondan alınır. valued, seçeneğin
// değer alıp almadığını söyler (almayan seçenekte değer boş döner). Kalan argümanları döndürür.
func parseLongOption(command any, arg string, rest []string, allowed []codes.Code, valued func(codes.Code) bool) (k codes.Code, value string, remaining []string) {
	name, value, hasValue := strings.Cut(arg, "=")
	k, ok := optionCode(name)
	if !ok || !slices.Contains(allowed, k) {
		unknownOption(command, name, allowed)
	}
	if valued(k) && !hasValue {
		if len(rest) == 0 {
			exitUsage(messages.Text(messages.H8675, codes.OptionFlag(k), command))
		}
		value, rest = rest[0], rest[1:]
	}
	return k, value, rest
}
