package main

import (
	"cmp"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

const (
	trMaxHelperSteps = 8
	trChatGuard      = "tokenizer_config.json"
)

type trFolderLoaderSpec struct {
	modelTypes, always, modelFiles []string
	chatFamily                     bool
}

var (
	trModelLoaders = map[string]trFolderLoaderSpec{
		"vits":       {modelTypes: []string{"vits"}, always: []string{"config.json"}, modelFiles: []string{"model.safetensors", "vocab.json", "tokenizer_config.json"}},
		"whisper":    {modelTypes: []string{"whisper"}, always: []string{"config.json"}, modelFiles: []string{"model.safetensors", "tokenizer.json", "generation_config.json"}},
		"pretrained": {modelTypes: []string{"llama", "qwen3", "gemma3", "gemma3_text"}, always: []string{"config.json"}, modelFiles: []string{"model.safetensors"}, chatFamily: true},
	}
	trChatFiles = []string{"tokenizer.json", "generation_config.json"}
)

type trLinker struct {
	root               string
	b                  *builder
	lang               *codes.Language
	single0, pathProps map[string]bool
	loaders            map[string][2]string
	byFile             map[string][]*Node
	targets            []*Node
}

// linkTRPaths extractTR ve extractDataFiles tamamlandıktan sonra 18.16 TR test yollarını bağlar.
func linkTRPaths(root string, b *builder) {
	if b.trIdx == nil {
		return
	}
	lang := codes.Active()
	setOf := func(ks ...codes.Code) map[string]bool {
		m := make(map[string]bool, len(ks))
		for _, k := range ks {
			m[lang.Name(k)] = true
		}
		return m
	}
	byFile, all := map[string][]*Node{}, []*Node(nil)
	for _, n := range b.nodes {
		if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
			all = append(all, n)
		}
	}
	slices.SortFunc(all, func(a, c *Node) int { return cmp.Compare(a.ID, c.ID) })
	for _, n := range all {
		byFile[n.File] = append(byFile[n.File], n)
	}
	l := &trLinker{
		root: root, b: b, lang: lang, byFile: byFile, targets: all,
		single0:   setOf(codes.FnReadFile, codes.FnReadBytes, codes.FnReadLines, codes.FnReadCsv, codes.FnReadWav, codes.FnLoadTokenizer, codes.FnLoadTensors, codes.FnLoadMetadata, codes.FnPretrainedGguf, codes.FnPretrainedVad, codes.FnCopy),
		pathProps: setOf(codes.MemExists, codes.MemIsFile, codes.MemIsDir, codes.MemSize, codes.MemAbsolute),
		loaders:   map[string][2]string{lang.Name(codes.FnPretrainedVits): {"vits", "yol"}, lang.Name(codes.FnPretrainedWhisper): {"whisper", "yol"}, lang.Name(codes.FnPretrainedModel): {"pretrained", lang.Name(codes.ArgFolder)}},
	}
	for _, rel := range b.trIdx.Files {
		if !strings.Contains(rel, ":") {
			l.linkFile(rel)
		}
	}
}

type trVal struct {
	strs            []string
	isPath, tainted bool
}

type trEnv struct {
	consts, vars map[string]trVal
	funcs        map[string]*ast.FunctionLiteral
	funcOwners   map[string][]string
	rel, testID  string
	outer, depth int
	visited      map[string]bool
}

func (e *trEnv) lineAt(line int) int {
	if e.outer > 0 {
		return e.outer
	}
	return line
}

