"""Baseline measurement set runner: every question, every side (TR and its references), every run in its own process.

Usage (from the repository root; prepare.sh makes the work folder):
  python -I tools/bench/baseline/run.py --work W --fetch                                  (the models, at their revisions)
  python -I tools/bench/baseline/run.py --work W --machine highram --out DIR --make-sounds (O-7, once, A-11)
  python -I tools/bench/baseline/run.py --work W --machine highram --out DIR --agreement  (O-4, Colab machines first)
  python -I tools/bench/baseline/run.py --work W --machine highram --out DIR [--question S1] [--skip-done]
  python -I tools/bench/baseline/run.py --rows --out DIR      (draft rows from metrics.json)
  python -I tools/bench/baseline/run.py --self-check NAME     (the checks behavior_baseline_test.go runs)
The questions (S1–S14, their commands and the machines' lists) are in questions.py.

The method is fixed before any run (tools/bench/baseline/README.md):
- order: one warm-up per side, then TR, E, E, TR, TR, E, E, TR, TR, E for each reference (ABBA);
- verdict: the i-th TR run over the i-th reference run gives five ratios; quartiles with
  statistics.quantiles(n=4, method="exclusive"); Q1 and Q3 on the same side of 1 decide, otherwise
  "not separated"; a quartile within 1 % of 1 is "borderline";
- threads: one number per machine, read from the TR process (the "<n> P" field of a gctrace line),
  given to each reference by name;
- memory: Linux VmHWM (counter, checked against ru_maxrss) and RssAnon (sampled every 10 ms);
  Windows PeakWorkingSetSize and PeakPagefileUsage (counters); GPU nvidia-smi every 1 s.
A disagreement, a failed run or (on the laptop) foreign load over 20 % stops with exit code 3.
"""
import argparse, json, os, re, shutil, statistics, subprocess, sys, tempfile, threading, time

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
BASE = os.path.join(ROOT, "tools", "bench", "baseline")
MACHINES = ["highram", "l4gpu", "l4cpu", "g4", "laptop"]  # L4's GPU and CPU parts run in one session (C-1)
ORDER = ["TR", "E", "E", "TR", "TR", "E", "E", "TR", "TR", "E"]
SAMPLE_S = 0.010
GPU_SAMPLE_S = 1.0
FOREIGN_LOAD_LIMIT = 20.0
BORDER = 0.01
STOP = 3


class Stop(Exception):
    """A reason to stop the question: disagreement, a failed run or foreign load (exit code 3)."""


# ---------------------------------------------------------------- method

def order():
    """The run order of one TR/reference pair: one warm-up per side, then ABBA."""
    return {"warmup": ["TR", "E"], "runs": list(ORDER)}


def verdict(ratios):
    """Verdict over the per-round ratios TR/reference of a measure where smaller is better."""
    q1, median, q3 = statistics.quantiles(ratios, n=4, method="exclusive")
    if q3 < 1:
        side = "tr_better"
    elif q1 > 1:
        side = "tr_worse"
    else:
        side = "not_separated"
    border = abs(q1 - 1) < BORDER or abs(q3 - 1) < BORDER
    return {"q1": q1, "median": median, "q3": q3, "verdict": side, "borderline": border}


# ---------------------------------------------------------------- parsers

def parse_proc_status(text):
    """VmHWM and RssAnon (kB) from a /proc/<pid>/status text; a missing field is None."""
    out = {"VmHWM": None, "RssAnon": None}
    for line in text.splitlines():
        name, _, rest = line.partition(":")
        if name in out:
            out[name] = int(rest.split()[0])
    return out


def parse_nvidia_smi(text):
    """Used memory (MiB) of each GPU from `nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits`."""
    return [int(line.strip()) for line in text.splitlines() if line.strip()]


GC_LINE = re.compile(r"^gc \d+ @.*?(\d+)->(\d+)->(\d+) MB.*?(\d+) P\b")


def parse_gctrace(text):
    """The P count (GOMAXPROCS) and the largest live heap (MB) from GODEBUG=gctrace=1 lines."""
    procs, live = None, None
    for line in text.splitlines():
        m = GC_LINE.match(line.strip())
        if m:
            procs = int(m.group(4))
            live = max(live or 0, int(m.group(3)))
    return {"procs": procs, "live_heap_mb": live}


