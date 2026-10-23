package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/sekerahmet/TCB/tools/internal/specref"
)

// `spec-refs`: dil tanımı (docs/spec) madde atıflarının denetimi. docs/, tests/*.tr, Go yorumları, lib/ ve
// examples/ dosyalarında `**N.M**`, `dil tanımı N.M`, `§N.M`, `[N.M](…spec/…)` ve spec dosyalarında
// parantez içi anmalar taranır (tarayıcı tools/internal/specref'tedir; `tools/renumber` aynısını kullanır).
//   - olmayan maddeye atıf hatadır (çıkış kodu 1); spec içindeki zayıf anmalar yalnızca not düşer;
//   - numaradan sonraki ekin ses uyumu denetlenir (`15.9'daki`, `15.24'teki`);
//   - `--list` her atıfı hedef maddenin başlığıyla yazar: atıf doğru maddeyi mi gösteriyor, göz atılır.
// Kaynak Go kodunu yüklemez; yalnızca metin tarar, bu yüzden hızlıdır.

// runSpecRefs `spec-refs` komutudur.
func runSpecRefs(args []string, stdout, stderr io.Writer) int {
	list := false
	o, rest := parseAuditFlagsWith("spec-refs", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&list, "list", false, "list every reference with the heading of its target instead of the findings")
	})
	if o == nil {
		return exitUsage
	}
	if len(rest) != 0 {
		fmt.Fprintf(stderr, "codegraph spec-refs: unexpected argument %q\n%s", rest[0], usage)
		return exitUsage
	}
	root, err := resolveRoot(o.root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	files, err := specref.Collect(root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return exitFail
	}
	ix := specref.BuildIndex(files)
	if len(ix.Items) == 0 {
		fmt.Fprintln(stderr, "codegraph: no language-definition items found (docs/spec/NN_*.md with **N.M** lines)")
		return exitFail
	}
	ms := specref.Scan(files, ix)
	findings := filterFindings(specref.Check(ms, ix), o.in)
	errors := 0
	for _, f := range findings {
		if f.Severity == "error" {
			errors++
		}
	}
	if list {
		return writeRefList(stdout, stderr, o, filterRefs(specref.Refs(ms, ix), o.in))
	}
	if o.asJSON {
		if code := writeJSON(stdout, stderr, map[string]any{"schema": "codegraph-spec-refs/1", "items": len(ix.Items),
			"mentions": len(ms), "findings": findings}); code != exitOK {
			return code
		}
	} else {
		fmt.Fprintf(stdout, "%d item(s), %d mention(s), %d error(s), %d note(s)\n", len(ix.Items), len(ms), errors, len(findings)-errors)
		forEachLimited(stdout, findings, o.limit, "  ", moreHint, func(f specref.Finding) string {
			return fmt.Sprintf("%s:%d  %s  [%s] %s", f.File, f.Line, f.Severity, f.Check, f.Message)
		})
	}
	if errors > 0 {
		return exitFail
	}
	return exitOK
}

func filterFindings(list []specref.Finding, in string) []specref.Finding {
	var out []specref.Finding
	for _, f := range list {
		if underIn(f.File, in) {
			out = append(out, f)
		}
	}
	return out
}

func filterRefs(list []specref.Ref, in string) []specref.Ref {
	var out []specref.Ref
	for _, r := range list {
		if underIn(r.File, in) {
			out = append(out, r)
		}
	}
	return out
}

// writeRefList `--list` çıktısıdır: atıf → hedef başlığı.
func writeRefList(stdout, stderr io.Writer, o *auditOpts, refs []specref.Ref) int {
	if o.asJSON {
		return writeJSON(stdout, stderr, map[string]any{"schema": "codegraph-spec-refs-list/1", "refs": refs})
	}
	fmt.Fprintf(stdout, "%d reference(s) → target heading\n", len(refs))
	forEachLimited(stdout, refs, o.limit, "  ", moreHint, func(r specref.Ref) string {
		return fmt.Sprintf("%s:%d  %s → %s  (%s)", r.File, r.Line, r.Ref, strings.TrimSpace(r.Target), r.Where)
	})
	return exitOK
}