func (l *trLinker) linkFile(rel string) {
	rawBytes, err := os.ReadFile(filepath.Join(l.root, filepath.FromSlash(rel)))
	if err != nil {
		return
	}
	raw := string(rawBytes)
	sb, err := source.Split(raw)
	prog := parseTRSource(sb, err == nil && sb.Coded)
	if prog == nil {
		return
	}
	consts, funcs, funcOwners := map[string]trVal{}, map[string]*ast.FunctionLiteral{}, map[string][]string{}
	var tests []*ast.TestStatement
	for _, s := range prog.Statements {
		switch s := s.(type) {
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
				funcs[fl.Name] = fl
			}
		case *ast.TestStatement:
			tests = append(tests, s)
		}
	}
	top := &trEnv{consts: consts, vars: map[string]trVal{}, funcs: funcs, rel: rel, visited: map[string]bool{}}
	for _, s := range prog.Statements {
		if v, ok := s.(*ast.VarStatement); ok && v.IsConst && v.Value != nil {
			consts[v.Name.Value] = l.evalExpr(v.Value, top)
		}
	}
	for _, ts := range tests {
		if tid := l.b.trIDs[codeindex.NodeID(codeindex.KindTest, rel, strconv.Itoa(ts.Token.Line))]; tid != "" && l.b.nodes[tid] != nil {
			l.execScope(ts.Body, &trEnv{consts: consts, vars: map[string]trVal{}, funcs: funcs, funcOwners: funcOwners, rel: rel, testID: tid, visited: map[string]bool{}})
		}
	}
	if strings.HasSuffix(rel, "_test.tr") {
		l.linkComments(rel, raw, funcOwners)
	}
}

func parseTRSource(sb source.Parts, ok bool) *ast.Program {
	for _, mod := range []bool{false, true} {
		if !ok {
			break
		}
		p := parser.New(sb.Lexer())
		prog := p.ParseProgram()
		if mod {
			prog = p.ParseModule()
		}
		if len(p.Errs()) == 0 && prog != nil {
			return prog
		}
	}
	return nil
}

func (l *trLinker) execScope(root any, env *trEnv) (rets []trVal) {
	if root == nil {
		return nil
	}
	ast.Walk(root, func(n any) bool {
		if n == root {
			return true
		}
		switch s := n.(type) {
		case *ast.BlockStatement:
			saved := maps.Clone(env.vars)
			rets, env.vars = append(rets, l.execScope(s, env)...), saved
		case *ast.VarStatement:
			env.vars[s.Name.Value] = l.evalExpr(s.Value, env)
		case *ast.MultiVarStatement:
			for _, v := range s.Values {
				l.evalExpr(v, env)
			}
			for _, id := range s.Names {
				env.vars[id.Value] = trVal{}
			}
		case *ast.AssignStatement:
			l.evalExpr(s.Target, env)
			v := l.evalExpr(s.Value, env)
			if id, ok := s.Target.(*ast.Identifier); ok {
				env.vars[id.Value] = v
			}
		case *ast.ForInStatement:
			iter, saved := l.evalExpr(s.Iterable, env), maps.Clone(env.vars)
			if s.KeyVar != "" {
				env.vars[s.KeyVar] = trVal{}
			}
			if s.ValueVar != "" {
				env.vars[s.ValueVar] = trVal{strs: iter.strs, tainted: iter.tainted}
			}
			rets, env.vars = append(rets, l.execScope(s.Body, env)...), saved
		case *ast.ReturnStatement:
			if s.ReturnValue != nil {
				rets = append(rets, l.evalExpr(s.ReturnValue, env))
			}
		case *ast.FunctionLiteral:
			saved := maps.Clone(env.vars)
			for _, p := range s.Parameters {
				env.vars[p.Name] = trVal{}
			}
			l.execScope(s.Body, env)
			env.vars = saved
		case *ast.CallExpression, *ast.MemberExpression:
			l.evalExpr(n.(ast.Expression), env)
		default:
			return true
		}
		return false
	})
	return rets
}

func (l *trLinker) evalExpr(expr ast.Expression, env *trEnv) trVal {
	switch e := expr.(type) {
	case nil:
		return trVal{}
	case *ast.StringLiteral:
		return trVal{strs: []string{e.Value}}
	case *ast.Identifier:
		if v, ok := env.vars[e.Value]; ok {
			return v
		}
		return env.consts[e.Value]
	case *ast.ListLiteral:
		out, ok := []string(nil), len(e.Elements) > 0
		for _, el := range e.Elements {
			if v := l.evalExpr(el, env); v.tainted || len(v.strs) == 0 {
				ok = false
			} else if ok && len(out)+len(v.strs) <= 32 {
				out = append(out, v.strs...)
			}
		}
		if ok {
			return trVal{strs: out}
		}
	case *ast.InterpolatedString:
		return l.evalInterpolated(e, env)
	case *ast.InfixExpression:
		lv, rv := l.evalExpr(e.Left, env), l.evalExpr(e.Right, env)
		if e.Operator == "+" && (lv.tainted || rv.tainted) {
			return trVal{tainted: true}
		} else if e.Operator == "+" && len(lv.strs) > 0 && len(rv.strs) > 0 {
			return trVal{strs: crossConcat(lv.strs, rv.strs, false)}
		}
	case *ast.MemberExpression:
		if obj := l.evalExpr(e.Object, env); env.testID != "" && l.pathProps[e.Property] && obj.isPath && !obj.tainted {
			l.linkSingleVals(obj, env, env.lineAt(e.Line()))
		}
	case *ast.CallExpression:
		return l.evalCall(e, env)
	case *ast.FunctionLiteral:
		l.execScope(e.Body, env)
	default:
		l.execScope(expr, env)
	}
	return trVal{}
}

