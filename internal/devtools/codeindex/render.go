package codeindex

import (
	"fmt"
	"sort"
	"strings"
)

// Text sonucu insan için kısa düz metne çevirir: hedef, sonra komuta göre başvurular, testler ya da
// etkilenenler; belirsiz adda adaylar, bulunamayan adda yakın adlar. `[INFERRED]` etiketi
// yalnızca tahminlerde görünür (etiketsiz olanlar kesindir).
func Text(r *Result) string {
	var b strings.Builder
	switch r.Status {
	case StatusAmbiguous:
		fmt.Fprintf(&b, "%q is ambiguous: %d candidates (repeat with an id, a file:line:col or a longer name)\n", r.Query.Text, len(r.Candidates))
		for _, n := range r.Candidates {
			fmt.Fprintf(&b, "  %s  (id %s)\n", Short(n), n.ID)
		}
		return b.String()
	case StatusNotFound:
		fmt.Fprintf(&b, "%q not found\n", r.Query.Text)
		if len(r.DidYouMean) > 0 {
			b.WriteString("did you mean:\n")
			for _, n := range r.DidYouMean {
				fmt.Fprintf(&b, "  %s\n", Short(n))
			}
		}
		return b.String()
	}
	if r.Target != nil {
		fmt.Fprintf(&b, "%s\n", Short(*r.Target))
		if r.Target.Signature != "" {
			fmt.Fprintf(&b, "  %s\n", r.Target.Signature)
		}
	}
	if r.Docs != "" {
		for _, line := range strings.Split(r.Docs, "\n") {
			fmt.Fprintf(&b, "  | %s\n", line)
		}
	}
	writeNodes(&b, "members", r.Members)
	writeUses(&b, "users", r.Users, r.Counts, r.Truncated)
	writeUses(&b, "tests", r.Tests, nil, r.Truncated)
	if a := r.Affected; a != nil {
		writeNodes(&b, "importing modules", a.Modules)
		writeUses(&b, "users outside the module", a.Users, r.Counts, r.Truncated)
		writeNodes(&b, "tests", a.Tests)
	}
	return b.String()
}

func writeNodes(b *strings.Builder, title string, nodes []Node) {
	if len(nodes) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s (%d):\n", title, len(nodes))
	for _, n := range nodes {
		fmt.Fprintf(b, "    %s\n", Short(n))
	}
}

func writeUses(b *strings.Builder, title string, uses []Use, counts *Counts, truncated bool) {
	if len(uses) == 0 {
		return
	}
	total := len(uses)
	if counts != nil {
		total = counts.Total
	}
	fmt.Fprintf(b, "  %s (%d", title, total)
	if counts != nil && len(counts.ByRole) > 0 {
		var parts []string
		for role, n := range counts.ByRole {
			parts = append(parts, fmt.Sprintf("%d %s", n, role))
		}
		sort.Strings(parts)
		fmt.Fprintf(b, ": %s", strings.Join(parts, ", "))
	}
	b.WriteString("):\n")
	for _, u := range uses {
		who := ""
		if u.In != nil {
			who = u.In.Qualified
			if who == "" {
				who = u.In.Name
			}
		}
		line := fmt.Sprintf("    %-10s %s  %s", u.Role, u.Pos, who)
		if u.Steps > 0 {
			line += fmt.Sprintf("  (%d steps)", u.Steps)
		}
		if u.Confidence == Inferred {
			line += "  [INFERRED]"
		}
		b.WriteString(line + "\n")
	}
	if truncated {
		b.WriteString("    ... more (raise --limit or use --all)\n")
	}
}
