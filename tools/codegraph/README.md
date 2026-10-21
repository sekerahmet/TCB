# codegraph

A local, deterministic knowledge graph of this repository, so an agent (or a person) can ask
"who calls this", "which test guards that rule", "where is this message used" without reading many
files. No network, no model, no new dependency: Go's standard library only. Developer tool; not
shipped to users.

```sh
go run ./tools/codegraph build                  # writes .codegraph/graph.json (git ignores it)
go run ./tools/codegraph query messages.New     # a node and its neighbours
go run ./tools/codegraph path interp.VM H2100   # shortest path between two nodes
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
answered (`query "Cümle parçalardan"` finds the rule); several are listed (exit 3).

`tests-for` lists the direct tests (`tests` edges), then the indirect ones: callers are followed
backwards over EXTRACTED `calls` edges, at most 4 steps, up to the first test on each chain. They
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
| `tr_func`, `tr_method`, `tr_struct`, `tr_field`, `tr_const`, `tr_var`, `tr_test` | `tr:func:<file>:<name>`, `tr:method:<file>:<Struct>.<name>`, ... | `.tr` sources, see "TR sources" |
| `doc`, `heading` | `doc:<path>`, `heading:<path>#<slug>` | every `.md` file; headings use GitHub slugs |

| Edge | Meaning | Label |
| :-- | :-- | :-- |
| `contains` | package → file → func/type/method, doc → heading → rule | EXTRACTED |
| `imports` | package → package (module packages only) | EXTRACTED |
| `calls` | func → func; a call resolved by `go/types` | EXTRACTED; a function used as a value: INFERRED |
| `implements` | type → interface of the module, by `types.Implements` | EXTRACTED |
| `uses` | func (or package, for initialisers) → package-level `const` or `var` it refers to | EXTRACTED |
| `denotes` | Go constant `messages.H2100` / `codes.K003` → the message / name code node `H2100` / `K003` it stands for | EXTRACTED |
| `tests` | test → its package; test → function it calls (directly or through helpers in test files); test → function its name says (`TestParse`, `TestVM_Run`) | EXTRACTED / EXTRACTED / INFERRED |
| `uses_message` | func → `msg:Hnnnn`, from the `messages.Hnnnn` constant | EXTRACTED |
| `uses_name` | func → `name:Xnnn`, from the `codes.Xnnn` constant | EXTRACTED |
| `guarded_by` | rule → test func or test file named after `Denetim:` | EXTRACTED when the test sits under a path the rule names or under the README's folder, else INFERRED |
| `links_to` | heading → heading, doc, Go file, package or any file; Markdown links | EXTRACTED; link whose `#anchor` is missing: INFERRED (to the file) |

Every edge has `confidence`, and `file`/`line` of the first place it was seen and `count`.
`warnings` lists what could not be resolved: a `Denetim:` test that does not exist, a broken
Markdown link. They are real findings; `build` prints the first five.

Limits: only files built for the current OS are read (like `tools/impact`); calls through an
interface point at the interface method, not at implementers (use `implements`); `implements`
looks at non-test types only.

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
(through `internal/devtools/codeindex`, the TR code index package, which uses `internal/lang/source` and `parser`; coded files with a header are read too, lines counted as
everywhere else). The graph does not depend on the active writing language (`TR_DIL`): a file without
the header is always read as Turkish, a coded one is first decoded to Turkish (`source.Decode` with
`source.Turkish()`), never through `source.Prepare`, which follows the active language; the test
`TestTRGraphIsIndependentOfTheActiveLanguage` builds the fixture graph under `TR_DIL=tr` and `TR_DIL=en`
in sub-processes and compares the `.tr` facts. Node kinds: `tr_func`, `tr_struct`, `tr_method`, `tr_field`, `tr_const`, `tr_var` (top level)
and `tr_test` (a `test` block), all with file:line:end; the file is a `file` node `tr:file:<path>`
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
  `main`; or a literal `a..b` / `a...b`) onto function, type and doc-section line ranges, and onto
  the codes named in changed dictionary lines. Then it lists the tests that exercise the changed
  code (direct first, then indirect with their distance, first `--limit`), and the other local
  `claude/` branches that touch the same nodes or files, with the result of a
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
  was produced by it). Name codes are never really deleted: a removed or moved name stays in the
  dictionary with its `kaldırıldı` / `taşındı` field, so the list for `names` is empty. History
  before a file was renamed is not visible to it.

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
- `graph_test.go`: nodes with file:line, `calls`, `implements`, `imports`, `uses_message`,
  `uses_name`, the three kinds of `tests` edge, `guarded_by` with its labels, headings, links, the
  broken-link and missing-test warnings, label and endpoint invariants, determinism, schema version.
- `query_test.go`: `query`, ambiguity (exit 3), not found (exit 4), `path`, `who-uses`,
  `tests-for` (direct and indirect, depth limit), text search, flags after names, usage errors,
  missing graph, `build`.
- `fresh_test.go`, `root_test.go`, `history_test.go`, `changed_test.go`, `deleted_test.go`: the
  stamp and automatic rebuild, finding the repository root, and the three git commands.