func (l *trLinker) evalInterpolated(is *ast.InterpolatedString, env *trEnv) trVal {
	acc := []string{""}
	for _, part := range is.Parts {
		if lit, ok := part.(*ast.StringLiteral); ok {
			acc = crossConcat(acc, []string{lit.Value}, false)
			continue
		}
		if v := l.evalExpr(part, env); v.tainted {
			return trVal{tainted: true}
		} else if len(v.strs) > 0 {
			acc = crossConcat(acc, v.strs, false)
			continue
		}
		if slices.ContainsFunc(acc, func(p string) bool { return !strings.Contains(p, "/") || strings.HasPrefix(p, "*") }) {
			return trVal{}
		}
		acc = crossConcat(acc, []string{"*"}, false)
	}
	return trVal{strs: acc}
}

func crossConcat(bases, suffixes []string, join bool) (out []string) {
	for _, b := range bases {
		for _, s := range suffixes {
			if len(out) < 32 && join {
				out = append(out, path.Join(b, s))
			} else if len(out) < 32 {
				out = append(out, b+s)
			}
		}
	}
	return out
}

func (l *trLinker) evalCall(c *ast.CallExpression, env *trEnv) trVal {
	pos, named := make([]trVal, len(c.Arguments)), map[string]trVal{}
	for i, a := range c.Arguments {
		pos[i] = l.evalExpr(a, env)
	}
	for _, na := range c.NamedArgs {
		named[na.Name] = l.evalExpr(na.Value, env)
	}
	fn, ok := c.Function.(*ast.Identifier)
	if !ok {
		l.evalExpr(c.Function, env)
		return trVal{}
	}
	if _, shadowed := env.vars[fn.Value]; shadowed {
		return trVal{}
	}
	switch {
	case fn.Value == l.lang.Name(codes.FnTempDir):
		return trVal{tainted: true}
	case fn.Value == l.lang.Name(codes.TypeString) && len(pos) == 1:
		return trVal{strs: pos[0].strs, tainted: pos[0].tainted}
	case fn.Value == l.lang.Name(codes.TypeString):
		return trVal{}
	case fn.Value == l.lang.Name(codes.TypePath):
		if len(pos) == 0 {
			return trVal{}
		}
		acc := slices.Clone(pos[0].strs)
		for i, v := range pos {
			if v.tainted || len(v.strs) == 0 {
				return trVal{tainted: v.tainted}
			} else if i > 0 {
				acc = crossConcat(acc, v.strs, true)
			}
		}
		return trVal{strs: acc, isPath: true}
	}
	line := env.lineAt(c.Line())
	if env.testID != "" {
		l.recordBuiltinRead(fn.Value, pos, named, env, line)
	}
	if fl, ok := env.funcs[fn.Value]; ok {
		return l.callHelper(fn.Value, fl, pos, named, env, line)
	}
	return trVal{}
}

