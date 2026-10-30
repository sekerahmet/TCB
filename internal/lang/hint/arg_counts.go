package hint

import (
	"strconv"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çekirdek metodun (Metin, Liste, Sözlük, Aralık) yanlış sayıda argümanla çağrılması çalışma anında
// ve denetimde aynı iletidir (H2011, 13.11): metodun adı ve kullanımı programdaki kimlikle yazılır
// (.TR_sort(), `.TR_join([ayırıcı])`), kullanımın parametreleri iletinin dilindedir.

// Counts are accepted argument counts, a message part written in the message's language: Counts(1,
// 2) → "1 ya da 2" (en "1 or 2"), Counts(0, 1, 2) → "0, 1 ya da 2".
func Counts(ns ...int) messages.Localized { return counts(ns) }

type counts []int

func (c counts) MessageText(lang string) string {
	parts := make([]string, len(c))
	for i, n := range c {
		parts[i] = strconv.Itoa(n)
	}
	return messages.OrListIn(lang, parts...)
}

// ExpectedCount is the accepted count of positional arguments as an argument-count message writes
// it: n when minN == maxN, "minN or more" when maxN < 0 (no upper bound), otherwise every count
// from minN to maxN.
func ExpectedCount(minN, maxN int) any {
	switch {
	case maxN < 0:
		return messages.New(messages.H2016, minN)
	case maxN == minN:
		return minN
	}
	var ns []int
	for n := minN; n <= maxN; n++ {
		ns = append(ns, n)
	}
	return counts(ns)
}

// MemberArgCount is the error of core member k on receiver type typ (codes.TypeList ...) called
// with given positional arguments where it takes minN to maxN (maxN < 0: no upper bound). An
// argument to .TR_sort() is a sort key, which is not there yet (13.3): the message shows today's
// way, sorting tuples whose first item is the key (H1116).
func MemberArgCount(typ, k codes.Code, minN, maxN, given int) error {
	var sortKey *messages.Error
	if k == codes.MemSort && given > 0 {
		sortKey = messages.New(messages.H1116)
	}
	return messages.New(messages.H2011, memberCode(k), ExpectedCount(minN, maxN), given,
		memberUsage{k, memberForms[memberKey{typ, k}]}, sortKey)
}

// memberCode is a member as the program writes it, without its receiver: .TR_sort.
type memberCode codes.Code

func (m memberCode) MessageText(string) string { return "." + codes.Spelling(codes.Code(m)) }

// memberUsage is a core member's call form in backticks: `.TR_join([ayırıcı])`; a member that takes
// no arguments has an empty form (`.TR_sort()`).
type memberUsage struct {
	k    codes.Code
	form any
}

func (u memberUsage) MessageText(lang string) string {
	form := ""
	switch f := u.form.(type) {
	case messages.Localized:
		form = f.MessageText(lang)
	case string:
		form = f
	}
	return "`" + memberCode(u.k).MessageText(lang) + "(" + form + ")`"
}

type memberKey struct{ typ, k codes.Code }

// textPart is a parameter of a usage written in the message's language (messages.Localized).
type textPart struct {
	c    messages.Code
	args []any
}

func (p textPart) MessageText(lang string) string { return messages.TextIn(lang, p.c, p.args...) }

// memberForms are the parameters of the core members' usage that take arguments; a parameter in
// square brackets is optional.
var memberForms = map[memberKey]any{
	{codes.TypeString, codes.MemSplit}:      textPart{messages.H2331, nil},
	{codes.TypeString, codes.MemReplace}:    "eski, yeni",
	{codes.TypeString, codes.MemContains}:   "alt",
	{codes.TypeString, codes.MemStartsWith}: textPart{messages.H2332, nil},
	{codes.TypeString, codes.MemEndsWith}:   "sonek",
	{codes.TypeString, codes.MemGet}:        "indeks",
	{codes.TypeList, codes.MemFilter}:       codes.ReceiverRef(codes.TypeFunction),
	{codes.TypeList, codes.MemMap}:          codes.ReceiverRef(codes.TypeFunction),
	{codes.TypeList, codes.MemGet}:          "indeks",
	{codes.TypeList, codes.MemJoin}:         textPart{messages.H2331, nil},
	{codes.TypeList, codes.MemContains}:     textPart{messages.H2301, nil},
	{codes.TypeList, codes.MemAppend}:       "eleman, ...",
	{codes.TypeList, codes.MemReduce}:       textPart{messages.H2333, []any{codes.ReceiverRef(codes.TypeFunction)}},
	{codes.TypeDict, codes.MemContains}:     "anahtar",
	{codes.TypeDict, codes.MemGet}:          "anahtar",
}
