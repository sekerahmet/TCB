package main

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Komut satırı seçenekleri ad kodlarıdır (opt): komutlar gibi her yazım dilinin adı kabul
// edilir (--dil de --lang da; betikler TR_DIL'den bağımsız çalışır), çıktı etkin dildedir.
// Seçenek iki tireyle, sözcükleri tireyle ayrılarak yazılır: kodun adındaki alt çizgi komut
// satırında tiredir (go_profili → --go-profili; GNU ve Go araçlarının yazımı). Kısa bayraklar
// (-k, -h, -v, -w) simgedir, kod almaz.

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

// parseLongOption '--ad' ya da '--ad=değer' biçimindeki bir seçeneği çözer: izin verilmeyen ya da
// bilinmeyen seçenek H8674 ile, değer gereken seçenekte eksik değer H8675 ile çıkışa götürür.
// rest, seçenekten sonraki argümanlardır; değer ayrı bir argümansa ondan alınır. valued, seçeneğin
// değer alıp almadığını söyler (almayan seçenekte değer boş döner). Kalan argümanları döndürür.
func parseLongOption(command any, arg string, rest []string, allowed []codes.Code, valued func(codes.Code) bool) (k codes.Code, value string, remaining []string) {
	name, value, hasValue := strings.Cut(arg, "=")
	k, ok := optionCode(name)
	if !ok || !slices.Contains(allowed, k) {
		list := make([]string, len(allowed))
		for i, a := range allowed {
			list[i] = codes.OptionFlag(a)
		}
		exitWithError(messages.Text(messages.H8674, command, name, strings.Join(list, ", ")))
	}
	if valued(k) && !hasValue {
		if len(rest) == 0 {
			exitWithError(messages.Text(messages.H8675, codes.OptionFlag(k), command))
		}
		value, rest = rest[0], rest[1:]
	}
	return k, value, rest
}
