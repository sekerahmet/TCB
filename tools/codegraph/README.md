# codegraph

A local, deterministic knowledge graph of this repository, so an agent (or a person) can ask
"who calls this", "which test guards that rule", "where is this message used" without reading many
files. No network, no model, no new dependency: Go's standard library only. Developer tool; not
shipped to users.

```sh
go run ./tools/codegraph build                  # writes .codegraph/graph.json (git ignores it)
go run ./tools/codegraph query messages.New     # a node and its neighbours
go run ./tools/codegraph path interp.VM H2100   # shortest directed path between two nodes (--undirected, --through-hubs: see below)
go run ./tools/codegraph who-uses H2100         # what uses a message code, name code or function
go run ./tools/codegraph tests-for interp.VM.Run
go run ./tools/codegraph history messages.New       # commits that touched it (computed from git)
go run ./tools/codegraph changed claude/some-branch # graph nodes a branch touches, their tests, overlapping branches
go run ./tools/codegraph deleted                  # message/name codes that existed once and are gone
go run ./tools/codegraph tests-for internal/lang/codes/language.go:227   # the declaration that contains a line
grep -rn "code list %s" internal | go run ./tools/codegraph query -       # each grep hit with its declaration
```

**A location is a name.** `query`, `who-uses`, `tests-for` and `history` take `file:line` and `file:line:col`
(gopls' and `tr dizin`'s form) for a tracked file and answer for the narrowest node whose lines contain it: the
first to the last line of a declaration, a func literal counting as its function, a field inside its type, a doc
comment (directly above, no blank line between) as the declaration it documents, a heading section in Markdown,
a code line in a dictionary file. A line in no declaration (between declarations, imports) answers for the file
and says so on stderr. In a `.tr` file a column resolves like `tr dizin` (codeindex `Reader.At`: the declaration
the name under the cursor refers to). `query -` reads `path:line[:col]:text` lines (`grep -n`, `rg --vimgrep`)
from stdin and writes under each the node that contains it; there is no text search in codegraph (grep and
ripgrep exist). `TestEveryFunctionBoundaryLocatesItsFunction` compares the first, last, doc and func-literal lines
of every function of two packages, and the blank line between declarations, with go/ast (DENETİM 2026-10-26:
a grep hit could not be taken to the graph).

Flags: `build --root DIR --out FILE --warnings`; queries take `--graph FILE`, `--limit N` (lines per
group, default 12) and `--all`, before or after the names. Output is short plain text; edges tagged
`[INFERRED]` are guesses, unmarked ones are `EXTRACTED`. `explain` is another name for `query`.

## Staying fresh

Nobody has to remember `build`. `build` writes a `stamp` into `graph.json`: a digest of the tree's content, not of
`HEAD`. It holds the path of every tracked file (`git ls-files -s`: each is a file node) and, for the sources the
graph parses (`.go`, `.json`, `.md`, `.tr`, `.js` and `go.mod`), the git object id of their content; a changed source
counts with its working-tree content (`git status --porcelain -uall`, then `git hash-object --stdin-paths`, which
applies git's line-ending filter), a deleted one as absent. Of the untracked files only Go sources count (the Go
loader reads every `.go` file of a folder; nothing reads the others). So committing, merging or switching to a branch
with the same content keeps the graph fresh: a hook that asks `tests-for` before every first edit no longer rebuilds
after a commit (TCB 2026-10-25, item 5; `TestStampFollowsContentNotCommits`). `.codegraph/` itself is ignored;
`TestStampCoversEverySourceTheGraphReads` fails when a node or an edge comes from a file type the stamp misses. Every query
(`query`, `explain`, `path`, `who-uses`, `tests-for`) computes the stamp again first. If it differs,
the query rebuilds the graph, rewrites `graph.json` (through a temporary file, so two parallel
queries never read half a file) and prints one line on stderr: `graph rebuilt (N s)`.

- `--no-rebuild` uses the graph as it is.
- `--root DIR` is the repository root; `.codegraph/` always lives there. Without it the root is
  searched upwards from the current directory, so you can ask from any subfolder: the first folder
  whose `go.mod` says `module github.com/sekerahmet/TCB`, else `git rev-parse --show-toplevel`,
  else an error that asks for `--root`. `--graph FILE` still overrides the graph path.
- git missing or failing: the old graph is used and stderr says
  `warning: cannot check whether the graph is stale`. A graph without a stamp (built without git)
  is never checked.
- The rebuild fails (a half-edited file does not type-check): the old graph is used with
  `warning: rebuild failed (...)`; the next query tries again.
- The language cannot be built (a broken or half-converted `internal/lang/codes/tr.json`: the TR
  language packages are compiled into codegraph): the Go graph is built without the `.tr` side, the
  build lists `tr sources skipped: ...` and every query says
  `warning: answers about .tr code are missing (...)` on stderr.

Cost when the graph is fresh (this repository, Windows, 2026-10-26, built binary): `tests-for` takes about 1.5 s,
1.4 s with `--no-rebuild` (reading the 62 MB `graph.json`), so the stamp's git calls cost about 0.1 s; `go run` adds
about 1 s once its build is cached. A stale graph costs one `build` (7-15 s, see "Time"), so the edit hook waits about
that long on a stale graph; a real edit makes the graph stale, there is no incremental rebuild.

Exit codes: `0` answered, `1` failure (no graph, load error), `2` usage, `3` ambiguous name (the
candidates are listed on stdout; repeat with an id or a longer name), `4` not found (similar names
on stderr; for `path`: no path).

