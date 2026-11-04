package main

import (
	"go/types"
	"path"
	"sort"
	"strings"
)

type declPosKey struct {
	file string
	off  int
}

type useFile struct {
	rel    string
	isTest bool
}

type typeUseIndex struct {
	usesByDecl map[declPosKey][]useFile
	selByDecl  map[declPosKey][]useFile
	pkgUsers   map[string][]useFile
	pkgFiles   map[string][]useFile
	concretes  []*types.Named
}

func linkTypeQueries(b *builder, idx *goIndex, m *module, reach *testReach) {
	st := idx.typeQueries
	if len(st.loaders) == 0 || len(st.sites) == 0 {
		return
	}
	caps := propagateQueryCaps(b, st.caps)
	scopes := resolveTestLoadScopes(b, st.loaders, reach)
	if len(scopes) == 0 {
		return
	}
	uses := buildTypeUseIndex(m)
	for _, site := range st.sites {
		tests := reach.testsWithin(site.owner, testMaxSteps/2)
		for _, t := range tests {
			tScopes := scopes[t]
			if len(tScopes) == 0 {
				continue
			}
			c := caps[site.callee]
			if !c.any() {
				c = caps[site.owner].merge(caps[t])
			}
			if !c.any() {
				continue
			}
			targets := map[string]bool{}
			for _, sc := range tScopes {
				collectSiteTargets(m, uses, sc, site, c, targets)
			}
			selfFile := ""
			if tn := b.nodes[t]; tn != nil {
				selfFile = fileID(tn.File)
			}
			for target := range targets {
				if target != "" && target != selfFile && b.nodes[target] != nil {
					b.addEdge(t, target, edgeTests, inferred, site.file, site.line)
				}
			}
		}
	}
}

func propagateQueryCaps(b *builder, seed map[string]queryCap) map[string]queryCap {
	caps := map[string]queryCap{}
	for k, v := range seed {
		caps[k] = v
	}
	callers := map[string][]string{}
	for k, e := range b.edges {
		switch {
		case k.kind == edgeCalls && e.Confidence == extracted:
			callers[k.to] = append(callers[k.to], k.from)
		case k.kind == edgeContains && b.nodes[k.to] != nil && b.nodes[k.to].Kind == kindClosure:
			callers[k.to] = append(callers[k.to], k.from)
		}
	}
	for changed := true; changed; {
		changed = false
		for callee, c := range caps {
			if !c.any() {
				continue
			}
			for _, caller := range callers[callee] {
				merged := caps[caller].merge(c)
				if merged != caps[caller] {
					caps[caller] = merged
					changed = true
				}
			}
		}
	}
	return caps
}

func resolveTestLoadScopes(b *builder, loaders []typeLoader, reach *testReach) map[string][]loadScope {
	out := map[string][]loadScope{}
	add := func(t string, sc loadScope) {
		for _, cur := range out[t] {
			if cur.incTests == sc.incTests && cur.singlePkg == sc.singlePkg && len(cur.bases) == len(sc.bases) {
				same := true
				for i := range cur.bases {
					if cur.bases[i] != sc.bases[i] {
						same = false
						break
					}
				}
				if same {
					return
				}
			}
		}
		out[t] = append(out[t], sc)
	}
	writtenVars := map[string][]string{}
	for k := range b.edges {
		if (k.kind == edgeWritesVar || k.kind == edgeInitsVar) && b.nodes[k.to] != nil && b.nodes[k.to].Kind == kindVar {
			writtenVars[k.from] = append(writtenVars[k.from], k.to)
		}
	}
	for _, l := range loaders {
		for _, t := range reach.testsReaching(l.id) {
			add(t, l.scope)
		}
		walkCallers(l.id, func(id string) []string { return reach.callers[id] }, reach.pkgOf, func(r reached) bool {
			if r.Steps > 4 {
				return false
			}
			for _, v := range writtenVars[r.ID] {
				for _, t := range reach.testsReaching(v) {
					add(t, l.scope)
				}
			}
			return true
		})
	}
	return out
}

