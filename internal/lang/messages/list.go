package messages

// OrList writes items as a list of alternatives in the active language, with the language's
// list pattern (CLDR "or" list): two items with H2010 ("a ya da b"), more with the separator
// H2005 ("a, b") and the final pattern H2006 ("a, b ya da c"). Languages differ in these
// patterns (English: "a or b", but "a, b, or c"), so Go code never joins alternatives itself.
func OrList(items ...string) string { return orList(Text, items) }

// OrListIn is OrList in the given language (for a parameter value written in the language of
// its message, Localized).
func OrListIn(lang string, items ...string) string {
	return orList(func(c Code, args ...any) string { return TextIn(lang, c, args...) }, items)
}

func orList(text func(Code, ...any) string, items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return text(H2010, items[0], items[1])
	}
	s := items[0]
	for _, it := range items[1 : len(items)-1] {
		s = text(H2005, s, it)
	}
	return text(H2006, s, items[len(items)-1])
}