A name is resolved in this order, first tier with a match wins: node id (with or without its
`kind:` prefix), exact name (`messages.New`, `H2100`, `interp.VM.Interrupt`), name ending at a `.`
or `/` boundary (`New`, `lang/messages.New`), same case-insensitively, then the value of a string constant (`query "%"`
is `lexer.TokPercent`; operators, tokens, format names; in Git Bash quote `"//"` as `'//'`: MSYS turns `//` into `/`). When none matches and the
query has at least 3 characters, text search: a name that starts with it, a name that contains it,
a `detail` (message text, rule text, heading) that contains it, all case-insensitive. One match is
answered (`query "Cümle parçalardan"` finds the rule); several are listed (exit 3). A bare name (no dot, colon or slash) is resolved among the TR names (`.tr` modules, TR declarations and tests, name codes) by the rule of `codeindex.Resolve` itself, not a copy (`codeindex.NameKeys`, `codeindex.PickByName`), so `query`, `tr dizin tanım` and `tr dizin yeniden_adlandır` see the same candidates (`query bakış_sayısı`: 7). Case: a query without capitals ignores case (Turkish rule); one with capitals first matches exactly, and only when nothing does, ignoring case (`GÖMMEKATMANI` finds `GömmeKatmanı`; ripgrep's `--smart-case` with a fallback); that pass also tries the ASCII lower case (`GIZLI_DURUM`, typed on an English keyboard), and there is no such pass when the query is exactly a word of the language (`Metin` is the type name, not the builtin `metin`). Go and document names keep codegraph's tiers: an exact-case name comes first, since case means something in Go (`identifier` is `interp.identifier`; a lowercase query that resolves this way also lists, on stderr, the names that match only in another case, `ast.Identifier`, so they are not a silent "none"), and the case-insensitive tiers compare with both foldings, Turkish and Go's (`ifexpression` finds `ast.IfExpression`, which Turkish folding alone, `I` → `ı`, missed; audit group 7). Classes: a module name comes before declarations (`katman` is the module, not the parameters named `katman`); a builtin name joins the first class that matches (`metin` is both the module `tools/bench/tr/metin.tr` and the builtin TR_fn_str); a test's description only when nothing else matches (`ortalama` is the builtin). A capitalised query that matches only case-insensitively ranks with the case-insensitive Go tiers. Name codes the index does not know (keywords, unused builtins) are candidates in the builtin class too. The planning exam asks both tools every TR name (each declaration's short name, module names, test descriptions, every name of every builtin in the index) as written, in lower and in upper case, and checks the same set (audit group 6, R1). Text queries try whole words first (Turkish case folding, word boundary; name, then detail and doc comment), then a name prefix, then a substring: `query "ölü"` finds the doc comments that say "ölü", not every "bölüm".

`tests-for` lists the direct tests (`tests` edges), then the indirect ones: callers are followed
backwards over the chain rule below (EXTRACTED `calls`, TR-to-TR `calls`, `uses`, `reads_var`, `writes_var`), up to the first test (Go or TR) on each chain. They
are shown nearest first as `N steps via <caller next to the target>`, always `[INFERRED]`. This is
computed when you ask, not stored in `graph.json`. For a Go file, `tests-for path/file.go` lists the tests of the
declarations in it (the collection `changed` uses) and the rules those tests guard, so "run tests-for <file> before
touching it" answers for Go files too. The tests a test file declares come first as `this file's tests` (benchmarks
are measurements and stay out); `changed` lists a changed test the same way, as `changed tests (they run themselves)`. A function a test file uses as a value (a table entry
`{"ceil", (*Tensor).Ceil}`, a method expression, `f := Cube`) is an INFERRED `calls` edge that the chain does not follow
(the call happens later, elsewhere); when its owner is in a test file, `tests-for` lists the tests under "tests that use
it as a function value": the test itself, or the first tests that read the table or call the helper (chain rule).
Functions of a package that a test builds and runs as a binary get the "covered by subprocess tests" line (`builds`).

## Graph

`graph.json` has `schema_version` (readers refuse other versions), `module`, `nodes`, `edges` and
`warnings`. It is sorted and carries no timestamp: the same tree gives the same bytes.

| Node kind | Id | Source |
| :-- | :-- | :-- |
| `package`, `file`, `func`, `method`, `type`, `const`, `var` | `go:pkg:`, `go:file:`, `go:func:`, `go:type:`, `go:const:`, `go:var:` + path/name | `go list` + `go/types`, test files included; a package in a folder git ignores (`scratch/`) is left out, as the TR side reads only tracked files (`goload.Options.SkipIgnored`); file:line; `const`/`var` are package-level only (`lexer.TokVar`, `messages.H2100`) |
| `message` | `msg:H2100` | `internal/lang/messages/tr.json` (text) and `definitions.json` (params) |
| `name` | `name:TR_kw_if` | `internal/lang/codes/tr.json` (name and family) |
| `rule` | `rule:<readme>:<line>` | a list item or paragraph with `Denetim:`, or a row of a `Kural` / `Test` table |
| `tr_func`, `tr_method`, `tr_struct`, `tr_field`, `tr_const`, `tr_var`, `tr_test`, `tr_param` | `tr:func:<file>:<name>`, `tr:method:<file>:<Struct>.<name>`, ... | `.tr` sources, see "TR sources" |
| `doc`, `heading` | `doc:<path>`, `heading:<path>#<slug>` | every `.md` file; headings use GitHub slugs |

| Edge | Meaning | Label |
| :-- | :-- | :-- |
| `contains` | package → file → func/type/method, doc → heading → rule | EXTRACTED |
| `imports` | package → package (module packages only) | EXTRACTED |
| `calls` | func → func; a call resolved by `go/types` | EXTRACTED; a function used as a value: INFERRED |
| `implements` | type → interface of the module, by `types.Implements` | EXTRACTED |
| `dispatches_to` | interface method → every method that implements it (a method promoted from an embedded field counts), by `types.Implements` | INFERRED: which one runs depends on the value; `query Backend.Softmax` lists them, `callees` prints "dispatches to: ..." under an interface callee, `path` and `tests-for` follow the edge. The chain rule (`reachEdge`) includes it: the tests of `Tensor.Softmax` are tests of `cpuBackend.Softmax`. The type-to-interface pairs come from one function, `moduleImplementations`, shared with `dead-methods` |
| `uses` | owner → package-level `type` or `const` it refers to (variables are `reads_var`/`writes_var`), wherever the name is written: signatures (parameter, result, receiver), field types, embedded types, interface method signatures, type constraints, composite literals, initialisers. The owner is the function or method; for a type declaration the type; for a package-level `const`/`var` the declared name (`var ValInt = TypeName(codes.TypeInt)`: ValInt), for an unnamed spec (`var _ T = ...`) the file | EXTRACTED |
| `refs_spec` | the declaration, heading or file that mentions a language-definition item → `spec:N.M` (see above) | EXTRACTED |
| `reads_var` | owner → package-level `var` it reads | EXTRACTED; access sites with mode `read` (also the read half of `x += 1`, `x++`: mode `read` next to the `read+write` write site) |
| `writes_var` | owner → package-level `var` it changes: assignment, `+=`, `++`/`--`, element or field write (`m[k] = v`, `v.f = x`), `delete`/`clear`, `&v`, `sync/atomic` call, qualified write from another package (`a.V = 1`) | EXTRACTED; site modes `write`, `read+write`, `elem-write`, `addr`, `atomic` (same vocabulary as `writes_field`) |
| `inits_var` | a write inside a package's `func init` (owner = that init), and the variable's own declaration initializer (`var active = load()`: the variable to itself, mode `initializer`) | EXTRACTED; "who sets it up" is a separate group in `query`, apart from the steady-state writers |
| `embeds` | `//go:embed` variable → the data file(s) its patterns name (relative to the package folder) | INFERRED |
| `reads_file` | function (non-test) → repository file named by a string constant it passes to `reporoot.Path`/`filepath.Join`/`Glob` | INFERRED |
| `walks` | test → data file whose whole list it walks: the test (or a test helper) has a `range` over a function or variable that returns a collection and depends on the file inside the package that embeds it (`for _, k := range codes.Codes()`) | INFERRED |
| `denotes` | Go constant `messages.H2100` / `codes.CmdRun` → the message / name code node `H2100` / `TR_cmd_run` it stands for | EXTRACTED |
| `tests` | test → its package; test → function it calls (directly or through helpers in test files); test → function its name says (`TestParse`, `TestVM_Run`) | EXTRACTED / EXTRACTED / INFERRED Also test → repository file (a `.tr` file, a doc) when the test, or a helper it calls through the chain rule below, writes the path as a string constant: `reporoot.Path("tools/gpu/gpt_training.tr")`, `reporoot.Path("lib", "*.tr")` or a `filepath.Glob` pattern; a bare wildcard literal is test data and does not count, nor does a bare file name (no `/`); INFERRED, so `tests-for lib/egitim.tr` lists the Go tests that guard it. A TR program in a test string that imports the library (`aktar "tr/hazir"`, coded `TR_kw_import "tr/hazir"` too) links the test to `lib/hazir.tr`, whether the string goes to the interpreter or to a file the test hands to `tr` (`TestReadyModelMatchesTransformers`); `codeindex.Snippet` resolves the import by the file rule against `lib/`. A test whose edge from the same site also goes to other files reads this one through a pattern or an embedded file set (`lib/embedded.go`'s `go:embed *.tr`); `tests-for` lists it last, under "tests that read it among other files", so a library file starts with its behaviour tests (parti K2). `tests-for` writes under each Go test the first line of the rule it guards (the test's doc comment, else the first `//` block after the imports), read from the file at query time. `tr dizin` stays TR-only (it parses `.tr`, not Go); the Go-to-`.tr` link lives here. Also test → each module function a string `case` calls when the test (or a test-file helper it calls) carries that case's text and reaches the switch's function: a command run by name (`run([]string{"who-uses", ...})`) tests the function of its case directly, INFERRED (`case_tests.go`). |
| `uses_message` | func → `msg:Hnnnn`, from the `messages.Hnnnn` constant | EXTRACTED |
| `uses_name` | func → `name:Xnnn`, from the `codes.Xnnn` constant; message → `name:K4nn` when its template (any language) writes that environment variable's name (`TR_GPU_SYNC=1`) | EXTRACTED |
| `reads_name` | func → `name:Xnnn` when the constant is an argument of a function of the `codes` package (`codes.Spellings(codes.FnPrint)`, `codes.Display().Name(k)`): it reads the code's name, it does not implement the builtin | EXTRACTED. `who-uses` lists it; the tests of a name are not the tests of a function that only reads it (parti K2, C: `tr.startsWithPrint` reads `yazdır`'s spellings for the REPL, and six TR tests that print headed `tests-for cmd/tr/main.go`). `TestANameCodeGivenToTheCodesPackageIsReadNotImplemented` finds the reads with go/types and checks each |
| `uses_env` | func → `name:K4nn`, from `os.Getenv`/`LookupEnv`/`Setenv`/`Unsetenv` (or `testing.T.Setenv`) given a string constant (or a named one) equal to the variable's name in any writing language, and from an environment string `NAME=value` that goes into an environment slice, as an element of a slice or composite literal or an argument appended (a child's `cmd.Env`, `[]string{"TR_DIL=en"}`, `Name + "=x"`, `messages.LanguageEnvVar + "=en"`, `"TR_DIL=" + lang`; a log message `"TR_DIL=en ile koşulmalı"` is not one); the compute core cannot see name codes, so it reads the variable by its source-language name. A test that sets the variable (Setenv or an environment string; itself, a helper in a test file, or a function only test files call, such as `testkit.RunWithLanguageFile`) gets a `tests` edge to the code, so `tests-for TR_env_LANG` lists the tests that run with TR_DIL set; production code that sets it for its own child does not. `TestEveryEnvironmentVariableIsSetByATest` requires one such test for every K4nn code, with no exception list (value language round 3) | EXTRACTED |
| `guarded_by` | rule → test func or test file named after `Denetim:` | EXTRACTED when the test sits under a path the rule names or under the README's folder, else INFERRED |
| `links_to` | heading → heading, doc, Go file, package or any file; Markdown links | EXTRACTED; link whose `#anchor` is missing: INFERRED (to the file) |
| `implemented_by` | TR declaration → Go function that runs a language construct it uses: an operator (`@`, `+`, `==` ...) or a construct (call, index, member access, string interpolation, loop ...) | INFERRED. Read from the interpreter, not a hand list (`dispatch.go`): the compiler's `case *ast.CallExpression:` branches and the operator switches of the interpreter and its hooks (`switch op { case "@": ... }`, or the text-to-constant switch and the constant switches); a branch without calls is the switch's own function; a branch that calls a compile helper (a method of the interpreter's compiler, `c.interpolation(...)`) also gets what the helper calls, closures included, along a chain of helpers up to 8 steps, so moving a branch's body into a helper keeps the link (value language steps 1-2: interpolation → `valueText`, `Value.Inspect`); a general compile method with its own AST type switch (`compiler.expression`) is not followed through a helper, it dispatches every construct. codeindex records each declaration's constructs with the left operand's inferred type; a known type keeps only its implementations (Tensör → hooks, numbers and text → interpreter). The chain of `tests-for` follows it only from `tools/bench` programs, so `tests-for interp.VM.callPrepared` lists `tools/bench/tr/fib.tr` under measurements, and TR tests do not all become tests of the interpreter (trial 9 H1, trial 10). The edge keeps its construct (`via`: `MemberExpression`, `operator @`; the first seen). `who-uses` and `query` list a function's Go callers first and the bridge last, one line per construct (`via CallExpression: 1098 TR declarations`); `--all` lists the declarations (DENETİM, parti K2, B: 1175 TR declarations buried the one Go caller of `interp.memberKeys`) |
| `related` | name code → name code its definition lists as related (`"related"` in `internal/lang/codes/definitions.json`: yazdır → girdi, birleştir → böl) | EXTRACTED. `query TR_fn_print` shows the related names, the first place to look when designing a new name |
| `cites` | test → repository file or doc whose path its comments or its strings name (`// Kod düzeni standardı (docs/development.md)`, `t.Errorf("...; docs/development.md")`), when it does not read it | INFERRED. The owner is the declaration holding the comment or string (a test, or the tests that reach a helper, a table or a type through the chain rule); a comment outside every declaration (the rule block after the imports) belongs to every test of its file. A part with `/` is resolved from the root and from the file's folder; a bare file name (`README.md`, `layout_test.go`) only in a comment and only from the file's folder; a part that names no tracked file is dropped. `tests-for` lists these tests apart, under "tests that cite it", after the tests that read it (a test that also reads the file is listed once, as a reader); `changed` does not run them (TCB 2026-10-25, item 2: `tests-for docs/development.md` said "no tests found") |
| `loads` | JavaScript file → repository file it loads by a relative path: `require('./x')`, `require.resolve('../x')`, `import ... from './x'`, `import('./x')`, and `path.join(__dirname, '..', 'media', 'x.js')` (or `path.resolve`) when every part is a string; Node's order `x`, `x.js`, `x/index.js` | EXTRACTED. A package name (`vscode`, `fs`) is not a repository file; a path built from a variable is not seen. `tests-for` follows it backwards to the files a Go test runs, among the indirect tests: `e2e.TestExtensionJSTests  2 steps via editors/vscode/test/coded_view_test.js` for `editors/vscode/coded_view.js` (the test runs `test/*_test.js`, the test file loads the module); `changed` uses the same chain (TCB 2026-10-25, item 4). A `.tr` import (`imports` between `.tr` files) is followed the same way: the tests of a `.tr` file include those of the files that import it, transitively: the Go test that runs `tools/gpu/gpt_training.tr` for `lib/katman.tr` (2 steps), the TR tests of `lib/katman_test.tr` (1 step; at equal distance a file's own test file first); `changed --go-tests` looks the `.tr` file up too, so a `lib/*.tr` change selects them (parti K2) |
| `scans` | test → `scan` node: a repository-wide check that lists files and filters them by name, `reporoot.WalkDir(...)` with a callback, or `git ls-files` (`scans.go`). The node holds the scope: base folders from the root, included and excluded name endings (`*.go under the repository, not *_test.go`) | INFERRED. The base is resolved from the root only: `reporoot.Root()`, `reporoot.Path(...)`, `Join(root, part...)` where a part is a string, a loop variable over a string list or a package-level list (`layoutRoots`), a local assigned once, or a parameter its direct callers pass (`scanHookTypes(t, root)`); a temporary copy or a folder read from data is not a scan. The filter is read from skip guards (an `if` whose only statement is `return`/`continue`: `!strings.HasSuffix(path, ".go") \|\| strings.HasSuffix(path, "_test.go")` includes `.go` and excludes `_test.go`), from other `if`s and tagless `switch` cases (positive), `switch filepath.Ext(...)`, `filepath.Ext(x) != ".tr"`, `d.Name() != "README.md"`; a suffix given by a caller (`repoFiles(t, ".tr")`) and the filter of a caller of a helper that returns `git ls-files` output make one scan per caller. A walk that skips files (`!d.IsDir()`: packages through `build.ImportDir`) is not a file scan; folder skips (`testdata`, `negative`) are not read, so a scope is a superset. `tests-for` lists them apart for every file the scope covers, one line per scan with at most 8 test names (`--all` all): "repository-wide checks that scan it (16 tests in 9 scans; ...)" with `internal_test.TestNoTurkishLiterals` for `tools/edithook/main.go`; their rules join "rules guarded by these tests"; `changed` does not add them (the full check runs them). `TestEveryRepositoryScanIsRecognized` reads every test file without codegraph and requires a scan node for each listing from the root, and that each scope covers a tracked file (TCB 2026-10-25: the full check failed on that file with that test, its answer had not shown it) |
| `compares_value` | declaration → exported string constant whose value it names by text in an identity use: a `switch` case (`case "%":` in `interp.opCode`, `case "+", "-", "*", "/", "%", "**":` in the checker), `==`/`!=` (`op == "%"`), or a key of a string-keyed map literal (`map[string]T{"%": ...}`) | INFERRED (`value_compares.go`). The value is the same; which constant is meant is not written, so every exported, non-test module constant of that value is a target. Only when the declaration's package depends (transitively) on the constant's package: `reporoot`'s `name == "."` is not `lexer.TokDot`. The empty string does not count. `who-uses lexer.TokPercent` lists them under `compares_value` beside the `uses` of the name. `TestEveryOperatorTheInterpreterOrCheckerNamesByTextIsListed` builds the expected set from the syntax trees of the interpreter and the checker without codegraph (gaps.md critical 1, DENETİM 2026-10-26: the interpreter and the checker named `%` by text, the list of the constant's users looked complete) |
| `formats`, `encodes`, `stdlib_calls` | declaration → method or field the standard library uses through an interface or reflection on a module value the declaration hands it: `formats` when a formatter (`fmt`, `log`, `testing`'s Log/Error/Fatal/Skip families, the module's `messages` package) gets it as `any` (its `String`, `Error`, `Format`, `GoString`: `fmt.Sprintf("... %s", k)` in `codes.Language.Name` for `codes.Code.String`); `encodes` when `encoding/json` (`Marshal`, `MarshalIndent`, `Unmarshal`, `Encode`, `Decode`) gets it (its exported fields without `json:"-"`, those of nested module structs through pointers, slices, arrays and maps, and `MarshalJSON`, `UnmarshalJSON`, `MarshalText`, `UnmarshalText`: `profiler.statusWriter.put` for `profiler.Snapshot.Start`); `stdlib_calls` when it is passed for a non-empty standard library interface parameter (`sort.Sort(byLen(x))` for `Len`, `Less`, `Swap`; `io.Writer`) | INFERRED (`stdlib_dispatch.go`): the call happens at run time. The kind is the use's role (the display role of the "role of a use" request is `formats`). The argument's type is resolved without loading expression types (`Info.Types` would make the build 17% slower, measured 5.4 s → 6.3 s): identifier, selector, composite literal, `&x`, `*x`, `x[i]`, `x.(T)`, `T(x)` and a call's first result; a value converted to a basic type (`uint16(k)`) carries no method. `who-uses` of a field lists these after its accesses ("no direct read or write outside its declaration" when they are its only uses). `dead-methods` takes the same method names into its external-contract list. A value returned as `error` and formatted elsewhere is not followed. `TestEveryStandardLibraryDispatchHasAnEdge` loads the repository with full go/types and requires an edge for every formatter and JSON argument (1197 on 2026-10-26) (gaps.md critical 3, DENETİM 2026-10-26: `who-uses codes.Code.String` missed `language.go:227`, `who-uses profiler.Snapshot.Start` showed one literal and no reader) |
| `builds` | function in a test file → module package it compiles with `exec.Command("go", "build", ..., pkg)` (a module path, or `.` / `./x` from the test file's folder) | EXTRACTED. The tests that run the binary cannot be traced per function, so `tests-for` counts them on one line for every function of that package, "covered by subprocess tests (N; ...): cmd/tr (2), cmd/tr/e2e (86)": all tests of the package when the builder is `TestMain`, else the tests that reach the builder (chain rule); `changed` prints the same line. A package the built one imports (transitively) gets, when nothing else tests it, "weak evidence: linked into the binary of cmd/tr (88 tests run it); a subprocess test may reach this code, which one is not traced" instead of "no tests found" |

Every edge has `confidence`, and `file`/`line` of the first place it was seen and `count`.
`warnings` lists what could not be resolved: a `Denetim:` test that does not exist, a broken
Markdown link. They are real findings; `build` prints the first five.

Limits: types come from one build configuration, this machine's GOOS/GOARCH (like `tools/impact`). The files that
configuration does not build (`cuda_linux.go`, `file_error_unix.go`, `race_on.go` on Windows; `go list`'s
`IgnoredGoFiles`) are read without types (`other_os.go`): each becomes a file node "not built for windows/amd64", a
function declared only there becomes a node, every declaration in it is `contains`-linked from that file (so `query
gpu.runExitHandlers` lists `cuda_linux.go`, `cuda_other.go` and `cuda_windows.go`), and its message and name code uses
(`messages.H2154`, resolved from the import, not from types) are INFERRED `uses_message`/`uses_name` edges, so `who-uses
H2154` finds `file_error_unix.go`. Calls and type uses in those files are not seen: `build` warns which configuration
was type-checked and lists the files. Plain calls in those files (`F(...)` of the same package, `pkg.F(...)` of a module
package, unless the function declares that name itself) are `calls` edges, so a test built only on Linux
(`mmap_unix_test.go: TestMmapTruncatedDoesNotCrash` → `OpenShard`) joins the chain. When the asked name occurs in those
files (as a word, outside comments), `query`, `who-uses` and `tests-for` add a group "in other build configurations" with
each declaration it occurs in (tests marked); a name they never mention gets nothing (DENETİM verification Y4). Type-checking every configuration and merging (as `staticcheck -matrix` does) was not chosen: it multiplies
every build, and the Windows and Linux versions of a function share one node ID. `implements` and
`dispatches_to` look at non-test types only. A call through an interface points at the interface
method and, from there, at every implementation (`dispatches_to`, below).

## Struct fields and closures

Every field of a struct type of the module is a `field` node (`tensor.Tensor.grad`, file:line, its
type in `detail`). Each selector `x.f` is resolved with `go/types` to the field it really names
(promoted fields of embedded structs and fields of generic instances included; same-named fields
of other types never mix), and keyed composite literals `T{f: v}` too. The access is kept on a
`reads_field` / `writes_field` edge from the enclosing function, with every site
(`file`, `line`, `mode`, and the func literal node it sits in). Modes: `read`, `write`
(`x.f = v`, `x.f, y.g = ...`), `read+write` (`x.f++`, `x.f += v`), `elem-write` (`x.f[i] = v`,
`*x.f = v`: the contents change), `addr` (`&x.f`), `literal` (`T{f: v}`). Unkeyed literals
(`T{1, 2}`) and fields of unnamed struct types are not recorded.

- `who-uses Tensor.grad` or `fields Tensor.grad [--mode read|write]`: the accesses by file:line,
  mode and accessor; inside a func literal the accessor is `binaryKind > func literal@L102`.
- `fields <Type>`: a table of the type's fields with read and write counts.
- Every `func literal` is a `closure` node (`outer > func literal@Lnn`, `contains` from the
  function or the enclosing literal) carrying `captured`: the local variables and parameters it
  takes from the enclosing scopes with their types (`out *tensor.Tensor`), `(assigned)` when the
  literal assigns them. `captures <func> [--type T]` lists them for every literal in a function
  (nested ones too), `--type Tensor` keeps variables whose type contains the text.

## TR sources

`.tr` files under `lib/`, `tools/` and `examples/` are read with TR's own parser
(through `internal/devtools/codeindex`, the TR code index package, which uses `internal/lang/source` and `parser`; coded files with a header are read too, lines are the file's real lines, the header line counts). The graph does not depend on the active writing language (`TR_DIL`): the index reads every file
as the interpreter does, with identities (a coded file's tokens are identities; a file without the header is Turkish
and is encoded first), never through `source.Prepare`, whose positions follow the reader's view. A name that is a code
is its identity in the graph, as on disk (`katman.TR_type_Linear.TR_mem_forward`); the declaration carries its names in
every writing language and its qualified names as aliases (`Doğrusal`, `Linear`, `katman.Linear.forward`), so a query in
any language finds it, as in `tr dizin`. A bare identity (`TR_type_GPT`) names the code's own node (`name:`), not a
declaration. The test `TestTRGraphIsIndependentOfTheActiveLanguage` builds the fixture graph under `TR_DIL=tr` and
`TR_DIL=en` in sub-processes and compares the `.tr` facts. Syntax errors in the intentional error examples
(`examples/tr/hatalar/`) are not warnings. Node kinds: `tr_func`, `tr_struct`, `tr_method`, `tr_field`, `tr_const`, `tr_var` (top level)
and `tr_test` (a `test` block), all with file:line:end; `tr_param` is a function or method parameter (`tr:param:<file>:<Owner>.<name>`, `contains` edge from its owner, detail = `name = default text`; a bare-name `query`/`who-uses` lists same-named parameters together with other matches, so one name used in several meanings shows up at once; named arguments `f(name: v)` are `uses` edges from the caller to the parameter, or to the field for a struct without `kurucu`); the file is a `file` node `tr:file:<path>`
with the module name (`katman`) and the import path (`tr/katman`) as aliases.

**Division of labour with `tr dizin`.** The `.tr` extraction above is only a thin adapter
(`codeindex_adapter.go`): `internal/devtools/codeindex` owns parsing, name resolution and the node
model for `.tr` code, and codegraph maps its index into graph nodes and edges (ids and edge kinds
stay as described, so queries and the extractor digest are unchanged). Questions about TR code
itself (every use of a name with its role and position, which tests reach a declaration, what an
edit affects, the declaration under a cursor position) are answered by the TR command, not by
codegraph: `tr dizin tanım|kullananlar|sınayanlar|etki <ad>` (`--json` for agents, see
[`internal/devtools/codeindex/README.md`](../../internal/devtools/codeindex/README.md)). Codegraph
stays the tool for Go code, messages and codes, docs and history.

- `imports`: file → file for `aktar` (`"tr/x"` is `lib/x.tr`, other paths are relative to the
  importing file, `.tr` optional).
- `calls`: a call (or use as a value, INFERRED) of a top-level function or struct in the file or in
  a module imported without alias; `alias.Name` through `olarak`. `x.method()` on a receiver of
  unknown type is an INFERRED edge to the TR methods of that name that are visible (at most 4).
- `uses`: a top-level constant or variable.
- `uses_name`: a built-in global name (`yazdır`), a built-in member name (`x.şekil`) and a named
  argument, resolved with the language dictionary to the name code node (a local variable, a
  parameter, a loop or catch variable of the same name shadows the built-in).
- `tests`: a call made inside a `test` block also is a `tests` edge, so `tests-for NedenselDikkat`
  lists the TR tests.
- A file that does not parse is a warning (`tr syntax error, definitions skipped: ...`) and keeps
  its file node. Scoping is approximate (the names bound anywhere in a declaration shadow for all
  of it), calls through values are not followed.

`query Muon`, `query katman`, `who-uses dikkat` (the code TR_fn_attention with its TR call sites) work on TR.

## Call and usage questions

- `callees <func> [--in PKG]`: the functions a function or method calls, grouped by package
  (`--in` keeps one package or folder prefix).
- `calls-into <pkg> [--min N] [--in SRC] [--tests]`: the functions that call at least N (default 2)
  distinct functions of a package, with the callees: "which Go function calls which tensor
  functions". Only EXTRACTED calls count; callers inside the package and in test files are left out
  (`--tests` adds the tests); `--in` narrows the callers to a package or a folder prefix
  (`--in internal/stdlib`).
- `who-uses <name> --by-package`: the users of a function, constant, message or name code
  summarised per package (functions, use sites, users in tests).
- Not found: stderr lists `did you mean:` names (containing the query or within a small edit
  distance of it, also checked against the part after the last `.` or `/`).
- A built-in's name in either language resolves to its name code: `query zaman_ms` (or `time_ms`)
  prints the code, its Go constant, where the code is used or registered with the implementation
  named on the same line (`{codes.FnTimeMs, timeMsBuiltin}`), and the heading in `docs/codes.md`.
  Name-code nodes carry both language names in `aliases`.

## History commands (from git, never stored)

These three ask git when you run them; `graph.json` stays small and `build` fast. They need a git
repository (a failing git is an error, exit 1).

- `history <name>`: commits (short hash, date, subject; no diffs), newest first, `--limit N`
  (`--all` for every one). A function, method or type uses `git log -L` (by name when the
  declaration is a plain `func`/`type` line, else by its current line range); a message or name
  code uses a pickaxe over the dictionary files and marks the commits that **added** and
  **removed** it, and those that **renamed** it from an older key (the history goes on with the
  older key's commits; see the renames below); a file or doc uses `git log --follow`.
- `changed <branch|range>`: maps the hunks of `git diff -U0 <base>...<branch>` (`--base`, default
  `main`; or a literal `a..b` / `a...b`) onto Go and TR declaration and doc-section line ranges, and onto
  the codes named in changed dictionary lines. Then it lists the tests that exercise the changed
  code (the same tests `tests-for` lists for each changed node; direct first, then indirect with their distance,
  first `--limit`), and the other local `claude/` branches not yet merged into the base that touch the same nodes or
  files, with the result of a
  `git merge-tree --write-tree` trial merge (`clean` or the conflicting files). The graph is of
  the checked-out tree: when the ref is the checked-out HEAD the diff runs from the merge base (from `a` for `a..b`) to
  the working tree, committed, staged and unstaged edits and tracked new files included, and its new-side lines are
  mapped (new functions included; the lines are the graph's own, so an uncommitted edit above a committed one does not
  shift it); otherwise the old-side lines, which is exact only while the tree equals the merge base. `dups --changed`
  takes its lines from the same function (`changeHunks`). `--go-tests` prints only the Go tests to run, one `folder TestName` per
  line: the changed tests, the tests of the changed code, the subprocess tests, and the tests that read any changed file (a data
  file the graph does not parse, such as `planning_questions.txt`, maps to no node, its readers are still listed); the pre-check
  (`tools/full_check.sh --kip=on`) runs them (`TestChangedGoTestsSelectsTheTestsOfAChangedDataFile`); an untracked file is in neither (DENETİM 2026-10-26:
  the check run before a commit missed its edits; `TestChangedSeesTheWorkingTree`). Added code with no old-side counterpart is counted under "mapped to no node" (by file) and,
  for Go files, named from the branch's own copy of the file ("declarations on the branch that the
  graph does not have": new functions, new tests, new test files). A graph written by an older
  codegraph (`extractor` below the tool's) is rebuilt by the next query even when its stamp matches.
- `deleted [--retired-json] [--family messages|names]`: message and name codes that were in the
  dictionary files in the past and are gone today, with the commit that removed them and the one
  that added them, and codes that were defined again after a deletion (a bug: numbers are never
  reused). `--retired-json` prints the `retired.json` content (`internal/lang/messages/retired.json`
  was produced by it). Since 2026-10-21 (the language is a draft) removed name codes are deleted from the
  dictionary, so the `names` list shows them too. History
  before a file was renamed is not visible to it.
- Renames: every row of a mapping document (`identityMaps`: `docs/code_identities.md`) whose first two cells are
  code keys (``| <old code> | `TR_kw_if` |``) is a rename event. Where one commit removes the old key and adds the
  new one in the same file the two are one `renamed <old code> → TR_kw_if` event: the old key is not `deleted`,
  the new key's `history` goes on with the old key's commits, and a `query` of the old code answers with
  `TR_kw_if` (the old key is an alias of the name code node). Chains are followed (`TR_a` renamed again to
  `TR_b`: a new row ``| TR_a | `TR_b` |``), so a later rename of an identity or the library codes of item 15
  step 3 need only their rows (`TestEveryRenameInTheMappingKeepsItsHistory` runs over every row of the document
  and the repository's own history; `TestARenamedCodeIsNotDeletedAndKeepsItsHistory`).

## Duplicate code (`dups`, `similar`)

Say "this looks like a copy of ..." before code is written or merged. Both commands read the Go
sources directly (no graph needed, no network, no new dependency) and are deterministic.

```sh
go run ./tools/codegraph dups                       # candidates in the whole repository
go run ./tools/codegraph dups --changed claude/x    # only what the branch added or changed (the branch must be checked out)
go run ./tools/codegraph dups --min 60 --in internal/stdlib --json
go run ./tools/codegraph similar tokenize.parallel  # the functions most like this one, with a score
go run ./tools/codegraph dups --list-exclusions     # the deliberate exceptions and why
```

- **`dups [--min N] [--in DIR] [--changed branch|range] [--base B] [--limit N|--all] [--json] [--no-exclude]`.**
  A candidate is a set of two or more places whose syntax trees have the same shape for at least
  `--min` nodes (default 100, dupl's default). Each location is `file:from-to` plus the function it
  is in; the diff summary says whether the copies are exact (names and values too) or only the same
  shape and lists the most frequent `a -> b` renamings. `--in` keeps candidates with a location under
  a directory. `--changed` keeps candidates with a location on the lines the branch added or
  changed (the `changed` command's hunks) and marks those locations `[changed]`; the other
  locations can be anywhere, so "this new function copies an existing one" is the case it exists
  for. Every form compares from the **merge base**, like `git diff A...B` (a tree diff would count
  what `main` did meanwhile as the branch's work): `--changed main` and `--changed main..HEAD`
  (also `main...HEAD`, `main..`) mean "what is new on the checked-out branch since it left main";
  the checked-out branch's own name means `--base...branch` (`--base` defaults to `main`); a range
  `A..B` needs B to be the checked-out commit (the line numbers are the working tree's), otherwise
  it is refused. The changed lines reach the working tree, uncommitted edits included, as in `changed`: a copy
  written before the commit is a candidate. Pinned by `TestDupsChangedUsesTheMergeBaseNotTheTreeDiff` and
  `TestChangedSeesTheWorkingTree`.
  Exit codes as elsewhere; JSON schema `codegraph-dups/1`.
- **`similar <func> [--limit N] [--json]`.** Dice score (0..1) between the target and every other
  function over the sets of 4-grams of their node sequences; functions of fewer than 12 nodes and
  scores below 0.30 are not listed. The name is `Func`, `Type.Method`, or either with a package
  directory (`tokenize.parallel`, `internal/compute/tokenize.parallel`); an ambiguous name lists its
  candidates (exit 3). It finds the *near-miss* copies `dups` cannot (an extra statement, a
  different literal shape), e.g. `ShortNumber` and `FormatBytes` score 0.76 and `registerIsFinite`
  and `registerRounding` 0.85. Schema `codegraph-similar/1`.

**Candidate ids.** Every candidate has a stable `id` (text and JSON): its locations' `dir/file:Func` names
sorted and joined with `~` (`tensor/helpers_test.go:bf16Deviation~ml/behavior_grad_composite_test.go:compositeDeviation`;
`@line` when no function starts in the span; more than 3 locations end in `+N`). The `#N` rank moves
whenever another candidate appears or goes away and line numbers move with every edit; function names
stay while the copy exists, so a document or a hand-over note should cite the id. There is no accept-by-id
list on purpose: it would silence a specific pair even after the copy grows, and the exclusion rules
(`dups_exclude.go`, a glob and a reason) already cover the deliberate exceptions.

**Method.** Each file's syntax tree is flattened in pre-order into one sequence of numbers, one per
node: (node type, operator, child count). Names and literal values are *not* in it (they only
feed the diff summary), the child count makes the sequence decode to exactly one tree, and a unique
separator between files stops every match at a file boundary. Import declarations are skipped.
The tool then looks for runs of `--min` equal numbers with a rolling hash (Karp-Rabin) over all
windows, confirms hash hits by comparing the windows, keeps only matches that cannot be extended
to the left (so a long clone is reported once, not at every offset), extends them to the right as
far as they go, merges matches that start at the same place into sets of three or more
locations, and drops a set whose every location lies inside a bigger set's location. Groups of
more than 60 identical windows are boilerplate and are counted, not paired.

| | dupl ([mibk/dupl](https://github.com/mibk/dupl)) | PMD CPD ([docs](https://pmd.github.io/pmd/pmd_userdocs_cpd.html)) | here |
| :-- | :-- | :-- | :-- |
| Unit | serialised Go AST, node types only ("ignores the values of AST nodes"), threshold `-t` in tokens, default 100 | token stream, `--minimum-tokens` (required), Karp-Rabin | AST node sequence with the child count, `--min` nodes, default 100 |
| Search | suffix tree | Karp-Rabin string matching | rolling hash over windows + verification: simpler, no suffix tree, same result for the clone sizes asked for; memory is O(nodes) |
| Names and literals | ignored | identical by default; `--ignore-identifiers` / `--ignore-literals` loosen it | ignored always (same shape = clone); kept for the diff summary |
| Operators | not stated on the page | tokens | part of the node (`a + b` and `a - b` differ) |
| Suppression | none stated | `CPD-OFF` / `CPD-ON` comments | the exclusion list below (file or pair patterns with a reason) |
| Near-miss copies | not found | not found | `similar` (n-gram score) |
| Changed-only mode | none | none | `--changed` |

dupl's and CPD's pages do not say how they treat operators or how a match inside a function is
trimmed; those cells are "not stated", not claims.

**Deliberate exceptions** (`dups --list-exclusions` prints them with the reason; a silenced
candidate is counted by name in the output, `--no-exclude` shows it): the GPU simulator
(`internal/compute/gpu/sim*.go`) against the tensor conversions and arithmetic (an independent
re-implementation, sharing it would make the check circular); float64 reference implementations in
tests against production code; `internal/lang/ast` node boilerplate; generated files (the
`// Code generated ... DO NOT EDIT.` header, which also covers the Unicode tables) are not read;
tables are one leaf (every composite literal in a `*_test.go` file, and in production code those
with 8 or more elements: name tables, operator sets); **sibling runs** are one leaf too (PMD CPD has the same idea behind `--ignore-sequences`, [CPD docs](https://pmd.github.io/pmd/pmd_userdocs_cpd.html), 7.x)
(3 or more consecutive, same-shape units under one parent, each under 40 syntax-tree nodes:
the items of a `const (...)` block, back-to-back registration calls `wrap(v, codes.FnEmbedding, ...)`,
a kernel's `x := k.pointer("x")` parameter declarations, consecutive one-statement test
functions. The order of such a list is information, each unit says something else, and a real copy
of a list is a copy of a table, not of logic; 40 nodes is the size of `func TestX(t) { expectOut(t,
`...`, `...`) }`, so a unit with a branch or a loop is never a sibling); `.tr` files are not read by this command. To add an exception write one entry in
`dups_exclude.go` with a reason.

**Limits.** Exact-shape clones only (type 1 and 2 in the clone literature): a copy with an extra
statement is found by `similar`, not by `dups`. A match may begin in the middle of a function (the
span is the lines its nodes cover). Switch arms and sequences of one-line wrappers are reported
although they are rarely worth merging: judge each candidate. On this repository `dups` takes about
1 s and reads about 700 000 nodes of 800 files; at `--min 100` it lists about 30 candidates.

**For `.tr` code** the same idea belongs in `tr dizin` (the TR code index already has the syntax
trees and a language-independent reader): a `tr dizin kopya` subcommand over the `codeindex`
parser output with the same node sequence; see the hand-over note for the proposal.

## Audits (`dead-methods`, `package-state`, `time-ceilings`, `spec-refs`)

Four audits that were once done by hand (roadmap Evre 4 item 7). They read the sources directly
(type-checked with `goload`, or plain text for `spec-refs`), need no graph, and print **lists to
review, not decisions**. All take `--in DIR` (only results under a path), `--json`, `--limit N`.

```sh
go run ./tools/codegraph dead-methods     # methods nobody calls
go run ./tools/codegraph package-state    # process-wide mutable package variables
go run ./tools/codegraph time-ceilings    # tests with a wall-clock ceiling
go run ./tools/codegraph spec-refs        # language-definition references: missing items, suffixes
go run ./tools/codegraph spec-refs --list # every reference with the heading it points to
```

- **`dead-methods`** lists methods with no caller. A method called through an interface counts as
  used (the interface method's call sites are followed); a method that only *satisfies* an
  interface whose method nobody calls is `assertion-only`. Kinds: `unreferenced`, `assertion-only`,
  `shadowed` (an earlier interface check always catches the value first, as `ml.TensorValue.Operator`
  was before 00d36f84), `tests-only`. Sealed-interface markers (`statementNode`) and methods of
  external contracts (`String`, `Error`, `Unwrap`, `Read` ...) are skipped.
- **`package-state`** lists package variables that are written after their declaration, per
  package: `mutable`, `test-hook` (written only by tests), `address-taken`, and `synchronized`
  (`sync.Mutex`/`Once`, `sync/atomic` types) separately. Tables that are only read after `init`
  are counted but not listed.
- **`time-ceilings`** lists the places in `_test.go` files where `time.Since`/`time.Until`/
  `Time.Sub` (or a variable holding one of them) is compared with a constant. Rule
  (internal/README.md): duration tests look at growth (`testkit.ScalesLinearly`), not at a ceiling;
  a ceiling is allowed only for a wait that must end early, at half the distinguishing time.
- **`spec-refs`** scans docs/, `tests/*.tr`, Go comments, `lib/` and `examples/` (tracked files)
  for references to language-definition items: `**N.M**`, `**N.M–K.L**`, `dil tanımı N.M`,
  `§N.M`, `[N.M](…spec/…)` (errors) and bare numbers in parentheses inside `docs/spec` (notes).
  (a) a reference to an item that does not exist is an error (exit 1); (b) `--list` prints each
  reference with the heading of its target; (c) the Turkish suffix after the number is checked for
  vowel harmony and consonant assimilation (`15.9'daki`, `15.24'teki`, `15.3'ün`, `15.2'ye`).
  The scanner and the suffix rules live in `tools/internal/specref`, shared with the renumber tool.

### Renumbering items (`tools/renumber`, separate on purpose)

Renumbering writes files, and codegraph is read-only (a write would also make its graph stamp
stale), so it is its own tool:

```sh
go run ./tools/renumber --map moves.txt           # the plan: every mention, file:line
go run ./tools/renumber --map moves.txt --apply   # write it
go run ./tools/renumber --suffixes --apply        # only repair suffixes that break vowel harmony
```

`moves.txt` has one `old new` pair per line (`#` comments, `->` accepted). Edits are computed from
the original positions, so swaps do not chain; definitions, references and in-spec mentions
change together; suffixes are re-fitted to the new number; a range whose targets are not
consecutive is split (`15.31–15.38` → `15.11–15.15, 15.20–15.22`); text that is not a recognised
reference (`15.0` as a version) is never touched. Collisions and unknown old numbers are refused
before anything is written; `--apply` refuses a plan that no longer matches the files.

## Language-definition items as nodes (`spec`, `refs_spec`)

Every numbered item of `docs/spec` (`**17.7** ...`) is a node `spec:17.7` (name `17.7`, detail = its title), and every mention
of it is an edge from the place that mentions it. The scanner is the one `spec-refs` and `tools/renumber` use
(`tools/internal/specref`): bold `**17.7**`, `dil tanımı 17.7`, `§17.7`, link text, parenthesised numbers in the spec itself, and,
outside the spec, a bare `17.7` in prose or in a Go comment **when item 17.7 exists** (`15.5 ms` and version numbers are not
items). The mentioning place is the innermost declaration around the line (function, method, type, TR function or test,
document heading), the declaration just below when the mention sits in its doc comment, else the file. `who-uses 17.7`
is the impact list for a change or a renumbering (22 places today: documents, comments, tests); a test that mentions an item,
and a TR test block whose name starts with the number (`test "17.7 ..."`), also gets a `tests` edge, so `tests-for 17.7` lists
the tests of the rule. `tools/renumber` does not edit a bare number outside the spec (it may be a decimal): it lists it as
"review". Pinned by `TestSpecItemsAreNodesWithReferenceEdges`.

## `path`: directed by default

`path A B` follows edges in their direction (`calls`, `uses`, `dispatches_to`, `reads_var` ...; `contains` is not a
step) and says plainly when there is none: `no directed path from A to B` (exit 4). It used to fall back to an
undirected search, and once types became `uses` targets that turned a question like "does `Attention` call `Softmax`"
into the nonsense answer "both use the type `Tensor`". Two explicit options remain: `--undirected` ignores the
direction but never passes through a type, package, file or doc (the hubs that connect everything), `--through-hubs`
allows them and marks each `[hub]`. Pinned by `TestPathAnswersDirectedAndNeverSilentlyThroughATypeHub`.

## One rule for "a test reaches this through helpers"

Path constants, `go:embed` files, TR text in Go tests, the name codes' tests and the *indirect* tests of `tests-for` are
all the same question, so there is one function for it (`reachEdge` in `test_reach.go`; the walk and its limits,
`WalkCallers`, `TestMaxHops`, `TestMaxSteps`, live in `internal/devtools/codeindex/reach.go`, so `tr dizin sınayanlar`
answers with the same rule) and one rule: walk from the thing to its callers and users (EXTRACTED calls, type and const
uses, variable reads and writes, TR-to-TR calls even when INFERRED, since a TR method call on a receiver of unknown
type is an ordinary call; a Go function value passed around, INFERRED, is not followed), stop at the first test on a
chain, cross **at most one package boundary** and take **at most eight steps**.

What a test is, is one rule too (`test_rule.go`): a TR `test` block, or a Go function in a `_test.go` file whose name
is `Test`, `Benchmark`, `Fuzz` or `Example` followed by nothing or a non-lowercase letter (the `go test` rule).
`tests-for`, `changed`, the name-code tests and the spec tests all call `isTestNode`; `changed` lists the very tests
`tests-for` would list for each changed node (`testDistances`: direct, list walkers, chains, name codes; measurements
apart), and maps TR declarations as well as Go ones (`hasLineSpan`). Pinned by the planning exam (independent audit
Y1, Y3, T3).

Why these numbers. The honest chain inside the package that loads or implements something is long
(loader → parser → registry → accessor → test is 5 steps), so steps are generous (8) while boundaries are tight (1):
the tests that guard something come from its own package or from a package that calls it directly. Steps alone were
not usable (7 steps gave 209 tests for `tr.json`); the older limits (3 steps for path constants, 4 for indirect
tests) cut the honest inner chains. The old two places now call the one function.

## Tests written in TR (`tests/*.tr`, TR code inside Go test strings)

Behaviour is tested with TR code, and a test that the graph cannot see leaves `tests-for` empty. Three
things close that:

- **`tests/*.tr` is indexed** (`trRoots`), so its `test "..."` blocks are `tr_test` nodes. Member (`.güncelle`) and
  named-argument (`her_dakika:`) codes are builtin nodes in `tr dizin`, and every builtin a test block exercises is a
  `tests` edge to its name code (`tests-for TR_fn_default_device` lists `tests/tensor_test.tr:1044`). A function or method builtin is
  exercised when it is called; naming it as a value (`değişken f = zaman_ms`) is not a test of it. Constants,
  properties, types and named arguments are used by reading them (`codeindex.exercisesBuiltin`; independent audit S3).
- **TR in Go test strings** (`tr_snippets.go`): a string a Go test gives to a function that reaches the TR reader is
  a TR program; `codeindex.Snippet` reads the name codes it uses and the test gets a `tests` edge to each. The
  helper is not named: the language entry points (`lexer.New`, `source.Prepare` ...) are the sinks, and a function
  that passes its own string parameter to a sink makes that parameter a sink, until nothing changes. The text may be
  a constant, a `fmt.Sprintf` format (verbs become placeholders: `%q` → `"x"`), a named constant, a local variable,
  the text another function returns, parts joined with `+` (an unknown part becomes `x`; every pair of texts, at most 64),
  or a field of a table the test ranges over (`for _, c := range []struct{ code string }{...} { run(c.code) }`, the table
  written in the loop or assigned to a local; fields by name or position). A string that goes to a
  helper that never reaches the reader (`notTR(t, "kn.güncelle(1)")`) links nothing. Same chain rule as path
  constants (one package boundary) but at most four steps (half the limit): this edge is listed as a direct test, and
  a longer chain is weak evidence. The codes counted are the ones the text exercises (same rule as a test block).
- **TR written to a file the test hands to `tr`** (`tr_file_writes.go`): a `.tr` file is a TR program, so the content
  written to a path whose text ends in `.tr` is a TR text, linked the same way (`TestReadyModelMatchesTransformers` writes
  `hazır_model(...)` to `karsilastir.tr` and runs the binary on it). The file sink is `os.WriteFile`; a function that
  passes its own content and path parameters to a file sink is one too (`testkit.WritePath`, `testkit.WriteFile`, a test
  helper), until nothing changes. The path is read through `filepath.Join`, `FromSlash`, `[]byte(...)` and a map the
  test ranges over (`for name, text := range map[string]string{"a.tr": ...}`); a program built with `strings.Join` of
  written lines is read too. `Snippet` gets the repository's `lib/`, so a library name the text calls comes with its
  library code (`TR_fn_pretrained_model`). Text written to a file that is not `.tr` is not TR.
- **A Go function is tested through the name it implements**: `tests-for checkpoint.Checkpoint.saveOnInterval`
  walks up its callers (`update` implements `güncelle`, TR_mem_update) and lists the tests of the builtin, member and
  named-argument codes they use (`tests through the name codes it implements`, tests of the same package first).
  When the declaration itself implements the code and the test is in its package, the test is direct: `tests-for
  internal/lang/interp/stats.go` starts with `TestStatsNameTheBadValueByItsPlace  direct` (parti K2, A).
  Type names, keywords and messages do not count: every test uses them. A Go test counts only when it can run the
  function: the function's package is the test's own or one the test's package imports (transitively, test files'
  imports included). A test of `yazdır` in `checker` cannot run `tr.startsWithPrint` of `package main`, which nobody
  imports; it used to head `tests-for cmd/tr/main.go` with 168 such tests (TCB 2026-10-25, item 3;
  `TestNameCodeTestsMustLinkTheFunctionsPackage`). A TR test runs in the `tr` binary, which links every package.

A test that can skip carries the condition in its `skip` field, shown as `[skips when: g.lt == nil]` next to it; the condition is not evaluated. The body is walked statement by statement (control flow; closures and `t.Run` subtests are not entered, there is no statement limit): every path that reaches `t.Skip` adds its condition, made of the `if` branches taken (`(err := f(); err != nil)` with its init, `else` as the negation) and of the earlier silent returns it passed (`no earlier return (err := os.Symlink(...); err == nil)`: the skip after a helper's early return happens only when the link failed; a helper that skips after a loop of returns gets `no earlier return (...)`, not `always`); a branch ending in `t.Fatal` adds nothing (a failure, not an environment). A helper's label passes to its caller at the point of the call, also from inside an expression (`exec.Command(python(t), ...)`) and through helpers of helpers. A file build constraint (`//go:build unix`, `_amd64_test.go`) is part of it: `not amd64 (build constraint)`; a helper in another file that only one configuration builds passes its label under that constraint (`windows (build constraint) && (...)`: the helper of the other configuration is not in this graph, so the caller's path goes on), and a test in a file this configuration does not build (read without types) is labelled with the file's constraint and is not repeated under "in other build configurations" (audit group 7). When every listed test depends on its environment, `tests-for` adds a note that CI may run none of them; `testing.Short()` and `raceEnabled` alone do not count (CI runs them in its long and plain modes). The repository check `TestEveryRepositoryTestThatSkipsIsLabelled` finds every test that calls `t.Skip` (directly or through a test-file helper) and requires a label (the old leading-statement reader missed 22); `TestNoTestMainSkipsItsPackage` guards that no `TestMain` returns or exits 0 before `m.Run`, which labels do not read (trial 9 H2, audit group 6 H2). `tests-for` puts **the tests about it** first: a test whose name or doc comment names the target (a name code's names, a function's short name and the name codes it implements; a Go test by the words of its short name without the package, `TestTensorData` names `Data` but `dataset_test.TestReaderState` does not; a test in another package names a Go function's own short name only with its qualifier, the package for a function and the type for a method (`TestCodesName...`, `codes.Name` in the doc), since short names are often common words (audit group 7: checker's tests with "Name" were about `codes.Name`); a TR test by its description without the module) or that expects a message only the target produces (a Go test using the message code, and no function outside the target and what it calls produces that message: the wrapper `{fn}: {error}` made by twelve functions is nobody's own; a TR test line holding both the fixed text of a message that names a function and, in the `{fn}` slot, the target's name, as `yazdır() 'ayraç' adında bir parametre almaz` for H2427, while a line expecting `böl()`'s message does not expect `metin`). Each test is listed once, in the group of its strongest evidence: about it, then the tests that walk the whole list, then direct, then the chains below (a test in both inferred groups stays in its shorter one), weak evidence last; the planning exam checks this for every TR function, every name code with tests and every `internal/stdlib` function (trial 8 S1, audit group 6 S1). `tests-for` orders what it infers by **strength of evidence**: direct tests first, then the inferred groups (through calls, through the name codes a function implements) with the group holding the shortest, same-package chain first; tests found only through a chain longer than half the limit (more than 4 of 8 steps) go last under "weak evidence" (an 8-step chain usually passes through shared machinery: the interpreter, a registry). `tests-for` keeps **measurements** apart: `Benchmark*` functions and anything under `tools/bench` are listed under "measurements, not tests" (they time the code, they do not check it; the "measure first" rule wants them, a test count does not), direct and indirect alike. `tests-for` prints direct tests (a test that uses the name or file) before the tests that walk the whole list.
`name_tests_test.go` (`test_names_test.go`) keeps this from decaying: every builtin, member and named-argument code
has a direct test or is listed with a reason in `testdata/untested_names.txt` (today three gaps are written down
there honestly: `zaman_ms`, `ağırlık:`, `sapma:`). A stale exception fails too.

## Package variables: reads and writes (`reads_var`, `writes_var`, `inits_var`)

`query codes.active` used to give its 16 users under one `uses`; "who writes it" had no answer. The rule of
what a write is lives in one function, `writeTargets` (`var_access.go`): assignment, op-assignment, `++`/`--`,
`range` assignment, `&x`, `delete`/`clear`, `sync/atomic`. The graph extractor and the `package-state` audit both
call it, so there is no second rule: `TestPackageStateAndGraphAgreeOnWriters` pins that `package-state`'s writers
(init and initializers aside) are exactly the `writes_var` sites. Compared with the field rule (`reads_field`/
`writes_field`) the same modes apply, and an initializer or an `init` write is its own group because it runs
once, before anything reads: that is the part "who sets it up" and "who changes it while running" must not mix.

## Doc comments (`doc:` in `query`)

Every Go node (function, method, type, field, const, var, interface method) carries `doc`: the first
paragraph of its doc comment (at most 3 lines, 300 characters), else its end-of-line comment; for a
single-spec `const X = 1` or `type T int` the comment on the declaration counts. `query codes.active` prints
"active programın okunduğu ve çalıştığı kanonik dildir: her zaman Türkçe.", so the answer is not one file
open away. `//go:` directives are not comments. `.tr` nodes get the same line from `tr dizin`'s `Doc`.
Pinned by `TestQueryShowsGoDocComments`.

## Data files and `tests-for` (`embeds`, `reads_file`, `walks`)

Every file git tracks is a graph node: a data file (`tr.json`, `en.json`, `messages/*.json`, `definitions.json`,
anything `//go:embed`ded), and also a script, `.gitignore`, JavaScript, YAML or a `testdata` fixture (a `file` node
that only records the path, unless another extractor reads it). So a tracked path always resolves to its own file:
`tests-for tools/full_check.sh` is the script, not "ambiguous" between the declarations whose text mentions it
(`TestEveryTrackedFileIsANode` asks every tracked path; no exception list). A path to a file git does not track
answers "is a file git does not track: the graph reads tracked files only, nothing was searched for it" (exit 4), so
"not searched" and "searched, nothing found" differ: a tracked file or doc with no link says `no tests found
(searched: path constants, globs and go:embed in Go code, and the comments and messages of Go tests)`. For a doc,
`tests-for` also lists **the tests that guard the rules it states** (each `Denetim:` rule of a README with its tests:
the rule's text and its test change together), and the tests that cite it (`cites`, above). Three things tie a file
to tests that read it:

- **Where the file is read**: the `//go:embed` variable (`embeds`) and any non-test function that passes the
  file's path to `reporoot.Path`, `filepath.Join` or `Glob` (`reads_file`). Tests that call the loader, or a
  function that uses the embedded variable, are `tests` edges, followed up the call/use chain with two limits:
  at most one package boundary and eight steps. Steps alone were not a usable limit: from a loader to a function
  every package calls, hundreds of tests would "guard" the file.
- **Whole-list walkers** (`walks`): a test whose own `range` goes over a collection produced by the file's
  package (`for _, k := range codes.Codes()`) covers every entry; a test that asks for one entry (`codes.Name(k)`)
  does not. `tests-for` prints these first (with their rule line), the other tests after. This is what separates
  the 18 tests that guard every name of `tr.json` from the 200 that merely use a name.
- **Codes inherit their file**: each name or message code is `contains`-ed by every language file that defines it
  (`tr.json`, `en.json`, `definitions.json`), so `tests-for TR_fn_default_device` or `tests-for H4009` lists the walkers of those
  files: the question "which tests look at this new built-in" has an answer before the first edit.

`data_coverage_test.go` keeps the answer from going empty: every tracked `.json`, `.tr` and `.md` file (not
`testdata`) is tied to a test (a `tests`/`walks`/`cites` edge, or for a README one of its rules has a `Denetim:` test) or
is listed with a reason in `testdata/uncovered_data.txt` (a data file, not a Go string: a path-looking string in
test code would itself count as the file being tested). A stale exception also fails. Where a path is built from
a variable (`reporoot.Path("examples", lang, "*.tr")`) the variable part becomes `*`; a variable that ranges over a
list of string constants takes each of them (`for _, p := range []string{"docs/*.md"} { reporoot.Glob(reporoot.Path(p))
}`). A bare file name (no `/`) counts only as a root-relative path, and only where it is one: the single part given to
`reporoot.Path` (`reporoot.Path("README.md")`), or an element of a string list that also holds a path with `/`
(`[]string{"README.md", "SECURITY.md", "lib/README.md"}`); a list of bare names is a fixture, and a folder's own file
of the same name is not linked (`TestRootRelativePathsLinkFromTheRootOnly`; TCB 2026-10-25: the doc tests'
`belgeListesi` reads the root README and `docs/*.md`, `tests-for README.md` said "no tests found").

## Planning exam (`testdata/planning_questions.txt`)

The questions asked while planning real work with codegraph and `tr dizin` (five trials on 2026-10-23: a GPU memory
builtin, a debugging session, a checkpoint option, CPU softmax speed, the value-language rule), each with its expected
answer, are a data file; `TestPlanningQuestionsAreAnswered` asks them of **this repository** (not a fixture) and fails
naming the trial when a closed blind spot comes back. One line per question: `tool command args | expectation | ... |
src: trial, date`. Expectations are structural so a better output format does not break them: `contains`, `not`, `exit`,
`first-item`, `group: HEADING => TEXT`, `first-of`, `top: HEADING => N => TEXT`, `before: A => B`; `A || B` accepts either.
`tr dizin` questions go to codeindex exactly as the command does (default limit 12, `--tümü` for all). The graph and the
TR index are built once and every question is asked in-process (`runQueryOn`), so the exam takes about a minute
(2026-10-26, most of it the two builds and the tool comparisons) and runs under `-short` (the full check). A question
names a line by its text, `@{file#text}`, and a count by a group item (`TestPlanningQuestionsPinNoLineOrCount`). A new
trial adds lines, not code.

Writing the exam found one more blind spot: `query TR_fn_print` listed the builtin's Go registration among 170 TR users, beyond
the limit, under "registered or used in Go". That section now lists Go code only, registrations naming the implementation
first.

## Why this design (comparison)

[Graphify](https://github.com/Graphify-Labs/graphify) builds a graph from code (tree-sitter, about
40 languages), docs and media into `graph.json`, with `EXTRACTED` / `INFERRED` edge labels, a
`schema_version`, and `query`, `path`, `explain` commands; an ambiguous name is refused with the
candidates listed (exit 3), an unknown one exits 4. We take that shape: the two labels, the schema
version, the commands (`explain` is our `query`: node plus neighbours), the 0/3/4 exit codes and
the candidate list instead of a guess.

What we do differently, and why:

- **Go semantics instead of tree-sitter.** tree-sitter only sees syntax, so a call is guessed from
  names. `go/types` resolves the exact callee, method, interface and constant, so `calls`,
  `implements` and message/name uses are EXTRACTED, not INFERRED. One language, so no need for 40.
- **Repository-specific links no generic tool has:** message codes (`H…`) and name codes (`A…`) as
  nodes with their use sites, rules from the READMEs bound to the test that guards them
  (`Denetim:`), tests bound to what they test, Markdown headings and links.
- **Standard library only.** `golang.org/x/tools/go/packages` and `go/callgraph` would be a new
  module dependency, which `internal/layers_test.go` (`TestExternalDependencies`) allows only for
  `purego`. `go list -export` plus `go/importer` and `go/types` gives the same type information
  (the loader is the one of `tools/impact`); a static call graph needs only `Uses`, which we read
  ourselves. `callgraph` would add interface-dispatch edges (CHA/RTA); we leave them out on purpose
  and keep `implements`.
- **No model, no semantic pass.** Everything is deterministic and rebuildable in seconds; there is
  no community detection or free-text `query "question"`: names in, neighbours out.

`tools/impact` answers "what does changing this touch" live from the code (no graph file); codegraph
is the stored, wider graph (docs, rules, messages, tests) for navigation.

## Time

On this repository (2026-10-26: about 980 Go files, 25 000 nodes, 172 000 edges; `TestReadmeGraphSizeIsCurrent`
fails when one of them drifts by more than a quarter) `build` takes 7-8 s with the built binary and a warm Go build
cache (Windows; up to about 15 s on a loaded machine), and about 57 s with a cold cache in an older measurement
(almost all of it compiling the standard library export data that `go list -export` asks for; the module's own
packages are type-checked from source, not compiled). Query costs are under "Cost" above.

## Tests

The fixture is a small module in `testdata/mod` (packages, an interface, message and name
dictionaries, a README with rules, a doc with links, a broken link). Tests that build a graph in
a temporary git repository take several seconds each, so they call `slowFixture` and are skipped
by `-short`: `go test -short ./tools/codegraph` takes about 150 s, the full run about 260 s (2026-10-26, Windows; the
repository graph is built once per run and shared, `repoGraph`). The full check (`tools/full_check.sh`) runs Go tests
with `-short`; the skipped ones run in the `--kip=uzun` mode (CI).

- `extractor_digest_test.go`: `testdata/extractor_digest.txt` records the extractor revision and a
  digest of the extractor sources (comments and formatting do not count). If the code changes,
  the test fails and says what to do: raise `extractorRevision` (graph.go) when `graph.json`'s
  content changes, then write the printed lines into the file.
- `uses_coverage_test.go`: **no blind spot in who-uses.** Every use go/types records in `Info.Uses` (type, function, method, const, var, field; any position) must be covered by an edge into that object's node from the same file. Uses are found by one pass over every identifier through `objectID` (the object-to-node mapping), not by one branch per object kind, so a new kind of position cannot hide silently; this is how `gpu.MemorySample` (a type used only in signatures and a field type) was found missing. A small diverse module runs always, the whole repository (about 296 000 uses) runs without `-short`. Why a file and not a site: an edge keeps one site per (owner, target, kind), so an owner that is a package would hide the second file; owners are therefore as small as possible (the declared name, the file for `var _`).
- `graph_test.go`: nodes with file:line, `calls`, `implements`, `imports`, `uses_message`,
  `uses_name`, the three kinds of `tests` edge, `guarded_by` with its labels, headings, links, the
  broken-link and missing-test warnings, label and endpoint invariants, determinism, schema version.
- `query_test.go`: `query`, ambiguity (exit 3), not found (exit 4), `path`, `who-uses`,
  `tests-for` (direct and indirect, depth limit), text search, flags after names, usage errors,
  missing graph, `build`.
- `fresh_test.go`, `root_test.go`, `history_test.go`, `changed_test.go`, `deleted_test.go`: the
  stamp and automatic rebuild, finding the repository root, and the three git commands.
- `test_paths_test.go`, `js_loads_test.go`, `data_coverage_test.go`: a file's answer in `tests-for` (readers, citing
  tests, the guards of a doc's rules, JavaScript loads, an untracked file not searched) and, on this repository, every
  tracked path resolving to its own file (`TestEveryTrackedFileIsANode`).
- `library_imports_test.go`: an import is a dependency of test reach (a test string's `aktar "tr/X"`, plain and coded,
  the script a Go test runs, an importing `.tr` test file; `changed --go-tests` the same); on this repository, every
  library import of a test string and of a script run by `reporoot.Path` reaches `tests-for lib/X.tr` (read from the
  plain text, second path), and every library file starts with a behaviour test.
- `tr_snippets_test.go`, `tr_file_writes_test.go`: TR text in a Go test string, or written to a `.tr` file the test
  runs, links the test to its name codes (helpers, tables, maps, joins, library codes); on this repository every
  script an e2e test writes to a `.tr` file links its test to each code it uses (found from the plain syntax tree,
  second path).
- `dups_test.go`: the known duplicates from the Gemini duplicate audits (Parti 3 and 4: `teeWriter`,
  the `resign` closures, `registerIsFinite`/`registerRounding`; excerpts of the code at main
  `04c83dc0` in `testdata/dups`) are found by `dups`, the near-miss ones (`ShortNumber`/`FormatBytes`,
  `exprNames`/`exprCaptures`) by `similar`; the threshold; deterministic JSON; each exclusion
  silences exactly its case and `--no-exclude` shows it again, two production copies are still
  reported; generated files and tables are not duplicates; `--changed` on a temporary git repository
  (only the branch's new copy touches the set; a branch that is not checked out is refused).
