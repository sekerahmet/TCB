"""TR speed comparison: runs the same programs in TR, Python, Go and Rust and compares the times.

Usage (from the repository root):  python tools/bench/run.py [--repeat 3] [--only tr] [--threads 2] [--list] [test ...]

Each program prints its own measured time (ms) and a result value; the result values are expected to
be the same in all languages. Rust runs when rustc is on PATH. The interpreter and the Go program are rebuilt into a temporary
folder on every run (GOAMD64 and TR_CPU_AVX2 from the environment apply; AVX2 kernels are chosen at
run time). The Python version of matris_yerlesik needs numpy; without it the test is skipped.
"""
import argparse, json, os, re, shutil, subprocess, sys, tempfile, time

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
BENCH = os.path.join(ROOT, "tools", "bench")
TESTS = ["dongu", "fib", "sirala", "metin", "sozluk", "matris_dongu", "matris_yerlesik"]
EXE = ".exe" if os.name == "nt" else ""
# The model test: example 36's character model in TR and in PyTorch (py/char_model.py), ms per step.
MODEL = "karakter_modeli"
MODEL_TR = os.path.join("examples", "tr", "36_karakter_modeli.tr")
MODEL_STEPS = 300


def build(tmp):
    tr = os.path.join(tmp, "tr" + EXE)
    go = os.path.join(tmp, "compare" + EXE)
    env = dict(os.environ)
    subprocess.run(["go", "build", "-o", tr, "./cmd/tr"], cwd=ROOT, check=True, env=env)
    subprocess.run(["go", "build", "-o", go, "./tools/bench/go"], cwd=ROOT, check=True, env=env)
    return tr, go


def build_rust(tmp):
    """The Rust version, or None when there is no rustc (the Rust column is then left out)."""
    if shutil.which("rustc") is None:
        return None
    rs = os.path.join(tmp, "compare_rs" + EXE)
    subprocess.run(["rustc", "-C", "opt-level=3", "-o", rs, os.path.join(BENCH, "rs", "compare.rs")],
                   cwd=ROOT, check=True)
    return rs


def run(command, repeat):
    """Returns (best time in ms, result value), or (None, last error line)."""
    best, value = None, None
    for _ in range(repeat):
        r = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", timeout=600, cwd=ROOT)
        lines = r.stdout.strip().splitlines()
        if r.returncode != 0 or not lines:
            return None, (r.stderr.strip().splitlines() or ["error"])[-1]
        parts = lines[-1].split()
        ms = float(parts[-1])  # Go and Rust write fractional ms (microsecond resolution)
        best = ms if best is None else min(best, ms)
        value = parts[1]
    return best, value


def model_program(tmp, steps):
    """A copy of the TR character model example that trains `steps` steps and reads the data from ROOT."""
    with open(os.path.join(ROOT, MODEL_TR), encoding="utf-8") as f:
        source = f.read()
    for old, new in (("TR_const ADIM = 300", f"TR_const ADIM = {steps}"),
                     ('TR_read_file("../../"', 'TR_read_file("' + ROOT.replace("\\", "/") + '/"')):
        if source.count(old) != 1:
            raise SystemExit(f"{MODEL_TR}: '{old}' not found once; update model_program")
        source = source.replace(old, new)
    path = os.path.join(tmp, f"model_{steps}.tr")
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(source)
    return path


def model(tr, tmp, threads, repeat, langs):
    """Time per training step (ms) of the character model in TR and in PyTorch, and the final loss.

    Each program runs once with MODEL_STEPS steps and once with none; the difference of the best
    wall times divided by the steps is the step time. Start-up (import torch, the interpreter), data
    loading and text generation are the same work on both sides and cancel out.
    """
    env = dict(os.environ, GOMAXPROCS=str(threads), PYTHONIOENCODING="utf-8")
    py = [sys.executable, os.path.join(BENCH, "py", "char_model.py")]
    commands = {
        "tr": lambda n: [tr, model_program(tmp, n)],
        "py": lambda n: py + [str(n), str(threads)],
    }
    results = {}
    for lang in (l for l in langs if l in commands):
        command = commands[lang]
        times, loss = {}, None
        for n in (MODEL_STEPS, 0):
            for _ in range(repeat):
                start = time.perf_counter()
                r = subprocess.run(command(n), capture_output=True, text=True, encoding="utf-8",
                                   errors="replace", timeout=1200, cwd=ROOT, env=env)
                ms = (time.perf_counter() - start) * 1000
                if r.returncode != 0:
                    results[lang] = (None, (r.stderr.strip().splitlines() or ["error"])[-1])
                    break
                times[n] = min(times.get(n, ms), ms)
                losses = [line for line in r.stdout.splitlines() if "kayıp = " in line]
                if n and losses:
                    loss = f"{float(losses[-1].split('kayıp = ')[1]):.2f}"
            if lang in results:
                break
        if lang not in results:
            results[lang] = (round((times[MODEL_STEPS] - times[0]) / MODEL_STEPS, 2), loss)
    return results


