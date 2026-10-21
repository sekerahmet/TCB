package codeindex

import (
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// kindLabels ve roleLabels düğüm türlerinin ve başvuru rollerinin okuyanın dilindeki adlarının
// ileti kodlarıdır (JSON'da değişmez tanımlayıcılar yazılır, düz metinde okuyanın dili).
var kindLabels = map[Kind]messages.Code{
	KindFunction: messages.H8800, KindStruct: messages.H8801, KindMethod: messages.H8802, KindField: messages.H8803,
	KindConstant: messages.H8804, KindVariable: messages.H8805, KindModule: messages.H8806, KindTest: messages.H8807,
	KindBuiltin: messages.H8808, KindParameter: messages.H8860, KindOption: messages.H8861,
}

var roleLabels = map[Role]messages.Code{
	RoleDefinition: messages.H8810, RoleCall: messages.H8241, RoleRead: messages.H8812,
	RoleWrite: messages.H8813, RoleImport: messages.H8814, RoleTest: messages.H8807,
	RoleArgument: messages.H8858, RoleOptionKey: messages.H8859,
}

// roleOrder rolleler listelenirken kullanılan sıradır.
var roleOrder = []Role{RoleDefinition, RoleCall, RoleRead, RoleWrite, RoleImport, RoleTest, RoleArgument, RoleOptionKey}

// KindLabel bir düğüm türünün okuyanın dilindeki adıdır.
func KindLabel(k Kind) string {
	if c, ok := kindLabels[k]; ok {
		return messages.Text(c)
	}
	return string(k)
}

// RoleLabel bir rolün okuyanın dilindeki adıdır.
func RoleLabel(r Role) string {
	if c, ok := roleLabels[r]; ok {
		return messages.Text(c)
	}
	return string(r)
}

// Short bir düğümü tek satırda yazar: "tür ad dosya:satır:sütun" (tür okuyanın dilinde).
func Short(n Node) string {
	name := n.Qualified
	if name == "" {
		name = n.Name
	}
	if n.Display != "" {
		name = n.Display + " (" + n.Name + ")"
	}
	s := KindLabel(n.Kind) + " " + name
	if n.Pos != nil {
		s += "  " + n.Pos.String()
	}
	return strings.TrimSpace(s)
}

// optionName bir seçenek kodunun komut satırı yazımıdır (--ad, sözcükler tireyle).
func optionName(k codes.Code) string {
	return "--" + strings.ReplaceAll(codes.DisplayName(k), "_", "-")
}

// Text sonucu insan için kısa düz metne çevirir: hedef, sonra komuta göre başvurular, testler ya da
// etkilenenler; belirsiz adda adaylar, bulunamayan adda yakın adlar. Metinler ileti kodlarındandır
// (okuyanın dilinde); `[INFERRED]` etiketi yalnızca tahminlerde görünür, etiketsizler kesindir.
func Text(r *Result) string {
	var b strings.Builder
	switch r.Status {
	case StatusAmbiguous:
		fmt.Fprintln(&b, messages.Text(messages.H8780, r.Query.Text, len(r.Candidates)))
		for _, n := range r.Candidates {
			fmt.Fprintf(&b, "  %s\n", messages.Text(messages.H8781, Short(n), n.ID))
		}
		return b.String()
	case StatusNotFound:
		fmt.Fprintln(&b, messages.Text(messages.H8782, r.Query.Text))
		if len(r.DidYouMean) > 0 {
			fmt.Fprintln(&b, messages.Text(messages.H8783))
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
	for _, line := range strings.Split(r.Docs, "\n") {
		if line != "" {
			fmt.Fprintf(&b, "  | %s\n", line)
		}
	}
	if len(r.Members) > 0 {
		fmt.Fprintf(&b, "  %s\n", messages.Text(messages.H8784, len(r.Members)))
		for _, n := range r.Members {
			fmt.Fprintf(&b, "    %s\n", Short(n))
		}
	}
	switch r.Command {
	case CommandReferences:
		writeUses(&b, messages.H8785, r.Users, r.Counts, r.Truncated, messages.H8792)
	case CommandTestedBy:
		writeUses(&b, messages.H8786, r.Tests, nil, r.Truncated, messages.H8793)
	case CommandImpact:
		writeImpact(&b, r)
	}
	return b.String()
}

func writeImpact(b *strings.Builder, r *Result) {
	a := r.Affected
	if a == nil {
		return
	}
	if len(a.Modules) > 0 {
		fmt.Fprintf(b, "  %s\n", messages.Text(messages.H8787, len(a.Modules)))
		for _, n := range a.Modules {
			fmt.Fprintf(b, "    %s\n", Short(n))
		}
	}
	writeUses(b, messages.H8788, a.Users, r.Counts, r.Truncated, messages.H8792)
	if len(a.Tests) > 0 {
		fmt.Fprintf(b, "  %s\n", messages.Text(messages.H8789, len(a.Tests)))
		for _, n := range a.Tests {
			fmt.Fprintf(b, "    %s\n", Short(n))
		}
	}
}

// writeUses başvuru satırlarını yazar; başlıkta toplam ve rollere göre dağılım bulunur, liste
// boşsa none iletisi yazılır.
func writeUses(b *strings.Builder, heading messages.Code, uses []Use, counts *Counts, truncated bool, none messages.Code) {
	if len(uses) == 0 {
		fmt.Fprintf(b, "  %s\n", messages.Text(none))
		return
	}
	total, roles := len(uses), ""
	if counts != nil {
		total = counts.Total
		var parts []string
		for _, role := range roleOrder {
			if n := counts.ByRole[role]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, RoleLabel(role)))
			}
		}
		if len(parts) > 0 {
			roles = ": " + strings.Join(parts, ", ")
		}
	}
	fmt.Fprintf(b, "  %s\n", messages.Text(heading, total, roles))
	for _, u := range uses {
		who := ""
		if u.In != nil {
			who = u.In.Qualified
			if who == "" {
				who = u.In.Name
			}
		}
		line := fmt.Sprintf("    %-10s %s  %s", RoleLabel(u.Role), u.Pos, who)
		if u.Steps > 0 {
			line += "  " + messages.Text(messages.H8790, u.Steps)
		}
		if u.Confidence == Inferred {
			line += "  [INFERRED]"
		}
		b.WriteString(line + "\n")
	}
	if truncated {
		fmt.Fprintf(b, "    ... %s\n", messages.Text(messages.H8791, optionName(codes.K107), optionName(codes.K108)))
	}
}