def parse_result(stdout):
    """The JSON a program prints on its last `BASELINE ` line."""
    lines = [l for l in stdout.splitlines() if l.startswith("BASELINE ")]
    if not lines:
        raise Stop("no BASELINE line in the output")
    return json.loads(lines[-1][len("BASELINE "):])


# ---------------------------------------------------------------- one run

class Sampler:
    """Samples a running process's memory every 10 ms and the GPU memory every 1 s."""

    def __init__(self, proc):
        self.proc, self.done = proc, threading.Event()
        self.hwm_kb = self.anon_kb = self.ws_kb = self.pagefile_kb = self.gpu_mib = None
        self.threads = [threading.Thread(target=self._memory, daemon=True)]
        if shutil.which("nvidia-smi"):
            self.threads.append(threading.Thread(target=self._gpu, daemon=True))
        for t in self.threads:
            t.start()

    def _memory(self):
        while not self.done.is_set():
            if sys.platform.startswith("linux"):
                try:
                    with open(f"/proc/{self.proc.pid}/status", encoding="ascii") as f:
                        s = parse_proc_status(f.read())
                    self.hwm_kb = _max(self.hwm_kb, s["VmHWM"])
                    self.anon_kb = _max(self.anon_kb, s["RssAnon"])
                except (OSError, ValueError):
                    pass
            elif os.name == "nt":
                ws, pf = _windows_peaks(self.proc)
                self.ws_kb, self.pagefile_kb = _max(self.ws_kb, ws), _max(self.pagefile_kb, pf)
            time.sleep(SAMPLE_S)

    def _gpu(self):
        while not self.done.is_set():
            r = subprocess.run(["nvidia-smi", "--query-gpu=memory.used", "--format=csv,noheader,nounits"],
                               capture_output=True, text=True, stdin=subprocess.DEVNULL)
            if r.returncode == 0:
                self.gpu_mib = _max(self.gpu_mib, max(parse_nvidia_smi(r.stdout) or [0]))
            self.done.wait(GPU_SAMPLE_S)

    def stop(self):
        self.done.set()
        for t in self.threads:
            t.join()


def _max(a, b):
    return b if a is None else a if b is None else max(a, b)


def _windows_peaks(proc):
    """PeakWorkingSetSize and PeakPagefileUsage (kB) of a running process, or (None, None)."""
    import ctypes
    from ctypes import wintypes

    class Counters(ctypes.Structure):
        _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD)] + [
            (n, ctypes.c_size_t) for n in ("PeakWorkingSetSize", "WorkingSetSize", "QuotaPeakPagedPoolUsage",
                                           "QuotaPagedPoolUsage", "QuotaPeakNonPagedPoolUsage",
                                           "QuotaNonPagedPoolUsage", "PagefileUsage", "PeakPagefileUsage")]

    c = Counters()
    c.cb = ctypes.sizeof(c)
    handle = wintypes.HANDLE(int(proc._handle))
    if not ctypes.windll.psapi.GetProcessMemoryInfo(handle, ctypes.byref(c), c.cb):
        return None, None
    return c.PeakWorkingSetSize // 1024, c.PeakPagefileUsage // 1024


def run_once(cmd, env=None, timeout=3600):
    """Runs one command in its own process; returns wall time, output, exit code and memory."""
    with tempfile.TemporaryFile("w+", encoding="utf-8") as out, tempfile.TemporaryFile("w+", encoding="utf-8") as err:
        start = time.perf_counter()
        proc = subprocess.Popen(cmd, cwd=ROOT, env=env, stdout=out, stderr=err, stdin=subprocess.DEVNULL)
        sampler = Sampler(proc)
        maxrss_kb = None
        try:
            if hasattr(os, "wait4"):
                _, status, usage = os.wait4(proc.pid, 0)
                proc.returncode = os.waitstatus_to_exitcode(status)
                maxrss_kb = usage.ru_maxrss
            else:
                proc.wait(timeout=timeout)
        finally:
            wall_ms = (time.perf_counter() - start) * 1000
            sampler.stop()
        out.seek(0)
        err.seek(0)
        return {"cmd": cmd, "rc": proc.returncode, "wall_ms": wall_ms, "stdout": out.read(), "stderr": err.read(),
                "vmhwm_kb": sampler.hwm_kb, "ru_maxrss_kb": maxrss_kb, "rss_anon_sampled_kb": sampler.anon_kb,
                "peak_working_set_kb": sampler.ws_kb, "peak_pagefile_kb": sampler.pagefile_kb,
                "gpu_peak_mib": sampler.gpu_mib}