func (l *trLinker) recordBuiltinRead(name string, pos []trVal, named map[string]trVal, env *trEnv, line int) {
	pick := func(idx int, keys ...string) trVal {
		if idx < len(pos) {
			return pos[idx]
		}
		for _, k := range keys {
			if v, ok := named[k]; ok {
				return v
			}
		}
		return trVal{}
	}
	if ld, ok := l.loaders[name]; ok {
		l.linkLoaderArg(pick(0, ld[1]), trModelLoaders[ld[0]], env, line)
		return
	}
	switch {
	case l.single0[name]:
		l.linkSingleVals(pick(0), env, line)
	case name == l.lang.Name(codes.FnGgufChatFamily):
		l.linkSingleVals(pick(1, "yol"), env, line)
	case name == l.lang.Name(codes.FnFileResponse):
		if r, q := pick(0), pick(1); !r.tainted && !q.tainted {
			for _, rv := range r.strs {
				for _, qv := range q.strs {
					l.linkSingleVals(trVal{strs: []string{path.Join(rv, strings.TrimPrefix(qv, "/"))}}, env, line)
				}
			}
		}
	case name == l.lang.Name(codes.FnFolderChatFamily):
		if v := pick(1, "k"); !v.tainted {
			for _, p := range v.strs {
				if cand, ok := l.resolveFolder(env.rel, p); ok {
					l.linkChatFamily(env.testID, env.rel, line, cand)
				}
			}
		}
	case name == l.lang.Name(codes.TypeDataset):
		if v := pick(0); !v.tainted {
			for _, p := range v.strs {
				if hits := l.resolveSingle(env.rel, p); len(hits) > 0 {
					l.addTestEdges(env.testID, env.rel, line, hits)
				} else if cand, ok := l.resolveFolder(env.rel, p); ok {
					for _, n := range l.targets {
						if n.File != env.rel && path.Dir(n.File) == cand && (strings.HasSuffix(n.File, ".npy") || path.Base(n.File) == "meta.json") {
							l.b.addEdge(env.testID, n.ID, edgeTests, inferred, env.rel, line)
						}
					}
				}
			}
		}
	}
}

func (l *trLinker) callHelper(name string, fl *ast.FunctionLiteral, pos []trVal, named map[string]trVal, env *trEnv, outer int) trVal {
	if env.depth >= trMaxHelperSteps || env.visited[name] || fl.Body == nil {
		return trVal{}
	}
	if env.testID != "" && !slices.Contains(env.funcOwners[name], env.testID) {
		env.funcOwners[name] = append(env.funcOwners[name], env.testID)
	}
	vis, vars := maps.Clone(env.visited), map[string]trVal{}
	vis[name] = true
	child := &trEnv{consts: env.consts, vars: vars, funcs: env.funcs, funcOwners: env.funcOwners, rel: env.rel, testID: env.testID, outer: outer, depth: env.depth + 1, visited: vis}
	for i, p := range fl.Parameters {
		switch {
		case i < len(pos):
			vars[p.Name] = pos[i]
		case named[p.Name].tainted || len(named[p.Name].strs) > 0:
			vars[p.Name] = named[p.Name]
		case p.DefaultValue != nil:
			vars[p.Name] = l.evalExpr(p.DefaultValue, child)
		default:
			vars[p.Name] = trVal{}
		}
	}
	rets := l.execScope(fl.Body, child)
	if len(rets) == 0 {
		return trVal{}
	}
	out, isPath := []string(nil), true
	for _, r := range rets {
		if r.tainted || len(r.strs) == 0 {
			return trVal{tainted: r.tainted}
		}
		isPath = isPath && r.isPath
		for _, s := range r.strs {
			if !slices.Contains(out, s) && len(out) < 32 {
				out = append(out, s)
			}
		}
	}
	return trVal{strs: out, isPath: isPath}
}

