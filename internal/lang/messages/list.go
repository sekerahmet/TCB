package messages

// OrList writes items as a list of alternatives in the active language, with the language's
// list pattern (CLDR "or" list): two items with H2010 ("a ya da b"), more with the separator
// H2005 ("a, b") and the final pattern H2006 ("a, b ya da c"). Languages differ in these
// patterns (English: "a or b", but "a, b, or c"), so Go code never joins alternatives itself.
func OrList(items ...string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return Text(H2010, items[0], items[1])
	}
	s := items[0]
	for _, it := range items[1 : len(items)-1] {
		s = Text(H2005, s, it)
	}
	return Text(H2006, s, items[len(items)-1])
}
