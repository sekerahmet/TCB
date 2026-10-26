package main

import (
	"go/ast"
	"go/types"
	"maps"
	"slices"
	"strings"
)

// Test TR metnini bir dosyaya yazıp `tr` ikilisiyle de koşar (TestReadyModelMatchesTransformers: `hazır_model(...)`
// karsilastir.tr'ye yazılır, alt süreç onu çalıştırır). Dilin dosya uzantısı .tr'dir: yolu .tr ile biten dosyaya yazılan
// metin TR programıdır. Dosya yazan giriş noktası os.WriteFile'dır (fileSinks); içeriğini ve yolunu kendi
// parametrelerinden ileten işlev de dosya yazar (testkit.WritePath, testkit.WriteFile, testin yardımcısı: sabit
// noktaya kadar). Yolun metni .tr ile bitiyorsa içerik TR metnidir; içerik bir parametreyse o parametre TR metni alır.

// fileSinks dosya yazan giriş noktalarıdır: içerik parametresi -> yol parametresinin sırası. Kökü os.WriteFile'dır;
// ötekiler ondan türer (linkSnippets).
var fileSinks = map[paramKey]int{{memberID("os", "", "WriteFile"), 1}: 0}

// isTextType tip metin taşıyor mu: string, ...string ya da []byte (dosyaya yazılan içerik).
func isTextType(t types.Type) bool {
	if s, ok := t.(*types.Slice); ok {
		if b, ok := s.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Byte {
			return true
		}
	}
	return isStringType(t)
}

// pathTexts yolu ya da metni parçalarından kuran standart işlevlerin metnidir: filepath.Join ve path.Join parçaları "/"
// ile, FromSlash, ToSlash ve Clean metnin kendisi, strings.Join yazılı listenin öğelerini ayraçla birleştirir. Dosyaya
// yazılan TR metninin yolu (`filepath.Join(dir, "karsilastir.tr")`) ve satırlardan kurulan program böyle okunur.
func (x *goExtractor) pathTexts(fc *fileCtx, fn *types.Func, c *ast.CallExpr) ([]textSrc, bool) {
	switch pkg := fn.Pkg().Path(); {
	case (pkg == "path/filepath" || pkg == "path") && fn.Name() == "Join":
		var lists [][]textSrc
		for _, a := range c.Args {
			lists = append(lists, x.textSources(fc, a))
		}
		return joinTexts(lists, "/"), true
	case pkg == "path/filepath" && (fn.Name() == "FromSlash" || fn.Name() == "ToSlash" || fn.Name() == "Clean") && len(c.Args) == 1:
		return x.textSources(fc, c.Args[0]), true
	case pkg == "strings" && fn.Name() == "Join" && len(c.Args) == 2:
		list, ok := c.Args[0].(*ast.CompositeLit)
		sep := x.textSources(fc, c.Args[1])
		if !ok || len(sep) != 1 || len(sep[0].parts) > 0 || sep[0].call != "" {
			return nil, false
		}
		var lists [][]textSrc
		for _, e := range list.Elts {
			lists = append(lists, x.textSources(fc, e))
		}
		return joinTexts(lists, sep[0].lit), true
	}
	return nil, false
}

// paramArg argümanın parametre olabilecek kökü: dönüşüm (`[]byte(içerik)`) ve yol işlevi (`filepath.Join(dir, ad)`'ın
// son parçası, FromSlash) açılır; testkit.WriteFile içeriğini ve adını böyle iletir.
func paramArg(fc *fileCtx, e ast.Expr) ast.Expr {
	for {
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) == 0 {
			return e
		}
		if fn, ok := fc.u.Info.Uses[nameIdent(c.Fun)].(*types.Func); !isConversion(fc, c) && (!ok || fn.Pkg() == nil || fn.Pkg().Path() != "path/filepath") {
			return e
		}
		e = c.Args[len(c.Args)-1]
	}
}

// linkFileWrites dosya yazan işlevleri sabit noktaya kadar genişletir ve yolu .tr ile biten dosyaya giden içeriği TR
// metni sayar. Bir işlev os.WriteFile'a (ya da ondan türeyen birine) içeriği ve yolu kendi parametrelerinden veriyorsa
// o da dosya yazar. Yolun metni bilinen ve .tr ile biten bir çağrıda içerik çağıranın parametresiyse o parametre TR
// metni alır (sinks), metinse akış TR metnidir (trFile).
func (st *snippetState) linkFileWrites() {
	sinks := maps.Clone(fileSinks)
	bySite := map[int][]int{}
	for i, f := range st.flows {
		bySite[f.site] = append(bySite[f.site], i)
	}
	pathOf := func(content snippetFlow) *snippetFlow {
		at, ok := sinks[content.callee]
		if !ok {
			return nil
		}
		for _, j := range bySite[content.site] {
			if p := &st.flows[j]; p.callee == (paramKey{content.callee.fn, at}) {
				return p
			}
		}
		return nil
	}
	for changed := true; changed; {
		changed = false
		for _, f := range st.flows {
			if p := pathOf(f); p != nil && f.from.fn != "" && p.from.fn == f.from.fn {
				if _, seen := sinks[f.from]; !seen {
					sinks[f.from] = p.from.idx
					changed = true
				}
			}
		}
	}
	for i := range st.flows {
		f := &st.flows[i]
		p := pathOf(*f)
		if p == nil || !slices.ContainsFunc(st.expandTexts(p.texts, 0), func(s string) bool { return strings.HasSuffix(s, ".tr") }) {
			continue
		}
		if f.from.fn != "" {
			st.sinks[f.from] = true
		} else {
			f.trFile = true
		}
	}
}
