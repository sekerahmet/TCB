package main

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Komut satırı seçenekleri ad kodlarıdır (K1xx): komutlar gibi her yazım dilinin adı kabul
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

// optionName seçeneğin etkin dildeki yazımıdır (--dil, --go-profili).
func optionName(k codes.Code) string {
	return "--" + strings.ReplaceAll(codes.DisplayName(k), "_", "-")
}

// removedOption argümanın kaldırılmış bir seçenek olduğudur (ör. --web); çağıran yerine
// geçeni söyler.
func removedOption(arg string) bool {
	name, ok := strings.CutPrefix(arg, "--")
	if !ok || name == "" || strings.Contains(name, "_") {
		return false
	}
	_, dead := codes.FindRemovedCommandLine(codes.NamespaceOption, strings.ReplaceAll(name, "-", "_"))
	return dead
}
