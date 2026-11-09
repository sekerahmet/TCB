package main

import (
	"slices"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// isTRName düğümün çıplak adı trByName ile mi çözülür: .tr dosyası, TR bildirimi ya da testi, ad kodu.
func isTRName(n *Node) bool {
	return strings.HasPrefix(n.ID, "tr:") || strings.HasPrefix(n.ID, "name:")
}

// trName çıplak ya da nitelenmiş adla çözülen bir TR düğümüdür: sınıfı, çıplak adları ve nitelenmiş adları.
type trName struct {
	node      *Node
	class     codeindex.NameClass
	names     []string
	folded    []string
	qualified [][2]string
}

// trNameTable grafiğin TR ad düğümlerini bir kez toplar, adlar codeindex'inkilerdir: modülün adı ve takma adları,
// bildirimin kısa adı (kodlu bildirimde dillerdeki adları da) ve nitelenmiş adları, testin açıklaması, ad kodunun
// kendisi ve dillerdeki adları.
func (ix *index) trNameTable() []trName {
	if ix.trNames != nil {
		return ix.trNames
	}
	ix.trNames = []trName{}
	for i := range ix.g.Nodes {
		n := &ix.g.Nodes[i]
		c := trName{node: n}
		switch {
		case strings.HasPrefix(n.ID, "tr:file:"):
			c.class, c.names = codeindex.ClassModule, append([]string{n.Name}, n.Aliases...)
			for _, a := range n.Aliases {
				c.qualified = append(c.qualified, [2]string{a, codeindex.Fold(a)})
			}
		case strings.HasPrefix(n.ID, "name:"):
			c.class, c.names = codeindex.ClassBuiltin, nameNodeNames(n)
		case n.Kind == kindTRTest:
			c.class, c.names = codeindex.ClassTest, []string{n.Name[strings.Index(n.Name, ": ")+2:]} // "modül: açıklama"
			c.qualified = [][2]string{{n.Name, codeindex.Fold(n.Name)}}
		case strings.HasPrefix(n.ID, "tr:"):
			c.class, c.names = codeindex.ClassDeclaration, declNames(n)
			c.qualified = [][2]string{{n.Name, codeindex.Fold(n.Name)}}
			for _, a := range n.Aliases {
				if strings.Contains(a, ".") && !codeindex.IdentityQuery(a) {
					c.qualified = append(c.qualified, [2]string{a, codeindex.Fold(a)})
				}
			}
		default:
			continue
		}
		for _, name := range c.names {
			c.folded = append(c.folded, codeindex.Fold(name))
		}
		ix.trNames = append(ix.trNames, c)
	}
	return ix.trNames
}

// trByName bir sorgunun TR adaylarıdır; codeindex.Reader.Resolve'uyla aynı kural (codeindex.NameKeys harf kuralı,
// codeindex.PickByName çıplak ad sırası, codeindex.QualifiedMatch nitelenmiş ad kuralı ve codeindex.MemberQuery üye
// yazımı geri düşüşü), böylece `query`, `codeindex` tanım ve yeniden adlandırma sorguları aynı aday kümesini görür.
// folded adaylar büyük harfli sorgunun harfsiz geçişinden geldiyse true'dur. Grafikte codeindex'in bilmediği ad
// kodları da vardır (anahtar sözcükler, kullanılmayan yerleşikler); onlar da yerleşik sınıfındadır.
func (ix *index) trByName(q string, kinds []string) (found []*Node, folded bool) {
	table := ix.trNameTable()
	for ki, k := range codeindex.NameKeys(q) {
		picked := codeindex.PickByName(len(table), func(i int) codeindex.NameClass {
			c := &table[i]
			if len(kinds) > 0 && !contains(kinds, c.node.Kind) {
				return codeindex.ClassNone
			}
			for j, name := range c.names {
				if k.Match(name, c.folded[j]) {
					return c.class
				}
			}
			return codeindex.ClassNone
		})
		if len(picked) == 0 && !codeindex.IdentityQuery(q) {
			picked = codeindex.QualifiedMatch(len(table), k, func(i int) [][2]string {
				c := &table[i]
				if len(kinds) > 0 && !contains(kinds, c.node.Kind) {
					return nil
				}
				return c.qualified
			})
		}
		for _, i := range picked {
			found = append(found, table[i].node)
		}
		if len(found) > 0 {
			return found, ki > 0
		}
	}
	return ix.trMember(q, kinds, table)
}

func (ix *index) memberQuery(q string, table []trName) (string, []int) {
	return codeindex.MemberQuery(q, len(table), func(i int) codeindex.Kind {
		switch {
		case table[i].node.Kind == kindTRStruct:
			return codeindex.KindStruct
		case table[i].class == codeindex.ClassModule:
			return codeindex.KindModule
		}
		return ""
	}, func(i int) [][2]string { return table[i].qualified })
}

func (ix *index) trMember(q string, kinds []string, table []trName) (found []*Node, folded bool) {
	mq, types := ix.memberQuery(q, table)
	if mq == "" {
		return nil, false
	}
	vals, f := ix.trByName(mq, kinds)
	if len(types) == 0 || len(vals) == 0 {
		return vals, f
	}
	recv, member, _ := strings.Cut(q, ".")
	for _, i := range types {
		cp := *table[i].node
		if table[i].class == codeindex.ClassModule {
			cp.NoteCode, cp.Note = codeindex.MemberModuleNoteIn("en", recv)
		} else {
			cp.NoteCode, cp.Note = codeindex.MemberTypeNoteIn("en", recv)
		}
		found = append(found, &cp)
	}
	for _, v := range vals {
		cp := *v
		cp.NoteCode, cp.Note = codeindex.MemberValueNoteIn("en", recv, member)
		found = append(found, &cp)
	}
	return found, false
}

var knownNonSpecMembers = sync.OnceValue(func() map[string]bool {
	known := map[string]bool{}
	for _, k := range codes.Codes() {
		known[codeindex.Fold(k.String())] = true
		known[strings.ToLower(k.String())] = true
		if p, ok := codes.PlaceOf(k); ok && (p == codes.PlaceOther || p == codes.PlaceMember) {
			s := codes.Spelling(k)
			known[codeindex.Fold(s)] = true
			known[strings.ToLower(s)] = true
		}
	}
	return known
})

func isCodedOrNonMember(member string) bool {
	m := knownNonSpecMembers()
	return m[codeindex.Fold(member)] || m[strings.ToLower(member)]
}

// specMember TR'de henüz kodu olmayan, dil tanımının numaralı maddelerinde (başlıkta ya da gövdede) `.TR_<ad>` olarak
// geçen üyeleri alıcı adından bağımsız olarak `spec:` adaylarına çözer.
func (ix *index) specMember(q string, kinds []string) []*Node {
	if len(kinds) > 0 && !contains(kinds, kindSpec) {
		return nil
	}
	recv, member, ok := strings.Cut(q, ".")
	if !ok {
		return nil
	}
	mq := "x." + member
	if !strings.EqualFold(recv, "x") {
		if mq, _ = ix.memberQuery(q, ix.trNameTable()); mq == "" {
			return nil
		}
	}
	if vals, _ := ix.trByName(mq, nil); len(vals) > 0 || isCodedOrNonMember(member) {
		return nil
	}
	dot := "." + member
	if found := ix.matchSpecMember(dot, func(n *Node) []string { return n.Aliases }); len(found) > 0 {
		return found
	}
	return ix.matchSpecMember(dot, func(n *Node) []string {
		if len(n.Aliases) > 0 {
			return nil
		}
		return dotMemberTokens(n.Detail)
	})
}

func (ix *index) matchSpecMember(dot string, tokens func(*Node) []string) []*Node {
	for _, k := range codeindex.NameKeys(dot) {
		var found []*Node
		for i := range ix.g.Nodes {
			n := &ix.g.Nodes[i]
			if n.Kind == kindSpec && slices.ContainsFunc(tokens(n), func(a string) bool { return k.Match(a, codeindex.Fold(a)) }) {
				found = append(found, n)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}