func buildTypeUseIndex(m *module) *typeUseIndex {
	idx := &typeUseIndex{
		usesByDecl: map[declPosKey][]useFile{},
		selByDecl:  map[declPosKey][]useFile{},
		pkgUsers:   map[string][]useFile{},
		pkgFiles:   map[string][]useFile{},
	}
	idx.concretes, _, _ = moduleImplementations(m)
	for _, u := range m.Units {
		basePkg := strings.TrimSuffix(u.PkgPath, "_test")
		for _, f := range u.Files {
			rel := m.Rel(m.Fset.Position(f.Pos()).Filename)
			if rel != "" {
				idx.pkgFiles[basePkg] = append(idx.pkgFiles[basePkg], useFile{rel, isTestFile(rel)})
			}
		}
		for id, o := range u.Info.Uses {
			if o == nil || o.Pkg() == nil {
				continue
			}
			rel := m.Rel(m.Fset.Position(id.Pos()).Filename)
			if rel == "" {
				continue
			}
			uf := useFile{rel, isTestFile(rel)}
			idx.usesByDecl[declKeyOf(m, o)] = append(idx.usesByDecl[declKeyOf(m, o)], uf)
			if o.Pkg().Path() != u.PkgPath && isModPkg(m.Path, o.Pkg().Path()) {
				idx.pkgUsers[o.Pkg().Path()] = append(idx.pkgUsers[o.Pkg().Path()], uf)
			}
		}
		for se, sel := range u.Info.Selections {
			if sel == nil {
				continue
			}
			rel := m.Rel(m.Fset.Position(se.Pos()).Filename)
			if rel == "" {
				continue
			}
			uf := useFile{rel, isTestFile(rel)}
			seen := map[declPosKey]bool{}
			collectTypeDecls(m, sel.Recv(), seen)
			collectTypeDecls(m, sel.Type(), seen)
			for dk := range seen {
				idx.selByDecl[dk] = append(idx.selByDecl[dk], uf)
			}
		}
	}
	dedupUseMap(idx.usesByDecl)
	dedupUseMap(idx.selByDecl)
	dedupUseStringMap(idx.pkgUsers)
	dedupUseStringMap(idx.pkgFiles)
	return idx
}

func collectTypeDecls(m *module, t types.Type, out map[declPosKey]bool) {
	switch t := t.(type) {
	case *types.Pointer:
		collectTypeDecls(m, t.Elem(), out)
	case *types.Slice:
		collectTypeDecls(m, t.Elem(), out)
	case *types.Named:
		if t.Obj() != nil && t.Obj().Pkg() != nil && isModPkg(m.Path, t.Obj().Pkg().Path()) {
			out[declKeyOf(m, t.Obj())] = true
		}
	case *types.Signature:
		for i := 0; i < t.Params().Len(); i++ {
			collectTypeDecls(m, t.Params().At(i).Type(), out)
		}
		for i := 0; i < t.Results().Len(); i++ {
			collectTypeDecls(m, t.Results().At(i).Type(), out)
		}
	}
}

func collectSiteTargets(m *module, uses *typeUseIndex, sc loadScope, site typeQuerySite, c queryCap, out map[string]bool) {
	if site.kind&queryPkg != 0 && c.uses {
		if pkgPath := resolveQueryPkg(m, sc, site.spec); pkgPath != "" {
			for _, uf := range uses.pkgFiles[pkgPath] {
				if sc.matches(uf) {
					out[fileID(uf.rel)] = true
				}
			}
			for _, uf := range uses.pkgUsers[pkgPath] {
				if sc.matches(uf) {
					out[fileID(uf.rel)] = true
				}
			}
			return
		}
	}
	if site.kind&queryObj == 0 {
		return
	}
	for _, obj := range resolveQueryObjects(m, sc, site.spec, site.pkgHint) {
		addObjectDeclFile(m, obj, out)
		dk := declKeyOf(m, obj)
		if c.uses {
			for _, uf := range uses.usesByDecl[dk] {
				if sc.matches(uf) {
					out[fileID(uf.rel)] = true
				}
			}
		}
		if c.selections {
			for _, uf := range uses.selByDecl[dk] {
				if sc.matches(uf) {
					out[fileID(uf.rel)] = true
				}
			}
		}
		if c.impl {
			addInterfaceImplTargets(m, uses, sc, obj, c.selections, out)
		}
	}
}

func addInterfaceImplTargets(m *module, uses *typeUseIndex, sc loadScope, obj types.Object, includeSels bool, out map[string]bool) {
	iface := interfaceOfObj(obj)
	if iface == nil || iface.NumMethods() == 0 {
		return
	}
	for _, ct := range uses.concretes {
		if !sc.matchesPkg(m, ct.Obj().Pkg().Path()) {
			continue
		}
		if !types.Implements(ct, iface) && !types.Implements(types.NewPointer(ct), iface) {
			continue
		}
		addObjectDeclFile(m, ct.Obj(), out)
		if includeSels {
			for _, uf := range uses.selByDecl[declKeyOf(m, ct.Obj())] {
				if sc.matches(uf) {
					out[fileID(uf.rel)] = true
				}
			}
		}
		ms := types.NewMethodSet(types.NewPointer(ct))
		for k := 0; k < iface.NumMethods(); k++ {
			im := iface.Method(k)
			if sel := ms.Lookup(im.Pkg(), im.Name()); sel != nil {
				if fn, ok := sel.Obj().(*types.Func); ok {
					addObjectDeclFile(m, fn, out)
				}
			}
		}
	}
}

