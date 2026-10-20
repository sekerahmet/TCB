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
```

Flags: `build --root DIR --out FILE --warnings`; queries take `--graph FILE`, `--limit N` (lines per
group, default 12) and `--all`, before or after the names. Output is short plain text; edges tagged
`[INFERRED]` are guesses, unmarked ones are `EXTRACTED`.

Exit codes: `0` answered, `1` failure (no graph, load error), `2` usage, `3` ambiguous name (the
candidates are listed on stdout; repeat with an id or a longer name), `4` not found (similar names
on stderr; for `path`: no path).

A name is resolved in this order, first tier with a match wins: node id (with or without its
`kind:` prefix), exact name (`messages.New`, `H2100`, `interp.VM.Interrupt`), name ending at a `.`
or `/` boundary (`New`, `lang/messages.New`), same case-insensitively.

## Graph

`graph.json` has `schema_version` (readers refuse other versions), `module`, `nodes`, `edges` and
`warnings`. It is sorted and carries no timestamp: the same tree gives the same bytes.

| Node kind | Id | Source |
| :-- | :-- | :-- |
| `package`, `file`, `func`, `method`, `type` | `go:pkg:`, `go:file:`, `go:func:`, `go:type:` + path/name | `go list` + `go/types`, test files included; file:line |
| `message` | `msg:H2100` | `internal/lang/messages/tr.json` (text) and `definitions.json` (params) |
| `name` | `name:A001` | `internal/lang/codes/tr.json` (name and family) |
| `rule` | `rule:<readme>:<line>` | a list item or paragraph with `Denetim:`, or a row of a `Kural` / `Test` table |
| `doc`, `heading` | `doc:<path>`, `heading:<path>#<slug>` | every `.md` file; headings use GitHub slugs |

| Edge | Meaning | Label |
| :-- | :-- | :-- |
| `contains` | package → file → func/type/method, doc → heading → rule | EXTRACTED |
| `imports` | package → package (module packages only) | EXTRACTED |
| `calls` | func → func; a call resolved by `go/types` | EXTRACTED; a function used as a value: INFERRED |
| `implements` | type → interface of the module, by `types.Implements` | EXTRACTED |
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
dictionaries, a README with rules, a doc with links, a broken link).

- `graph_test.go`: nodes with file:line, `calls`, `implements`, `imports`, `uses_message`,
  `uses_name`, the three kinds of `tests` edge, `guarded_by` with its labels, headings, links, the
  broken-link and missing-test warnings, label and endpoint invariants, determinism, schema version.
- `query_test.go`: `query`, ambiguity (exit 3), not found (exit 4), `path`, `who-uses`,
  `tests-for`, flags after names, usage errors, missing graph, `build`.