// reasonMessages reddetme nedenlerinin iletileridir.
var reasonMessages = map[string]messages.Code{
	ReasonBuiltin: messages.H8841, ReasonModule: messages.H8842, ReasonNoSymbol: messages.H8843,
}

// matchLabels olası yerlerin eşleşme biçimlerinin iletileridir; whereLabels bulundukları yerin.
var matchLabels = map[string]messages.Code{
	MatchMemberGuess: messages.H8847, MatchShadowed: messages.H8848, MatchExact: messages.H8849,
	MatchSuffixed: messages.H8850, MatchCompound: messages.H8851, MatchDerived: messages.H8865,
}

var whereLabels = map[string]messages.Code{
	WhereCode: messages.H8852, WhereComment: messages.H8853, WhereString: messages.H8854,
	WhereDoc: messages.H8855, WhereData: messages.H8856,
}

// RenameText yeniden adlandırma etkisini insan için düz metne çevirir: hedefler, çakışmalar, kesin
// ve olası yerler (her yer dosya:satır:sütun, rol ya da eşleşme, satırın metni).
func RenameText(r *Result) string {
	var b strings.Builder
	switch r.Status {
	case StatusNotFound:
		fmt.Fprintln(&b, messages.Text(messages.H8782, r.Query.Text))
		for _, n := range r.DidYouMean {
			fmt.Fprintf(&b, "  %s\n", Short(n))
		}
		return b.String()
	case StatusRejected:
		if r.Reason == ReasonAmbiguous {
			fmt.Fprintln(&b, messages.Text(messages.H8780, r.Query.Text, len(r.Candidates)))
			for _, n := range r.Candidates {
				fmt.Fprintf(&b, "  %s\n", messages.Text(messages.H8781, Short(n), n.ID))
			}
			return b.String()
		}
		fmt.Fprintln(&b, rejectionText(r))
		return b.String()
	}
	fmt.Fprintln(&b, messages.Text(messages.H8835, r.Query.Text, r.NewName))
	if r.Mode == ModeTerm {
		fmt.Fprintln(&b, messages.Text(messages.H8866))
		writeLikely(&b, r)
		return b.String()
	}
	fmt.Fprintln(&b, messages.Text(messages.H8836, len(r.Targets)))
	for _, n := range r.Targets {
		fmt.Fprintf(&b, "  %s\n", Short(n))
	}
	for _, c := range r.Conflicts {
		fmt.Fprintln(&b, messages.Text(messages.H8839, r.NewName, Short(c.With)))
	}
	fmt.Fprintln(&b, messages.Text(messages.H8837, len(r.Certain)))
	for _, c := range r.Certain {
		writeChange(&b, c)
	}
	writeLikely(&b, r)
	return b.String()
}

// writeLikely olası yerleri, atlanan başka sözcük sayısını ve kesilmişse sınır notunu yazar.
func writeLikely(b *strings.Builder, r *Result) {
	fmt.Fprintln(b, messages.Text(messages.H8838, len(r.Likely)))
	for _, c := range r.Likely {
		writeChange(b, c)
	}
	if r.SkippedWords > 0 {
		fmt.Fprintln(b, messages.Text(messages.H8840, r.SkippedWords))
	}
	if r.Truncated {
		fmt.Fprintf(b, "... %s\n", messages.Text(messages.H8791, optionName(codes.K107), optionName(codes.K108)))
	}
}

// rejectionText reddedilen yeniden adlandırmanın nedenidir.
func rejectionText(r *Result) string {
	switch r.Reason {
	case ReasonInvalidName:
		return messages.Text(messages.H8844, r.NewName)
	case ReasonLanguageWord:
		return messages.Text(messages.H8845, r.NewName)
	case ReasonSameName:
		return messages.Text(messages.H8846, r.NewName)
	}
	if c, ok := reasonMessages[r.Reason]; ok {
		return messages.Text(c)
	}
	return r.Reason
}

func writeChange(b *strings.Builder, c Change) {
	var what string
	if c.Match == "" {
		what = RoleLabel(c.Role)
	} else {
		what = messages.Text(matchLabels[c.Match])
	}
	line := fmt.Sprintf("    %-22s %s", c.Pos, what)
	if c.Basis == BasisLocalType {
		line += "  " + messages.Text(messages.H8862)
	}
	if c.Where != "" && c.Match != MatchMemberGuess && c.Match != MatchShadowed {
		line += "  " + messages.Text(whereLabels[c.Where])
	}
	if c.Check != "" {
		line += "  " + messages.Text(messages.H8857)
	}
	if c.Word != "" && c.Match != MatchExact {
		line += fmt.Sprintf("  %s → %s", c.Word, c.Replacement)
	}
	if c.Context != "" {
		line += "  | " + c.Context
	}
	b.WriteString(line + "\n")
}
