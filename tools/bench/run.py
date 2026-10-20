"""TR speed comparison: runs the same programs in TR, Python and Go and compares the times.

Usage (from the repository root):  python tools/bench/run.py [--repeat 3] [--only tr] [--list] [test ...]

Each program prints its own measured time (ms) and a result value; the result values are expected to
be the same in all three languages. The interpreter and the Go program are rebuilt into a temporary
folder on every run (GOAMD64 and TR_CPU_AVX2 from the environment apply; AVX2 kernels are chosen at
run time). The Python version of matris_yerlesik needs numpy; without it the test is skipped.
"""
import argparse, json, os, subprocess, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
BENCH = os.path.join(ROOT, "tools", "bench")
TESTS = ["dongu", "fib", "sirala", "metin", "sozluk", "matris_dongu", "matris_yerlesik"]
EXE = ".exe" if os.name == "nt" else ""


def build(tmp):
    tr = os.path.join(tmp, "tr" + EXE)
    go = os.path.join(tmp, "compare" + EXE)
    env = dict(os.environ)
    subprocess.run(["go", "build", "-o", tr, "./cmd/tr"], cwd=ROOT, check=True, env=env)
    subprocess.run(["go", "build", "-o", go, "./tools/bench/go"], cwd=ROOT, check=True, env=env)
    return tr, go


def run(command, repeat):
    """Returns (best time in ms, result value), or (None, last error line)."""
    best, value = None, None
    for _ in range(repeat):
        r = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", timeout=600, cwd=ROOT)
        lines = r.stdout.strip().splitlines()
        if r.returncode != 0 or not lines:
            return None, (r.stderr.strip().splitlines() or ["error"])[-1]
        parts = lines[-1].split()
        ms = int(parts[-1])
        best = ms if best is None else min(best, ms)
        value = parts[1]
    return best, value


def parser():
    ap = argparse.ArgumentParser(description="TR / Python / Go speed comparison")
    ap.add_argument("tests", nargs="*", choices=TESTS, metavar="test",
                    help="tests to run (default: all): " + " ".join(TESTS))
    ap.add_argument("--repeat", type=int, default=3, help="runs per test; the best time counts (default 3)")
    ap.add_argument("--only", choices=["tr", "py", "go"], help="run only this language")
    ap.add_argument("--json", help="write the results to this file")
    ap.add_argument("--list", action="store_true", help="print the test names and exit")
    return ap


def main():
    a = parser().parse_args()
    if a.list:
        print("\n".join(TESTS))
        return
    if a.repeat < 1:
        parser().error("--repeat must be at least 1")
    tests = a.tests or TESTS
    langs = [a.only] if a.only else ["tr", "py", "go"]
    results = {}
    with tempfile.TemporaryDirectory() as tmp:
        tr, go = build(tmp)
        for t in tests:
            commands = {
                "tr": [tr, os.path.join(BENCH, "tr", t + ".tr")],
                "py": [sys.executable, os.path.join(BENCH, "py", "compare.py"), t],
                "go": [go, t],
            }
            results[t] = {lang: run(commands[lang], a.repeat) for lang in langs}
            cells = []
            for lang in langs:
                ms, _ = results[t][lang]
                cells.append(f"{lang.upper():>2} {'-' if ms is None else ms:>7} ms")
            values = {v for _, v in results[t].values()}
            line = f"{t:16} " + " | ".join(cells)
            if "tr" in langs and "py" in langs and results[t]["tr"][0] and results[t]["py"][0]:
                line += f" | TR/Python {results[t]['tr'][0] / max(results[t]['py'][0], 1):5.1f}x"
            if len(langs) > 1:
                line += " | same result" if len(values) == 1 else f" | RESULT DIFFERS {values}"
            print(line, flush=True)
    if a.json:
        with open(a.json, "w", encoding="utf-8") as f:
            json.dump(results, f, ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