def parser():
    ap = argparse.ArgumentParser(description="TR / Python / Go speed comparison")
    ap.add_argument("tests", nargs="*", choices=TESTS + [MODEL], metavar="test",
                    help="tests to run (default: all): " + " ".join(TESTS + [MODEL]))
    ap.add_argument("--threads", type=int, default=2,
                    help=f"threads for {MODEL} (GOMAXPROCS in TR, torch.set_num_threads; default 2)")
    ap.add_argument("--repeat", type=int, default=3, help="runs per test; the best time counts (default 3)")
    ap.add_argument("--only", choices=["tr", "py", "go", "rs"], help="run only this language (rs: Rust, needs rustc)")
    ap.add_argument("--json", help="write the results to this file")
    ap.add_argument("--list", action="store_true", help="print the test names and exit")
    ap.add_argument("--check-model", action="store_true",
                    help=f"build the {MODEL} TR program without running it and exit (fails if the example changed)")
    return ap


def main():
    a = parser().parse_args()
    if a.list:
        print("\n".join(TESTS + [MODEL]))
        return
    if a.check_model:
        with tempfile.TemporaryDirectory() as tmp:
            model_program(tmp, 1)
        # Both sides train on example 36's BELGELER (py/char_model.py reads the same list); a listed document
        # that no longer exists stops the PyTorch side (2026-11-01: docs/spec/14_sinirlamalar.md).
        with open(os.path.join(ROOT, MODEL_TR), encoding="utf-8") as f:
            listed = re.findall(r'"(docs/[^"]+\.md)"', f.read().split("TR_const BELGELER = [", 1)[1].split("]", 1)[0])
        missing = [p for p in listed if not os.path.exists(os.path.join(ROOT, p))]
        if not listed or missing:
            raise SystemExit(f"{MODEL_TR}: BELGELER lists missing documents: {missing or 'none found'}")
        return
    if a.repeat < 1:
        parser().error("--repeat must be at least 1")
    if a.threads < 1:
        parser().error("--threads must be at least 1")
    tests = a.tests or TESTS + [MODEL]
    langs = [a.only] if a.only else ["tr", "py", "go", "rs"]
    results = {}
    with tempfile.TemporaryDirectory() as tmp:
        tr, go = build(tmp)
        rs = build_rust(tmp) if "rs" in langs else None
        if "rs" in langs and rs is None:
            if a.only == "rs":
                parser().error("--only rs needs rustc")
            print("rustc not found: the Rust column is left out", flush=True)
            langs.remove("rs")
        for t in tests:
            if t == MODEL:
                if a.only in ("go", "rs"):
                    continue
                results[t] = model(tr, tmp, a.threads, a.repeat, langs)
                cells = [f"{lang.upper():>2} {'-' if ms is None else ms:>7} ms/step" for lang, (ms, _) in results[t].items()]
                line = f"{t:16} " + " | ".join(cells) + f" | {a.threads} threads"
                if len(results[t]) == 2 and results[t]["tr"][0] and results[t]["py"][0]:
                    line += f" | TR/PyTorch {results[t]['tr'][0] / results[t]['py'][0]:5.2f}x"
                    line += " | loss " + " / ".join(str(v) for _, v in results[t].values())
                print(line, flush=True)
                continue
            commands = {
                "tr": [tr, os.path.join(BENCH, "tr", t + ".tr")],
                "py": [sys.executable, os.path.join(BENCH, "py", "compare.py"), t],
                "go": [go, t],
                "rs": [rs, t],
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
    # A program that failed leaves (None, its last error line); say so and fail the run, or a broken
    # comparison reads as a missing number (2026-11-01: four nulls went unnoticed).
    failed = [f"{t} {lang}: {err}" for t, r in results.items() for lang, (ms, err) in r.items() if ms is None]
    if failed:
        print("FAILED:\n  " + "\n  ".join(failed), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