func addObjectDeclFile(m *module, obj types.Object, out map[string]bool) {
	if obj == nil || obj.Pkg() == nil || !isModPkg(m.Path, obj.Pkg().Path()) {
		return
	}
	if rel := m.Rel(m.Fset.Position(obj.Pos()).Filename); rel != "" {
		out[fileID(rel)] = true
	}
}

func resolveQueryPkg(m *module, sc loadScope, spec string) string {
	var hits []string
	for p := range m.Base {
		if !sc.matchesPkg(m, p) {
			continue
		}
		short := strings.TrimPrefix(strings.TrimPrefix(p, m.Path), "/")
		if p == spec || short == spec || path.Base(p) == spec {
			hits = append(hits, p)
		}
	}
	if len(hits) == 1 {
		return hits[0]
	}
	return ""
}

func resolveQueryObjects(m *module, sc loadScope, spec, hint string) []types.Object {
	if strings.Contains(spec, ".") {
		for i := len(spec) - 1; i > 0; i-- {
			if spec[i] != '.' {
				continue
			}
			pkgPath := resolveQueryPkg(m, sc, spec[:i])
			if pkgPath == "" {
				continue
			}
			if obj := lookupPkgObject(m.Base[pkgPath], spec[i+1:]); obj != nil {
				return []types.Object{obj}
			}
			return nil
		}
		return nil
	}
	if sc.singlePkg != "" && m.Base[sc.singlePkg] != nil {
		if obj := lookupPkgObject(m.Base[sc.singlePkg], spec); obj != nil {
			return []types.Object{obj}
		}
		return nil
	}
	if hint != "" {
		if pkgPath := resolveQueryPkg(m, sc, hint); pkgPath != "" {
			if obj := lookupPkgObject(m.Base[pkgPath], spec); obj != nil {
				return []types.Object{obj}
			}
			return nil
		}
	}
	var hits []types.Object
	for p, pkg := range m.Base {
		if sc.matchesPkg(m, p) {
			if obj := pkg.Scope().Lookup(spec); obj != nil {
				hits = append(hits, obj)
			}
		}
	}
	if len(hits) == 1 {
		return hits
	}
	return nil
}

func lookupPkgObject(pkg *types.Package, names string) types.Object {
	name, member, hasMember := strings.Cut(names, ".")
	obj := pkg.Scope().Lookup(name)
	if obj == nil || !hasMember {
		return obj
	}
	o, _, _ := types.LookupFieldOrMethod(obj.Type(), true, pkg, member)
	return o
}

func interfaceOfObj(obj types.Object) *types.Interface {
	switch o := obj.(type) {
	case *types.TypeName:
		it, _ := o.Type().Underlying().(*types.Interface)
		return it
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			it, _ := sig.Recv().Type().Underlying().(*types.Interface)
			return it
		}
	}
	return nil
}

func (sc loadScope) matches(uf useFile) bool {
	if !sc.incTests && uf.isTest {
		return false
	}
	dir := path.Dir(uf.rel)
	for _, b := range sc.bases {
		if b == "." {
			return true
		}
		if sc.singlePkg != "" {
			if dir == b {
				return true
			}
			continue
		}
		if dir == b || strings.HasPrefix(dir, b+"/") || uf.rel == b {
			return true
		}
	}
	return false
}

func (sc loadScope) matchesPkg(m *module, pkgPath string) bool {
	if sc.singlePkg != "" {
		return pkgPath == sc.singlePkg
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(pkgPath, m.Path), "/")
	if rel == "" {
		rel = "."
	}
	for _, b := range sc.bases {
		if b == "." || rel == b || strings.HasPrefix(rel, b+"/") {
			return true
		}
	}
	return false
}

func declKeyOf(m *module, obj types.Object) declPosKey {
	p := m.Fset.Position(obj.Pos())
	return declPosKey{p.Filename, p.Offset}
}

func isModPkg(modPath, p string) bool {
	return p == modPath || strings.HasPrefix(p, modPath+"/")
}

func dedupUseMap(m map[declPosKey][]useFile) {
	for k, list := range m {
		m[k] = compactUseFiles(list)
	}
}

func dedupUseStringMap(m map[string][]useFile) {
	for k, list := range m {
		m[k] = compactUseFiles(list)
	}
}

func compactUseFiles(list []useFile) []useFile {
	sort.Slice(list, func(i, j int) bool { return list[i].rel < list[j].rel })
	var out []useFile
	for i, u := range list {
		if i == 0 || u.rel != list[i-1].rel {
			out = append(out, u)
		}
	}
	return out
}
