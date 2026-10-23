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
```

Flags: `build --root DIR --out FILE --warnings`; queries take `--graph FILE`, `--limit N` (lines per
group, default 12) and `--all`, before or after the names. Output is short plain text; edges tagged
`[INFERRED]` are guesses, unmarked ones are `EXTRACTED`. `explain` is another name for `query`.

## Staying fresh

Nobody has to remember `build`. `build` writes a `stamp` into `graph.json`: `git rev-parse HEAD`
plus a digest of the changed files (`git status --porcelain -uall`, tracked or untracked, only
`.go`, `.json` and `.md`, with their contents; `.codegraph/` itself is ignored). Every query
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

Cost when the graph is fresh (this repository, Windows, the two git calls): a query takes about
0.21 s instead of 0.14 s with `--no-rebuild`, so about 80 ms. A stale graph costs one `build`
(about 3 s with a warm Go build cache).

Exit codes: `0` answered, `1` failure (no graph, load error), `2` usage, `3` ambiguous name (the
candidates are listed on stdout; repeat with an id or a longer name), `4` not found (similar names
on stderr; for `path`: no path).

A name is resolved in this order, first tier with a match wins: node id (with or without its
`kind:` prefix), exact name (`messages.New`, `H2100`, `interp.VM.Interrupt`), name ending at a `.`
or `/` boundary (`New`, `lang/messages.New`), same case-insensitively. When none matches and the
query has at least 3 characters, text search: a name that starts with it, a name that contains it,
a `detail` (message text, rule text, heading) that contains it, all case-insensitive. One match is
answered (`query "Cümle parçalardan"` finds the rule); several are listed (exit 3). Text queries try whole words first (Turkish case folding, word boundary; name, then detail and doc comment), then a name prefix, then a substring: `query "ölü"` finds the doc comments that say "ölü", not every "bölüm".

`tests-for` lists the direct tests (`tests` edges), then the indirect ones: callers are followed
backwards over the chain rule below (EXTRACTED `calls`, TR-to-TR `calls`, `uses`, `reads_var`, `writes_var`), up to the first test (Go or TR) on each chain. They
are shown nearest first as `N steps via <caller next to the target>`, always `[INFERRED]`. This is
computed when you ask, not stored in `graph.json`.

## Graph

`graph.json` has `schema_version` (readers refuse other versions), `module`, `nodes`, `edges` and
`warnings`. It is sorted and carries no timestamp: the same tree gives the same bytes.

| Node kind | Id | Source |
| :-- | :-- | :-- |
| `package`, `file`, `func`, `method`, `type`, `const`, `var` | `go:pkg:`, `go:file:`, `go:func:`, `go:type:`, `go:const:`, `go:var:` + path/name | `go list` + `go/types`, test files included; file:line; `const`/`var` are package-level only (`lexer.TokVar`, `messages.H2100`) |
| `message` | `msg:H2100` | `internal/lang/messages/tr.json` (text) and `definitions.json` (params) |
| `name` | `name:A001` | `internal/lang/codes/tr.json` (name and family) |
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
| `uses` | owner → package-level `type` or `const` it refers to (variables are `reads_var`/`writes_var`), wherever the name is written: signatures (parameter, result, receiver), field types, embedded types, interface method signatures, type constraints, composite literals, initialisers. The owner is the function or method; for a type declaration the type; for a package-level `const`/`var` the declared name (`var ValInt = TypeName(codes.A200)`: ValInt), for an unnamed spec (`var _ T = ...`) the file | EXTRACTED |
| `refs_spec` | the declaration, heading or file that mentions a language-definition item → `spec:N.M` (see above) | EXTRACTED |
| `reads_var` | owner → package-level `var` it reads | EXTRACTED; access sites with mode `read` (also the read half of `x += 1`, `x++`: mode `read` next to the `read+write` write site) |
| `writes_var` | owner → package-level `var` it changes: assignment, `+=`, `++`/`--`, element or field write (`m[k] = v`, `v.f = x`), `delete`/`clear`, `&v`, `sync/atomic` call, qualified write from another package (`a.V = 1`) | EXTRACTED; site modes `write`, `read+write`, `elem-write`, `addr`, `atomic` (same vocabulary as `writes_field`) |
| `inits_var` | a write inside a package's `func init` (owner = that init), and the variable's own declaration initializer (`var active = load()`: the variable to itself, mode `initializer`) | EXTRACTED; "who sets it up" is a separate group in `query`, apart from the steady-state writers |
| `embeds` | `//go:embed` variable → the data file(s) its patterns name (relative to the package folder) | INFERRED |
| `reads_file` | function (non-test) → repository file named by a string constant it passes to `reporoot.Path`/`filepath.Join`/`Glob` | INFERRED |
| `walks` | test → data file whose whole list it walks: the test (or a test helper) has a `range` over a function or variable that returns a collection and depends on the file inside the package that embeds it (`for _, k := range codes.Codes()`) | INFERRED |
| `denotes` | Go constant `messages.H2100` / `codes.K003` → the message / name code node `H2100` / `K003` it stands for | EXTRACTED |
| `tests` | test → its package; test → function it calls (directly or through helpers in test files); test → function its name says (`TestParse`, `TestVM_Run`) | EXTRACTED / EXTRACTED / INFERRED Also test → repository file (a `.tr` file, a doc) when the test, or a helper it calls through the chain rule below, writes the path as a string constant: `reporoot.Path("tools/gpu/gpt_training.tr")`, `reporoot.Path("lib", "*.tr")` or a `filepath.Glob` pattern; a bare wildcard literal is test data and does not count, nor does a bare file name (no `/`); INFERRED, so `tests-for lib/egitim.tr` lists the Go tests that guard it. `tests-for` writes under each Go test the first line of the rule it guards (the test's doc comment, else the first `//` block after the imports), read from the file at query time. `tr dizin` stays TR-only (it parses `.tr`, not Go); the Go-to-`.tr` link lives here. |
| `uses_message` | func → `msg:Hnnnn`, from the `messages.Hnnnn` constant | EXTRACTED |
| `uses_name` | func → `name:Xnnn`, from the `codes.Xnnn` constant; message → `name:K4nn` when its template (any language) writes that environment variable's name (`TR_GPU_SYNC=1`) | EXTRACTED |
| `uses_env` | func → `name:K4nn`, from `os.Getenv`/`LookupEnv`/`Setenv`/`Unsetenv` (or `testing.T.Setenv`) given a string constant (or a named one) equal to the variable's name in any writing language; the compute core cannot see name codes, so it reads the variable by its source-language name | EXTRACTED |
| `guarded_by` | rule → test func or test file named after `Denetim:` | EXTRACTED when the test sits under a path the rule names or under the README's folder, else INFERRED |
| `links_to` | heading → heading, doc, Go file, package or any file; Markdown links | EXTRACTED; link whose `#anchor` is missing: INFERRED (to the file) |

Every edge has `confidence`, and `file`/`line` of the first place it was seen and `count`.
`warnings` lists what could not be resolved: a `Denetim:` test that does not exist, a broken
Markdown link. They are real findings; `build` prints the first five.

Limits: only files built for the current OS are read (like `tools/impact`); `implements` and
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
(through `internal/devtools/codeindex`, the TR code index package, which uses `internal/lang/source` and `parser`; coded files with a header are read too, lines are the file's real lines, the header line counts). The graph does not depend on the active writing language (`TR_DIL`): a file without
the header is always read as Turkish, a coded one is first decoded to Turkish (`source.Decode` with
`source.Turkish()`), never through `source.Prepare`, which follows the active language; the test
`TestTRGraphIsIndependentOfTheActiveLanguage` builds the fixture graph under `TR_DIL=tr` and `TR_DIL=en`
in sub-processes and compares the `.tr` facts. Syntax errors in the intentional error examples (`examples/*/hatalar/`, `examples/*/errors/`) are not warnings. Node kinds: `tr_func`, `tr_struct`, `tr_method`, `tr_field`, `tr_const`, `tr_var` (top level)
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

`query Muon`, `query katman`, `who-uses dikkat` (the code T503 with its TR call sites) work on TR.

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
  named on the same line (`{codes.G502, timeMsBuiltin}`), and the heading in `docs/codes.md`.
  Name-code nodes carry both language names in `aliases`.

## History commands (from git, never stored)

These three ask git when you run them; `graph.json` stays small and `build` fast. They need a git
repository (a failing git is an error, exit 1).

- `history <name>`: commits (short hash, date, subject; no diffs), newest first, `--limit N`
  (`--all` for every one). A function, method or type uses `git log -L` (by name when the
  declaration is a plain `func`/`type` line, else by its current line range); a message or name
  code uses a pickaxe over the dictionary files and marks the commits that **added** and
  **removed** it; a file or doc uses `git log --follow`.
- `changed <branch|range>`: maps the hunks of `git diff -U0 <base>...<branch>` (`--base`, default
  `main`; or a literal `a..b` / `a...b`) onto Go and TR declaration and doc-section line ranges, and onto
  the codes named in changed dictionary lines. Then it lists the tests that exercise the changed
  code (the same tests `tests-for` lists for each changed node; direct first, then indirect with their distance,
  first `--limit`), and the other local `claude/` branches not yet merged into the base that touch the same nodes or
  files, with the result of a
  `git merge-tree --write-tree` trial merge (`clean` or the conflicting files). The graph is of
  the checked-out tree: when the ref is the checked-out HEAD the new-side lines are mapped (new
  functions included), otherwise the old-side lines, which is exact only while the tree equals the
  merge base. Added code with no old-side counterpart is counted under "mapped to no node" (by file) and,
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
  it is refused. Pinned by `TestDupsChangedUsesTheMergeBaseNotTheTreeDiff`.
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
the items of a `const (...)` block, back-to-back registration calls `wrap(v, codes.T500, ...)`,
a kernel's `x := k.pointer("x")` parameter declarations, consecutive one-statement test
functions. The order of such a list is information, each unit says something else, and a real copy
of a list is a copy of a table, not of logic; 40 nodes is the size of `func TestX(t) { expectOut(t,
`...`, `...`) }`, so a unit with a branch or a loop is never a sibling); `examples/tr` against `examples/en` are
`.tr` files, which this command does not read. To add an exception write one entry in
`dups_exclude.go` with a reason.

**Limits.** Exact-shape clones only (type 1 and 2 in the clone literature): a copy with an extra
statement is found by `similar`, not by `dups`. A match may begin in the middle of a function (the
span is the lines its nodes cover). Switch arms and sequences of one-line wrappers are reported
although they are rarely worth merging: judge each candidate. On this repository `dups` takes about
1 s and reads about 700 000 nodes of 800 files; at `--min 100` it lists about 30 candidates.

**For `.tr` code** the same idea belongs in `tr dizin` (the TR code index already has the syntax
trees and a language-independent reader): a `tr dizin kopya` subcommand over the `codeindex`
parser output with the same node sequence, which would see the parallel `examples/tr` and
`examples/en` as one family; see the hand-over note for the proposal.

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
  named-argument (`her_dakika:`) codes are builtin nodes in `tr dizin`, and every builtin a test block uses is a
  `tests` edge to its name code (`tests-for T720` lists `tests/tensor_test.tr:1044`).
- **TR in Go test strings** (`tr_snippets.go`): a string a Go test gives to a function that reaches the TR reader is
  a TR program; `codeindex.Snippet` reads the name codes it uses and the test gets a `tests` edge to each. The
  helper is not named: the language entry points (`lexer.New`, `source.Prepare` ...) are the sinks, and a function
  that passes its own string parameter to a sink makes that parameter a sink, until nothing changes. The text may be
  a constant, a `fmt.Sprintf` format (verbs become placeholders: `%q` → `"x"`), a named constant, a local variable,
  the text another function returns, or parts joined with `+` (an unknown part becomes `x`). A string that goes to a
  helper that never reaches the reader (`notTR(t, "kn.güncelle(1)")`) links nothing. Same chain rule as path
  constants (one package boundary, eight steps).
- **A Go function is tested through the name it implements**: `tests-for checkpoint.Checkpoint.saveOnInterval`
  walks up its callers (`update` implements `güncelle`, Y010) and lists the tests of the builtin, member and
  named-argument codes they use (`tests through the name codes it implements`, tests of the same package first).
  Type names, keywords and messages do not count: every test uses them.

`tests-for` orders what it infers by **strength of evidence**: direct tests first, then the inferred groups (through calls, through the name codes a function implements) with the group holding the shortest, same-package chain first; tests found only through a chain longer than half the limit (more than 4 of 8 steps) go last under "weak evidence" (an 8-step chain usually passes through shared machinery: the interpreter, a registry). `tests-for` keeps **measurements** apart: `Benchmark*` functions and anything under `tools/bench` are listed under "measurements, not tests" (they time the code, they do not check it; the "measure first" rule wants them, a test count does not), direct and indirect alike. `tests-for` prints direct tests (a test that uses the name or file) before the tests that walk the whole list.
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

A data file (`tr.json`, `en.json`, `messages/*.json`, `definitions.json`, anything `//go:embed`ded) is a graph
node, so `tests-for internal/lang/codes/tr.json` is no longer "not found". Three things tie it to tests:

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
  (`tr.json`, `en.json`, `definitions.json`), so `tests-for T720` or `tests-for H4009` lists the walkers of those
  files: the question "which tests look at this new built-in" has an answer before the first edit.

`data_coverage_test.go` keeps the answer from going empty: every tracked `.json`, `.tr` and `.md` file (not
`testdata`) is tied to a test (a `tests`/`walks` edge, or for a README one of its rules has a `Denetim:` test) or
is listed with a reason in `testdata/uncovered_data.txt` (a data file, not a Go string: a path-looking string in
test code would itself count as the file being tested). A stale exception also fails. Where a path is built from
a variable (`reporoot.Path("examples", lang, "*.tr")`) the variable part becomes `*`.

## Planning exam (`testdata/planning_questions.txt`)

The questions asked while planning real work with codegraph and `tr dizin` (five trials on 2026-10-23: a GPU memory
builtin, a debugging session, a checkpoint option, CPU softmax speed, the value-language rule), each with its expected
answer, are a data file; `TestPlanningQuestionsAreAnswered` asks them of **this repository** (not a fixture) and fails
naming the trial when a closed blind spot comes back. One line per question: `tool command args | expectation | ... |
src: trial, date`. Expectations are structural so a better output format does not break them: `contains`, `not`, `exit`,
`first-item`, `group: HEADING => TEXT`, `first-of`, `top: HEADING => N => TEXT`, `before: A => B`; `A || B` accepts either.
`tr dizin` questions go to codeindex exactly as the command does (default limit 12, `--tümü` for all). The graph and the
TR index are built once and every question is asked in-process (`runQueryOn`), so the exam takes about 7 s and runs under
`-short` (the full check). A new trial adds lines, not code.

Writing the exam found one more blind spot: `query G100` listed the builtin's Go registration among 170 TR users, beyond
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

On this repository (about 640 Go files, 9 000 nodes, 36 000 edges) `build` takes about 3 s with a
warm Go build cache and about 57 s with a cold one (almost all of it compiling the standard library
export data that `go list -export` asks for; the module's own packages are type-checked from
source, not compiled). Queries read the JSON file and answer in well under a second.

## Tests

The fixture is a small module in `testdata/mod` (packages, an interface, message and name
dictionaries, a README with rules, a doc with links, a broken link). Tests that build a graph in
a temporary git repository take several seconds each, so they call `slowFixture` and are skipped
by `-short`: `go test -short ./tools/codegraph` takes about 17 s, the full run about 45 s. The
full check (`tools/full_check.sh`) runs Go tests with `-short`; the skipped ones run in the
`--kip=uzun` mode (CI).

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
- `dups_test.go`: the known duplicates from the Gemini duplicate audits (Parti 3 and 4: `teeWriter`,
  the `resign` closures, `registerIsFinite`/`registerRounding`; excerpts of the code at main
  `04c83dc0` in `testdata/dups`) are found by `dups`, the near-miss ones (`ShortNumber`/`FormatBytes`,
  `exprNames`/`exprCaptures`) by `similar`; the threshold; deterministic JSON; each exclusion
  silences exactly its case and `--no-exclude` shows it again, two production copies are still
  reported; generated files and tables are not duplicates; `--changed` on a temporary git repository
  (only the branch's new copy touches the set; a branch that is not checked out is refused).