# ---------------------------------------------------------------- load and process list

def _cpu_times():
    """(busy, total) CPU time of the whole machine."""
    if sys.platform.startswith("linux"):
        with open("/proc/stat", encoding="ascii") as f:
            v = [int(x) for x in f.readline().split()[1:]]
        idle = v[3] + v[4]
        return sum(v) - idle, sum(v)
    import ctypes
    from ctypes import wintypes
    idle, kernel, user = wintypes.FILETIME(), wintypes.FILETIME(), wintypes.FILETIME()
    ctypes.windll.kernel32.GetSystemTimes(ctypes.byref(idle), ctypes.byref(kernel), ctypes.byref(user))
    t = lambda f: (f.dwHighDateTime << 32) | f.dwLowDateTime
    return t(kernel) + t(user) - t(idle), t(kernel) + t(user)


def foreign_load(seconds=1.0):
    """The busy share (%) of the machine over `seconds` while this runner starts nothing."""
    b0, t0 = _cpu_times()
    time.sleep(seconds)
    b1, t1 = _cpu_times()
    return 100.0 * (b1 - b0) / max(t1 - t0, 1)


def process_list():
    """The process list written to the log before and after each run."""
    cmd = ["tasklist"] if os.name == "nt" else ["ps", "-eo", "pid,pcpu,rss,comm", "--sort=-pcpu"]
    r = subprocess.run(cmd, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return r.stdout.splitlines()[:15]


# ---------------------------------------------------------------- a question

def _measures(value):
    """A side's measure: one number (ms) or a dict of named ms values."""
    return value if isinstance(value, dict) else {"ms": value}


def run_pair(question, tr, ref, agree, log, machine):
    """Warm-up and the ABBA runs of TR against one reference.

    tr and ref are dicts {"name", "cmd" or "cmds", "env", "measure", "warm"?, "parse"?}: one run of a side is its
    commands in order, each in its own process; measure(results, runs) gives ms (smaller is better), one number or a
    dict of named values, each with its own ratios and verdict. "warm" are other commands for the warm-up (on the
    laptop the agreement commands, O-4 N3). agree(tr_results, ref_results) on the warm-up returns None or the
    reason the outputs disagree; None as agree skips it (the agreement pass ran before, O-4)."""
    sides = {"TR": tr, "E": ref}
    first = {}
    for key in order()["warmup"]:
        first[key] = _run_side(question, sides[key], "warmup", log, machine, sides[key].get("warm"))
    if agree is not None:
        reason = agree(first["TR"]["results"], first["E"]["results"])
        if reason:
            raise Stop(f"{question}: {tr['name']} and {ref['name']} disagree: {reason}")
    times = {"TR": [], "E": []}
    memory = {"TR": [], "E": []}
    for i, key in enumerate(order()["runs"]):
        r = _run_side(question, sides[key], i, log, machine)
        times[key].append(_measures(sides[key]["measure"](r["results"], r["runs"])))
        memory[key].append(memory_of(r["runs"]))
    out = {"reference": ref["name"], "measures": {}, "memory": memory}
    for name in times["TR"][0]:
        tr_ms, ref_ms = [t[name] for t in times["TR"]], [t[name] for t in times["E"]]
        ratios = [a / b for a, b in zip(tr_ms, ref_ms)]
        out["measures"][name] = {"tr_ms": tr_ms, "ref_ms": ref_ms, "ratios": ratios, **verdict(ratios)}
    return out


def run_alone(question, side, log, machine, runs=5):
    """A question without a reference (S4, S8–S10, S14): one warm-up, then `runs` runs; median and range."""
    _run_side(question, side, "warmup", log, machine)
    values, memory = [], []
    for i in range(runs):
        r = _run_side(question, side, i, log, machine)
        values.append(_measures(side["measure"](r["results"], r["runs"])))
        memory.append(memory_of(r["runs"]))
    out = {"reference": None, "measures": {}, "memory": memory}
    for name in values[0]:
        v = [x[name] for x in values]
        out["measures"][name] = {"tr_ms": v, "median": statistics.median(v), "min": min(v), "max": max(v)}
    return out


def memory_of(runs):
    """The largest of each memory field over a side's processes (each process measured on its own, O-2)."""
    keys = ("vmhwm_kb", "ru_maxrss_kb", "rss_anon_sampled_kb", "peak_working_set_kb", "peak_pagefile_kb", "gpu_peak_mib")
    return {k: max((r[k] for r in runs if r.get(k) is not None), default=None) for k in keys}


def _run_side(question, side, index, log, machine, cmds=None):
    cmds = cmds or side.get("cmds") or [side["cmd"]]
    if machine == "laptop":
        load = foreign_load()
        log({"question": question, "side": side["name"], "index": index, "foreign_load": load})
        if load > FOREIGN_LOAD_LIMIT:
            raise Stop(f"{question}: foreign load {load:.0f} % before the run")
    runs, results = [], []
    for cmd in cmds:
        log({"question": question, "side": side["name"], "index": index, "processes_before": process_list()})
        run = run_once(cmd, side.get("env"))
        log({"question": question, "side": side["name"], "index": index, "processes_after": process_list(),
             **{k: v for k, v in run.items() if k not in ("stdout", "stderr")}})
        if run["rc"] != 0:
            raise Stop(f"{question}: {side['name']} failed (exit {run['rc']}): {run['stderr'].strip()[-400:]}")
        runs.append(run)
        results.append(side.get("parse", parse_result)(run["stdout"]))
    return {"runs": runs, "results": results}


REFERENCES = os.path.join(BASE, "references.py")
# The thread option of each reference, by its own name (O-1); references.py passes --threads on to each library
# by that library's name (thread_settings: torch.set_num_threads, faster-whisper cpu_threads, onnxruntime intra_op).
THREAD_FLAGS = {"llama-bench": "-t", "llama-completion": "-t", "whisper-cli": "-t", "lm": "--threads",
                "whisper_hf": "--threads", "faster_whisper": "--threads", "mms": "--threads", "piper": "--threads",
                "matmul": "--threads"}


def reference_cmd(name, threads, binary=None, argv=()):
    """The command of one reference with the machine's thread number; the only place the number is given."""
    t = [THREAD_FLAGS[name], str(threads)]
    if name == "llama-bench":
        return [binary, *argv, "-r", "1", "--no-warmup", *t, "-o", "json"]
    if name == "llama-completion":
        return [binary, *argv, "--temp", "0", *t]
    if name == "whisper-cli":
        return [binary, *argv, "-bs", "1", "-bo", "1", "-nf", *t]
    return [sys.executable, "-I", REFERENCES, name, *t, *argv]


def threads_from_tr(cmd, env=None):
    """The thread number of this machine: the P field of the TR process's gctrace (O-1), and its live heap."""
    e = dict(env or os.environ)
    e["GODEBUG"] = "gctrace=1"
    run = run_once(cmd, e)
    g = parse_gctrace(run["stderr"])
    if g["procs"] is None:
        raise Stop("no gctrace line: the thread number cannot be read from the TR process")
    return g


# ---------------------------------------------------------------- self checks (behavior_baseline_test.go)

def _self_check(name, arg):
    if name == "order":
        return order()
    if name == "verdict":
        return {k: verdict(v) for k, v in json.loads(arg).items()}
    if name == "memory":
        with open(arg, encoding="utf-8") as f:
            parts = json.load(f)
        return {"proc": parse_proc_status(parts["proc"]), "gpu": parse_nvidia_smi(parts["gpu"]),
                "gctrace": parse_gctrace(parts["gctrace"])}
    if name == "threads":
        with open(arg, encoding="utf-8") as f:
            n = parse_gctrace(f.read())["procs"]
        r = subprocess.run([sys.executable, "-I", REFERENCES, "threads", str(n)], capture_output=True, text=True,
                           stdin=subprocess.DEVNULL)
        return {"procs": n, "cpu_count": os.cpu_count(), "settings": json.loads(r.stdout),
                "commands": {name: reference_cmd(name, n, binary=name) for name in THREAD_FLAGS}}
    if name == "parsers":
        qs = load_questions()
        with open(arg, encoding="utf-8") as f:
            parts = json.load(f)
        return {"olcu": qs.parse_olcu(parts["olcu"]), "sse": qs.parse_sse(parts["sse"]),
                "llama_bench": qs.parse_llama_bench(parts["llama_bench"]), "ms_per_step": qs.ms_per_step(parts["rgn"]),
                "whisper_cpp_ms": qs.whisper_cpp_ms(parts["whisper_cpp"]), "wer": qs.wer(*parts["wer"])}
    if name == "items":
        qs = load_questions()
        out = {}
        for machine in MACHINES:
            ctx = qs.Ctx(sys.modules[__name__], machine, 97, arg, lambda rec: None, dry=True)
            out[machine] = {it["key"]: {"tr": it["tr"]["cmds"] if it["tr"] else None,
                                        "ref": it["ref"]["cmds"] if it["ref"] else None,
                                        "agree": bool(it["agree"]), "custom": bool(it["custom"])}
                            for q in qs.QUESTIONS[machine] for it in qs.items(ctx, q)}
        return out
    if name == "agreement":
        say = lambda text: [sys.executable, "-I", "-c", f"print('BASELINE ' + {json.dumps(json.dumps({'text': text}))})"]
        side = lambda n, t: {"name": n, "cmd": say(t), "measure": lambda res, runs: runs[0]["wall_ms"]}
        same = lambda a, b: None if a[0]["text"] == b[0]["text"] else f"{a[0]['text']!r} != {b[0]['text']!r}"
        run_pair("check", side("tr", "a"), side("ref", "b"), same, lambda rec: None, "check")
        return {"stopped": False}
    raise SystemExit(f"unknown self check {name!r}")


# ---------------------------------------------------------------- the questions (questions.py)

def load_questions():
    """questions.py by its path (python -I puts no script folder on sys.path)."""
    import importlib.util
    spec = importlib.util.spec_from_file_location("baseline_questions", os.path.join(BASE, "questions.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class Record:
    """The output folder: log.jsonl (every run, with the process lists), metrics.json and agreement.json."""

    def __init__(self, out):
        os.makedirs(out, exist_ok=True)
        self.out = out
        self.metrics = self._read("metrics.json")
        self.agreement = self._read("agreement.json")

    def _read(self, name):
        try:
            with open(os.path.join(self.out, name), encoding="utf-8") as f:
                return json.load(f)
        except OSError:
            return {}

    def log(self, rec):
        with open(os.path.join(self.out, "log.jsonl"), "a", encoding="utf-8", newline="") as f:
            f.write(json.dumps({"time": time.strftime("%Y-%m-%d %H:%M:%S %z"), **rec}, ensure_ascii=False) + "\n")

    def save(self):
        for name, value in (("metrics.json", self.metrics), ("agreement.json", self.agreement)):
            with open(os.path.join(self.out, name), "w", encoding="utf-8", newline="") as f:
                json.dump(value, f, indent=1, ensure_ascii=False)


def run_questions(a):
    qs = load_questions()
    rec = Record(a.out)
    with open(os.path.join(a.work, "tools.json"), encoding="utf-8") as f:
        trlang = json.load(f)["trlang"]
    threads = threads_from_tr([trlang, os.path.join(BASE, "matmul.tr"), "--size", "512", "--repeat", "2"])
    rec.metrics["_machine"] = {"machine": a.machine, "threads": threads["procs"], "threads_source": "gctrace P (O-1)",
                               "cpu_count": os.cpu_count(), "python": sys.version.split()[0], "platform": sys.platform}
    ctx = qs.Ctx(sys.modules[__name__], a.machine, threads["procs"], a.work, rec.log)
    if a.make_sounds:
        print(json.dumps(qs.make_sounds(ctx)))
        return 0
    order_ = a.question or qs.QUESTIONS[a.machine]
    if not qs.sounds_ok(ctx):  # O-7: without the fixed sounds S5 and S8 go last; the machine does not wait
        order_ = [q for q in order_ if q not in qs.SOUND_QUESTIONS] + [q for q in order_ if q in qs.SOUND_QUESTIONS]
    try:
        chosen = lambda it: not a.key or any(it["key"].startswith(k) for k in a.key)
        if a.agreement:
            agreement_pass(ctx, qs, order_, rec, chosen)
        else:
            for q in order_:
                for it in qs.items(ctx, q):
                    if not chosen(it) or (a.skip_done and it["key"] in rec.metrics):
                        continue
                    rec.metrics[it["key"]] = run_item(ctx, it, rec)
                    rec.save()
                    print(f"{it['key']}: done", flush=True)
    except Stop as e:
        rec.log({"stopped": str(e)})
        rec.save()
        print(f"STOPPED: {e}", flush=True)
        return STOP
    rec.save()
    return 0


def agreement_pass(ctx, qs, order_, rec, chosen=lambda it: True):
    """O-4: each item's agreement once, before any measure (Colab machines); GPU machines check the CUDA provider."""
    if ctx.gpu:
        r = run_once([sys.executable, "-I", "-c", "import onnxruntime, json; print(json.dumps(onnxruntime.get_available_providers()))"])
        if r["rc"] != 0 or "CUDAExecutionProvider" not in r["stdout"]:
            raise Stop(f"onnxruntime has no CUDAExecutionProvider: {r['stdout'].strip()} {r['stderr'][-200:]}")
    if ctx.machine == "highram":  # A-4: the English ids file against the voice's own mapping, once
        r = run_once(ctx.tr("piper.tr", "--model", ctx.model("piper-en-lessac-medium"), "--texts", qs.TEXTS, "--language",
                            "en", "--ids", qs.IDS_EN, "--check_ids"))
        if r["rc"] != 0 or not parse_result(r["stdout"])["ids_match"]:
            raise Stop(f"english_phoneme_ids.json does not match the voice's mapping: {r['stdout'][-400:]}")
    for q in order_:
        for it in qs.items(ctx, q):
            if not it["agree"] or not chosen(it):
                continue
            got = [_run_side(it["key"], s, "agreement", rec.log, ctx.machine, cmds)["results"]
                   for s, cmds in ((it["tr"], it["tr_agree"]), (it["ref"], it["ref_agree"]))]
            reason = it["agree"](*got)
            rec.agreement[it["key"]] = {"ok": reason is None, "reason": reason}
            rec.save()
            if reason:
                raise Stop(f"{it['key']}: {it['tr']['name']} and {it['ref']['name']} disagree: {reason}")
            print(f"{it['key']}: agree", flush=True)


def run_item(ctx, it, rec):
    """One item: a custom run (Dost's screen), TR alone, or the ABBA pair; on the laptop the warm-up agrees (N3)."""
    if it["custom"]:
        values, memory = [], []
        for i in range(it["runs"] + 1):  # the first is the warm-up
            v, m = it["custom"]()
            rec.log({"question": it["key"], "index": "warmup" if i == 0 else i - 1, "result": v, "memory": m})
            if i:
                values.append(v)
                memory.append(m)
        return {"reference": None, "memory": memory, "measures": {k: {"tr_ms": [v[k] for v in values],
                                                                        "median": statistics.median(v[k] for v in values)}
                                                                    for k in values[0]}}
    if it["ref"] is None:
        return run_alone(it["key"], it["tr"], rec.log, ctx.machine, it["runs"])
    tr, ref, agree = it["tr"], it["ref"], None
    if ctx.machine == "laptop" and it["agree"]:
        tr, ref, agree = dict(tr, warm=it["tr_agree"]), dict(ref, warm=it["ref_agree"]), it["agree"]
    elif it["agree"] and not rec.agreement.get(it["key"], {}).get("ok"):
        raise Stop(f"{it['key']}: no agreement yet; run the agreement pass first (--agreement, O-4)")
    return run_pair(it["key"], tr, ref, agree, rec.log, ctx.machine)


def fetch(work, only=None):
    """Downloads MODELS (or the `only` names) at their revisions into work/models; each large file's sha256 is the hub's."""
    qs = load_questions()
    unknown = set(only or ()) - set(qs.MODELS)
    if unknown:
        raise Stop(f"--models: unknown {sorted(unknown)}; the names are questions.py's MODELS")
    import urllib.request
    from huggingface_hub import HfApi, hf_hub_download
    api = HfApi()
    with open(qs.CATALOG, encoding="utf-8") as f:  # Dost's catalog: each file's sha256 (Dost checks the same numbers)
        known = {(e["source"]["repo"], e["source"]["revision"], x["path"]): x["sha256"]
                 for e in json.load(f)["entries"] for x in e["source"]["files"]}
    for name, (repo, rev, files) in qs.MODELS.items():
        if only and name not in only:
            continue
        folder = os.path.join(work, "models", name)
        hub = {} if name in qs.GITHUB or all((repo, rev, x) in known for x in files) else \
            {p.path: getattr(p, "lfs", None) for p in api.get_paths_info(repo, files, revision=rev)}
        for file in files:
            if name in qs.GITHUB:
                path = os.path.join(folder, file)
                os.makedirs(os.path.dirname(path), exist_ok=True)
                urllib.request.urlretrieve(f"https://raw.githubusercontent.com/{repo}/{rev}/{file}", path)
            else:
                path = hf_hub_download(repo, file, revision=rev, local_dir=folder)
            want = known.get((repo, rev, file)) or getattr(hub.get(file), "sha256", None)
            if want is not None and qs.sha256(path) != want:
                raise Stop(f"{name}/{file}: sha256 is not the {'catalog' if (repo, rev, file) in known else 'hub'}'s at {rev}")
            source = "catalog" if (repo, rev, file) in known else "hub" if want else None
            print(f"{name}/{file}: {os.path.getsize(path)} bytes" + (f" sha256 ok ({source})" if source else ""), flush=True)


def rows(out):
    """Draft rows from metrics.json: each measure's median, range, quartiles, verdict and the memory peaks (O-5)."""
    rec = Record(out)
    for key, r in rec.metrics.items():
        if key.startswith("_"):
            continue
        for name, m in r["measures"].items():
            line = f"{key} {name}: TR median {statistics.median(m['tr_ms']):.3f} range {min(m['tr_ms']):.3f}–{max(m['tr_ms']):.3f}"
            if r["reference"]:
                line += (f" | {r['reference']} median {statistics.median(m['ref_ms']):.3f} | ratio q1 {m['q1']:.3f} median "
                         f"{m['median']:.3f} q3 {m['q3']:.3f} {m['verdict']}" + (" borderline" if m["borderline"] else ""))
            print(line)
        mem = r["memory"]["TR"] if isinstance(r["memory"], dict) else r["memory"]
        peaks = {k: max((x.get(k) or 0 for x in mem), default=0) for k in ("vmhwm_kb", "rss_anon_sampled_kb", "gpu_peak_mib")}
        print(f"{key} memory TR peaks: {peaks}")
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--machine", choices=MACHINES)
    p.add_argument("--question", action="append", help="S1 … S14; repeat for several (default: the machine's list)")
    p.add_argument("--skip-done", action="store_true", help="skip questions already in the output folder")
    p.add_argument("--agreement", action="store_true", help="the agreement pass (O-4; Colab machines, before measuring)")
    p.add_argument("--work", help="prepare.sh's work folder: tools.json, models/, sounds/")
    p.add_argument("--out", default=os.environ.get("TR_RECORD_DIR"), help="output folder (default TR_RECORD_DIR)")
    p.add_argument("--rows", action="store_true", help="print draft rows from metrics.json in --out")
    p.add_argument("--fetch", action="store_true", help="download the models into --work (prepare.sh)")
    p.add_argument("--models", help="with --fetch: only these MODELS names, comma separated")
    p.add_argument("--key", action="append", help="only the items whose key starts with this (S1/qwen3-4b-q4km); repeat")
    p.add_argument("--make-sounds", action="store_true", help="make O-7's fixed sounds into --work/sounds (A-11)")
    p.add_argument("--self-check", nargs="+", metavar="NAME", help=argparse.SUPPRESS)
    a = p.parse_args()
    if a.self_check:
        try:
            print(json.dumps(_self_check(a.self_check[0], a.self_check[1] if len(a.self_check) > 1 else None)))
        except Stop as e:
            print(json.dumps({"stopped": True, "reason": str(e)}))
            return STOP
        return 0
    if a.rows:
        return rows(a.out)
    if not a.work:
        p.error("--work is prepare.sh's work folder")
    if a.fetch:
        try:
            fetch(a.work, a.models.split(",") if a.models else None)
        except Stop as e:
            print(f"STOPPED: {e}", flush=True)
            return STOP
        return 0
    if not a.machine or not a.out:
        p.error("--machine and --out (or TR_RECORD_DIR) are needed")
    return run_questions(a)


if __name__ == "__main__":
    sys.exit(main())