func candTRPaths(callerRel, p string) (out []string) {
	if p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"); p == "" || p == "." || p == "*" || strings.HasPrefix(p, "/") {
		return nil
	}
	for _, c := range []string{path.Clean(p), path.Clean(path.Join(path.Dir(callerRel), p))} {
		if c != "." && c != ".." && !strings.HasPrefix(c, "../") && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

func (l *trLinker) addTestEdges(testID, callerRel string, line int, nodes []*Node) {
	for _, n := range nodes {
		if n.File != callerRel {
			l.b.addEdge(testID, n.ID, edgeTests, inferred, callerRel, line)
		}
	}
}

func (l *trLinker) linkSingleVals(v trVal, env *trEnv, line int) {
	if !v.tainted {
		for _, p := range v.strs {
			l.addTestEdges(env.testID, env.rel, line, l.resolveSingle(env.rel, p))
		}
	}
}

func (l *trLinker) resolveSingle(callerRel, p string) []*Node {
	for _, cand := range candTRPaths(callerRel, p) {
		var hits []*Node
		for _, n := range resolvePattern(cand, l.byFile, l.targets) {
			if info, err := os.Stat(filepath.Join(l.root, filepath.FromSlash(n.File))); n.File != callerRel && (strings.ContainsAny(cand, "*?[") || err != nil || !info.IsDir()) {
				hits = append(hits, n)
			}
		}
		if len(hits) > 0 {
			return hits
		}
	}
	return nil
}

func (l *trLinker) resolveFolder(callerRel, p string) (string, bool) {
	if !strings.ContainsAny(p, "*?[") {
		for _, cand := range candTRPaths(callerRel, p) {
			if info, err := os.Stat(filepath.Join(l.root, filepath.FromSlash(cand))); err == nil && info.IsDir() ||
				slices.ContainsFunc(l.targets, func(n *Node) bool { return strings.HasPrefix(n.File, cand+"/") }) {
				return cand, true
			}
		}
	}
	return "", false
}

func (l *trLinker) linkLoaderArg(v trVal, spec trFolderLoaderSpec, env *trEnv, line int) {
	if v.tainted {
		return
	}
	for _, p := range v.strs {
		cand, ok := l.resolveFolder(env.rel, p)
		if !ok {
			continue
		}
		for _, f := range spec.always {
			l.addTestEdges(env.testID, env.rel, line, l.byFile[cand+"/"+f])
		}
		var cfg struct {
			ModelType string `json:"model_type"`
		}
		if !readJSON(filepath.Join(l.root, filepath.FromSlash(cand), "config.json"), &cfg) || !slices.Contains(spec.modelTypes, cfg.ModelType) {
			continue
		}
		for _, f := range spec.modelFiles {
			l.addTestEdges(env.testID, env.rel, line, l.byFile[cand+"/"+f])
		}
		if spec.chatFamily {
			l.linkChatFamily(env.testID, env.rel, line, cand)
		}
	}
}

func (l *trLinker) linkChatFamily(testID, callerRel string, line int, cand string) {
	if guard := cand + "/" + trChatGuard; len(l.byFile[guard]) > 0 {
		l.addTestEdges(testID, callerRel, line, l.byFile[guard])
		for _, f := range trChatFiles {
			l.addTestEdges(testID, callerRel, line, l.byFile[cand+"/"+f])
		}
	}
}

func (l *trLinker) linkComments(rel, raw string, funcOwners map[string][]string) {
	type span struct {
		from, to int
		owners   []string
	}
	var spans []span
	var allTests []string
	for _, n := range l.b.trIdx.Nodes {
		if n.Pos == nil || n.Pos.File != rel {
			continue
		}
		if tid := l.b.trIDs[n.ID]; n.Kind == codeindex.KindTest && tid != "" && l.b.nodes[tid] != nil {
			allTests, spans = append(allTests, tid), append(spans, span{n.Pos.Line, max(n.Pos.EndLine, n.Pos.Line), []string{tid}})
		} else if n.Kind == codeindex.KindFunction {
			spans = append(spans, span{n.Pos.Line, max(n.Pos.EndLine, n.Pos.Line), funcOwners[n.Name]})
		}
	}
	if len(allTests) == 0 {
		return
	}
	emit := func(owners []string, line int, cand string) {
		for _, n := range l.byFile[cand] {
			if n.File != rel {
				for _, owner := range owners {
					l.b.addEdge(owner, n.ID, edgeCites, inferred, rel, line)
				}
			}
		}
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for i := source.HeaderLines(raw); i < len(lines); i++ {
		comment, ok := trLineComment(lines[i])
		if !ok {
			continue
		}
		owners := allTests
		for _, s := range spans {
			if i+1 >= s.from && i+1 <= s.to {
				owners = s.owners
			}
		}
		for _, w := range pathWord.FindAllString(comment, -1) {
			for _, cand := range candTRPaths(rel, strings.TrimRight(w, ".-")) {
				emit(owners, i+1, cand)
			}
		}
		for _, w := range commentFileWords(comment) {
			if c := path.Clean(path.Join(path.Dir(rel), strings.TrimRight(w, ".-"))); !strings.HasPrefix(c, "../") {
				emit(owners, i+1, c)
			}
		}
	}
}

func trLineComment(line string) (string, bool) {
	for i, quoted := 0, false; i < len(line); i++ {
		if line[i] == '\\' && quoted {
			i++
		} else if line[i] == '"' {
			quoted = !quoted
		} else if line[i] == '#' && !quoted {
			return line[i+1:], true
		}
	}
	return "", false
}
