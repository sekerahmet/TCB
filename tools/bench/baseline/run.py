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
A disagreement, a failed run or (on the laptop) foreign load over 20 % for 60 s (the runner itself not counted) stops with exit code 3.
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
    # A byte that is not UTF-8 becomes U+FFFD: a tool's stray byte shows in the text, it does not stop the runner.
    text = dict(mode="w+", encoding="utf-8", errors="replace")
    with tempfile.TemporaryFile(**text) as out, tempfile.TemporaryFile(**text) as err:
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


def _own_cpu():
    """CPU seconds of this runner and its finished children (its own process tree: not foreign load)."""
    t = os.times()
    return t.user + t.system + t.children_user + t.children_system


# Units of _cpu_times per CPU second: /proc/stat counts clock ticks, GetSystemTimes 100 ns.
_CPU_UNIT = (os.sysconf("SC_CLK_TCK") if hasattr(os, "sysconf") and sys.platform.startswith("linux") else 10_000_000)


def foreign_load(seconds=1.0):
    """The busy share (%) of the machine over `seconds` while this runner starts nothing, without the runner's own
    process tree (on the laptop it counted itself; tani 2026-11-20: 1.5–5.7 CPU s of python in each window)."""
    b0, t0 = _cpu_times()
    o0 = _own_cpu()
    time.sleep(seconds)
    b1, t1 = _cpu_times()
    own = (_own_cpu() - o0) * _CPU_UNIT
    return 100.0 * max(b1 - b0 - own, 0) / max(t1 - t0, 1)


def top_process():
    """The process with the most CPU over a short window (the reason when the machine stays busy), or None without psutil."""
    try:
        import psutil
    except ImportError:
        return None
    me = os.getpid()
    procs = [p for p in psutil.process_iter(["name"]) if p.pid not in (0, me)]
    for p in procs:
        try:
            p.cpu_percent(None)
        except psutil.Error:
            pass
    time.sleep(0.5)
    best = None
    for p in procs:
        try:
            c = p.cpu_percent(None)
        except psutil.Error:
            continue
        if best is None or c > best[1]:
            best = (p.info["name"], c)
    return f"{best[0]} {best[1]:.0f} %" if best else None


def wait_for_quiet(question, log, measure=foreign_load, limit=None, wait=60.0, clock=time.monotonic, top=top_process):
    """Before a laptop run: the foreign load under the limit, waiting at most `wait` seconds for it to fall (TCB
    2026-11-20: Windows Defender scans after each run); if it stays high the question stops with the busiest process."""
    limit = FOREIGN_LOAD_LIMIT if limit is None else limit
    start = clock()
    while True:
        load = measure()
        if load <= limit:
            return load
        if clock() - start >= wait:
            busiest = top()
            log({"question": question, "foreign_load": load, "waited_s": round(clock() - start, 1), "busiest": busiest})
            raise Stop(f"{question}: foreign load {load:.0f} % before the run, still after {wait:.0f} s (busiest: {busiest})")


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

    tr and ref are dicts {"name", "cmd" or "cmds", "env", "measure", "warm"?, "parse"?, "agree_parse"?}: one run
    of a side is its commands in order, each in its own process; measure(results, runs) gives ms (smaller is
    better), one number or a dict of named values, each with its own ratios and verdict. "warm" are other commands for the warm-up (on the
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


def run_alone(question, side, log, machine, runs=5, warm=True):
    """A question without a reference (S4, S8–S10, S14): one warm-up, then `runs` runs; median and range.
    A trial (--trial) is one run without the warm-up."""
    if warm:
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


MEMORY_KEYS = ("vmhwm_kb", "ru_maxrss_kb", "rss_anon_sampled_kb", "peak_working_set_kb", "peak_pagefile_kb", "gpu_peak_mib")


def memory_of(runs):
    """The largest of each memory field over a side's processes (each process measured on its own, O-2)."""
    return {k: max((r[k] for r in runs if r.get(k) is not None), default=None) for k in MEMORY_KEYS}


def _run_side(question, side, index, log, machine, cmds=None):
    # Agreement commands (cmds given) read their output with the side's "agree_parse" when it has one (S1:
    # llama-completion's text, not llama-bench's JSON).
    parse = side.get("agree_parse") if cmds and "agree_parse" in side else side.get("parse", parse_result)
    cmds = cmds or side.get("cmds") or [side["cmd"]]
    if machine == "laptop":
        load = wait_for_quiet(question, log)
        log({"question": question, "side": side["name"], "index": index, "foreign_load": load})
    runs, results = [], []
    for cmd in cmds:
        log({"question": question, "side": side["name"], "index": index, "processes_before": process_list()})
        run = run_once(cmd, side.get("env"))
        log({"question": question, "side": side["name"], "index": index, "processes_after": process_list(),
             **{k: v for k, v in run.items() if k not in ("stdout", "stderr")}})
        if run["rc"] != 0:
            raise Stop(f"{question}: {side['name']} failed (exit {run['rc']}): {run['stderr'].strip()[-400:]}")
        runs.append(run)
        results.append(parse(run["stdout"]))
        check_device(question, side["name"], cmd, run, results[-1], log, index)
    return {"runs": runs, "results": results}


GPU_BACKENDS = ("CUDA", "Vulkan", "Metal", "SYCL", "HIP", "MUSA", "OpenCL", "CANN")


def expected_device(cmd):
    """Ş2 (DENETİM 944; TCB 2026-11-19 20:39): the device a command asks for, from the command itself: --device's
    value; llama.cpp's -ngl (0: the processor); whisper-cli's -ng (the processor). None: the command names none."""
    name = os.path.basename(str(cmd[0])).lower() if cmd else ""
    if name.startswith("llama-"):
        return "cpu" if "-ngl" in cmd and str(cmd[cmd.index("-ngl") + 1]) == "0" else "cuda"
    if name.startswith("whisper-cli"):
        return "cpu" if "-ng" in cmd else "cuda"
    if "--device" in cmd:
        return "cuda" if str(cmd[cmd.index("--device") + 1]).startswith("cuda") else "cpu"
    return None


def actual_device(cmd, run, parsed):
    """The device a run really used, from its own output, and where it was read: llama-bench's JSON (backends,
    n_gpu_layers), llama-completion's log (layers offloaded to the GPU), whisper-cli's log (use gpu, the loaded
    backends), else the result's actual_device (the references: the model's own device; TR's programs: a tensor's own
    device) or a reference's device. A TR program's device is its command's echo, so trlang's result must name
    actual_device (DENETİM 948 N1)."""
    name = os.path.basename(str(cmd[0])).lower()
    if name.startswith("llama-bench"):
        row = json.loads(run["stdout"][run["stdout"].index("["):])[0]
        gpu = (row.get("n_gpu_layers") or 0) > 0 and any(b in str(row.get("backends")) for b in GPU_BACKENDS)
        return ("cuda" if gpu else "cpu"), f"llama-bench backends {row.get('backends')}, n_gpu_layers {row.get('n_gpu_layers')}"
    if name.startswith("llama-completion"):
        m = re.search(r"offloaded (\d+)/(\d+) layers to GPU", run["stderr"])
        sysinfo = re.search(r"system_info:[^\n]*", run["stderr"])
        backends = re.findall(r"\| (\w+) :", sysinfo.group(0)) if sysinfo else []
        if not m and not sysinfo:
            return None, "llama-completion printed no system_info"
        gpu = bool(m and int(m.group(1)) > 0)
        return ("cuda" if gpu else "cpu"), f"llama-completion {m.group(0) if m else 'no layer offloaded'}, backends {backends}"
    if name.startswith("whisper-cli"):
        m = re.search(r"use gpu\s*=\s*(\d)", run["stderr"])
        loaded = re.findall(r"load_backend: loaded (\w+) backend", run["stderr"])
        if not m:
            return None, "whisper-cli printed no 'use gpu'"
        gpu = m.group(1) == "1" and any(b != "CPU" for b in loaded)
        return ("cuda" if gpu else "cpu"), f"whisper-cli use gpu = {m.group(1)}, loaded {loaded}"
    if not isinstance(parsed, dict):
        return None, "the result names no device"
    d = parsed.get("actual_device") if name.startswith("trlang") else (parsed.get("actual_device") or parsed.get("device"))
    if not d:
        return None, "TR's result names no actual_device" if name.startswith("trlang") else "the result names no device"
    d = str(d).lower()
    return ("cuda" if d.startswith(("cuda", "gpu")) else "cpu" if d.startswith("cpu") else d), \
        f"result {'actual_device' if parsed.get('actual_device') else 'device'} {d}"


def check_device(question, side, cmd, run, parsed, log, index):
    """Ş2: every run's real device is read from its own output and logged; a run that did not use the device its
    command asks for, or does not say, stops the item: a row whose devices are not checked is not written."""
    want = expected_device(cmd)
    if want is None:
        return
    got, how = actual_device(cmd, run, parsed)
    log({"question": question, "side": side, "index": index, "device": got, "device_from": how})
    if got != want:
        raise Stop(f"{question}: {side}: device mismatch (aygıt uyuşmadı): ran on {got} ({how}), the command asks {want}")


REFERENCES = os.path.join(BASE, "references.py")
# The thread option of each reference, by its own name (O-1); references.py passes --threads on to each library
# by that library's name (thread_settings: torch.set_num_threads, faster-whisper cpu_threads, onnxruntime intra_op).
THREAD_FLAGS = {"llama-bench": "-t", "llama-completion": "-t", "whisper-cli": "-t", "lm": "--threads",
                "whisper_hf": "--threads", "faster_whisper": "--threads", "mms": "--threads", "piper": "--threads",
                "matmul": "--threads"}


def reference_cmd(name, threads, binary=None, argv=()):
    """The command of one reference with the machine's thread number; the only place the number is given."""
    # 0 is the library's own default (Ş1's scan): onnxruntime and faster-whisper take 0 themselves; llama.cpp and
    # whisper-cli get no -t at all.
    t = [] if threads == 0 and name in ("llama-bench", "llama-completion", "whisper-cli") else [THREAD_FLAGS[name], str(threads)]
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
                                        "agree": bool(it["agree"]), "custom": bool(it["custom"]),
                                        "check": it["check"]["name"] if it["check"] else None}
                            for q in qs.QUESTIONS[machine] for it in qs.items(ctx, q)}
        return out
    if name == "agreement":
        say = lambda text: [sys.executable, "-I", "-c", f"print('BASELINE ' + {json.dumps(json.dumps({'text': text}))})"]
        side = lambda n, t: {"name": n, "cmd": say(t), "measure": lambda res, runs: runs[0]["wall_ms"]}
        same = lambda a, b: None if a[0]["text"] == b[0]["text"] else f"{a[0]['text']!r} != {b[0]['text']!r}"
        # The reference's agreement command prints plain text, read with its agree_parse (S1's llama-completion).
        plain = [sys.executable, "-I", "-c", "print('b')"]
        ref = dict(side("ref", "b"), warm=[plain], agree_parse=lambda out: {"text": out.strip()})
        run_pair("check", side("tr", "a"), ref, same, lambda rec: None, "check")
        return {"stopped": False}
    if name == "base":  # --base-trlang: the base items of S5, S9 and S14, and a pair of one program against itself
        qs = load_questions()
        ctx = qs.Ctx(sys.modules[__name__], "highram", 97, arg, lambda rec: None, dry=True)
        # S9 is Dost's custom run (no base side); the count before the keyed dict shows S5's shared TR side gives one
        # base item per language, not one per reference (DENETİM 943).
        its = base_items(qs.items(ctx, "S5") + qs.items(ctx, "S9") + qs.items(ctx, "S14"), ctx.tools["trlang"], "BASE")
        nap = [sys.executable, "-I", "-c", "import time; time.sleep(0.2); print('BASELINE {}')"]
        side = {"name": "TR", "cmds": [nap], "measure": lambda res, runs: runs[0]["wall_ms"]}
        same = base_items([{"key": "S0/x/ref", "tr": side, "ref": side, "custom": None, "runs": 5}], sys.executable,
                          sys.executable)[0]
        pair = run_pair(same["key"], same["tr"], same["ref"], None, lambda rec: None, "check")
        return {"count": len(its),
                "items": {it["key"]: {"tr": it["tr"]["cmds"], "ref": it["ref"]["cmds"], "agree": bool(it["agree"])}
                          for it in its},
                "pair": {"key": same["key"], "reference": pair["reference"], **pair["measures"]["ms"]}}
    if name == "mms_ids":
        return _check_mms_ids(arg)
    if name == "forced":  # S1, S3's agreement verdict (B14) on language_model.tr --mode forced results in a JSON list
        with open(arg, encoding="utf-8") as f:
            return [load_questions().forced_verdict(r) for r in json.load(f)]
    if name == "agree_labels":  # S5's agreement functions log under their own item's key (A-11: all said "en")
        logged, qs = [], load_questions()
        ctx = qs.Ctx(sys.modules[__name__], "highram", 97, arg, logged.append, dry=True)

        class Seen(dict):  # every other float32 reference "has run" and said "a"
            __contains__ = lambda self, k: True
            __getitem__ = lambda self, k: "a"
        ctx.seen_texts = Seen()
        out = {}
        for it in qs.items(ctx, "S5"):
            it["agree"]([{"text": "a"}], [{"text": "a"}])
            out[it["key"]] = logged[-1]["agreement"]
        return out
    if name == "s5":  # S5's verdict (B15) on cases [{"tr", "texts", "source"}] in a JSON list
        with open(arg, encoding="utf-8") as f:
            return [load_questions().s5_verdict(c["tr"], c["texts"], c["source"]) for c in json.load(f)]
    if name == "epsilon":  # a GPU machine's ε from an agreement.json (B14): S1/a measured, S1/b taken from elsewhere
        import types
        ctx = types.SimpleNamespace(epsilon_from=arg)
        return {k: load_questions().highram_epsilon(ctx, k) for k in ("S1/a", "S1/b", "S1/c")}
    raise SystemExit(f"unknown self check {name!r}")


class _PadLetterTokenizer:
    """VitsTokenizer's behaviour that B13 found, without transformers: the pad token is the letter "k" (id 0, the blank)
    and stays a special token, so its own blanks are dropped (tokenize("week") has 8 tokens)."""

    def __init__(self):
        self.add_blank, self.ids = True, {"k": 0, "w": 9, "e": 7, " ": 19, "a": 26}

    def tokenize(self, text):
        out = []
        for part in re.split("(k)", text):
            if part == "k":
                out.append("k")
            elif part:
                chars = [c for c in part if c in self.ids]
                out += sum((["k", c] for c in chars), []) + ["k"] if self.add_blank else chars
        return out

    def convert_tokens_to_ids(self, tokens):
        return [self.ids[t] for t in tokens]


def _check_mms_ids(texts_path):
    """references.py's mms_ids and sentences: the blanks as VITS puts them, the paragraph split as mms.tr splits it."""
    import importlib.util
    import types
    spec = importlib.util.spec_from_file_location("baseline_references", REFERENCES)
    ref = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(ref)
    tok = _PadLetterTokenizer()
    out = {"tokenizer": {t: tok.convert_tokens_to_ids(tok.tokenize(t)) for t in ("week", "kew")},
           "mms_ids": {t: ref.mms_ids(tok, t) for t in ("week", "kew", "a wa")}, "add_blank_kept": tok.add_blank}
    a = types.SimpleNamespace(texts=texts_path, set="speech_to_text", language="en")
    out["sentences"] = ref.sentences(a)
    # run_once reads a tool's stray byte as U+FFFD instead of stopping (the laptop's cp1254 output, A-11).
    stray = "import sys; sys.stdout.buffer.write(bytes([97, 252, 98]))"
    out["stray_byte"] = run_once([sys.executable, "-I", "-c", stray])["stdout"]
    return out


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
    doc = None
    if a.settings:  # Ş1: the window's scan's choice (en-hizli.json); it goes with the run's record
        with open(a.settings, encoding="utf-8") as f:
            doc = json.load(f)
        rec.log({"settings": {k: v for k, v in doc.items() if not k.startswith("_")}, "settings_file": a.settings,
                 "scan": doc.get("_scan"), "scan_commit": doc.get("_commit")})
        reason = None if (a.trial or a.agreement or a.scan) else check_settings(doc, head_commit())
        if reason:  # a measure, not a trial: only a whole scan of this commit (DENETİM 947 and 948 Ş1)
            rec.log({"stopped": reason})
            rec.save()
            print(f"STOPPED: {reason}", flush=True)
            return STOP
    with open(os.path.join(a.work, "tools.json"), encoding="utf-8") as f:
        trlang = json.load(f)["trlang"]
    threads = threads_from_tr([trlang, os.path.join(BASE, "matmul.tr"), "--size", "512", "--repeat", "2"])
    rec.metrics["_machine"] = {"machine": a.machine, "threads": threads["procs"], "threads_source": "gctrace P (O-1)",
                               "cpu_count": os.cpu_count(), "python": sys.version.split()[0], "platform": sys.platform}
    ctx = qs.Ctx(sys.modules[__name__], a.machine, threads["procs"], a.work, rec.log)
    ctx.epsilon_from = a.epsilon_from  # B14: a GPU machine takes S1 and S3's ε from HIGHRAM's agreement.json
    if doc is not None:
        ctx.settings = {k: v for k, v in doc.items() if not k.startswith("_")}
    if a.scan:
        try:
            return scan_settings(ctx, qs, rec, a.scan, a.out, runs=1 if a.trial else 3)
        except Stop as e:
            rec.log({"stopped": str(e)})
            rec.save()
            print(f"STOPPED: {e}", flush=True)
            return STOP
    for kv in a.epsilon or []:  # BC17: ε for weights without TR's quantized path, written in the run script
        key, _, value = kv.partition("=")
        try:
            ctx.epsilon_given[key] = float(value)
        except ValueError:
            raise SystemExit(f"--epsilon wants KEY=NUMBER, not {kv!r}. Example: --epsilon S1/qwen3-4b-q8=2.3770726")
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
                its = qs.items(ctx, q)
                if a.base_trlang:
                    its = base_items(its, ctx.tools["trlang"], a.base_trlang)
                for it in its:
                    if not chosen(it) or (a.skip_done and it["key"] in rec.metrics):
                        continue
                    if a.trial and it["ref"] is not None:
                        trial_pair(ctx, it, rec)
                        continue
                    rec.metrics[it["key"]] = run_item(ctx, it, rec, trial=a.trial)
                    rec.save()
                    print(f"{it['key']}: done", flush=True)
    except Stop as e:
        rec.log({"stopped": str(e)})
        rec.save()
        print(f"STOPPED: {e}", flush=True)
        return STOP
    rec.save()
    return 0


# Ş1 (DENETİM 944 and 947, TCB 2026-11-22): at the window's start each reference's thread number is chosen by a short
# scan: the library's default (0), 6 and the machine's logical processors. S1's llama-bench reading (-p 512 -n 0) and
# generation (-p 0 -n 128 -d 512) are separate groups, each with its own setting (N1: on a hybrid processor the two may
# differ); S3's llama-bench reads with the reading group's threads and also tries flash attention 0 and 1, at depth 700
# only. Each setting runs its group's item once to warm, then SCAN_RUNS times; the setting with the shortest median of the
# measures (summed) is chosen, and the table keeps each measure's median too. The rule is written before the run; the
# choice and the table go to en-hizli.json and every run's record. A measure takes the file only whole (check_settings).
SCAN = [("llama.cpp read", "S1", "S1/qwen3-4b-q4km", slice(0, 1)), ("llama.cpp gen", "S1", "S1/qwen3-4b-q4km", slice(1, 2)),
        ("llama.cpp S3", "S3", "S3/qwen3-4b-q4km", slice(0, 1)),
        ("faster-whisper float32", "S5", "S5/tr/faster-whisper-float32", None),
        ("faster-whisper int8", "S5", "S5/tr/faster-whisper-int8", None), ("whisper.cpp", "S5", "S5/tr/whisper.cpp", None),
        ("onnxruntime", "S7", "S7/tr", None)]
SCAN_GROUPS = [g for g, _, _, _ in SCAN]
SCAN_RUNS = 3


def head_commit():
    """The commit the code runs from (git rev-parse HEAD), or None outside a git checkout."""
    r = subprocess.run(["git", "-C", BASE, "rev-parse", "HEAD"], capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return r.stdout.strip() if r.returncode == 0 else None


def check_settings(doc, commit):
    """Ş1 (DENETİM 947 and 948): a measure takes en-hizli.json only from a whole scan of its own commit: every group
    chosen and timed with SCAN_RUNS runs (a trial's file has one run per setting; a scan cut short leaves groups out),
    scanned at the commit the measure runs (a file left by an earlier window carries its own). The reason, or None."""
    table = doc.get("_scan", {})
    missing = [g for g in SCAN_GROUPS if g not in doc]
    short = [g for g in SCAN_GROUPS if g in doc and table.get(g, {}).get("runs", 0) < SCAN_RUNS]
    if missing or short:
        return f"en-hizli.json is not a whole scan: groups missing {missing}, fewer than {SCAN_RUNS} runs {short}"
    if commit is None or doc.get("_commit") != commit:
        return f"en-hizli.json was scanned at commit {doc.get('_commit')}, this measure runs {commit}: scan again"
    return None


def time_side(ctx, side, key, log, runs, measure, warm=True):
    """One setting: the median of the summed measures and each measure's median, after one warm-up run."""
    if warm:
        _run_side(key, side, "scan warmup", log, ctx.machine)
    got = []
    for i in range(runs):
        r = _run_side(key, side, f"scan {i}", log, ctx.machine)
        got.append(measure(r["results"], r["runs"]))
    return statistics.median(sum(g.values()) for g in got), {k: statistics.median(g[k] for g in got) for k in got[0]}


def scan_settings(ctx, qs, rec, groups, out, runs=SCAN_RUNS):
    """Ş1's scan of the named groups; merges into out/en-hizli.json, so a trial can run the groups as separate commands."""
    path = os.path.join(out, "en-hizli.json")
    try:
        with open(path, encoding="utf-8") as f:
            res = json.load(f)
    except OSError:
        res = {}
    table = res.setdefault("_scan", {})
    for group, q, key, part in SCAN:
        if group not in groups:
            continue
        fa = group == "llama.cpp S3"
        values = [0, 1] if fa else [0, 6, os.cpu_count()]
        meds, each = {}, {}
        for v in values:
            ctx.settings = {k: x for k, x in res.items() if not k.startswith("_")}
            ctx.settings[group] = {"flash_attn": v} if fa else {"threads": v}
            it = next(i for i in qs.items(ctx, q) if i["key"] == key)
            side = dict(it["ref"], cmds=it["ref"]["cmds"][part]) if part else it["ref"]
            # One of S1's two llama-bench commands is timed by itself (S1's measure names both commands' results).
            measure = (lambda r, rs: {"ms": r[0]["avg_ns"] / 1e6}) if part and not fa else side["measure"]
            meds[v], each[v] = time_side(ctx, side, key, rec.log, runs, measure, warm=runs > 1)
            print(f"{group} {'flash_attn' if fa else 'threads'} {v}: median {meds[v]:.1f} ms {each[v]}", flush=True)
        best = min(meds, key=meds.get)
        res[group] = {"flash_attn": best} if fa else {"threads": best}
        table[group] = {"medians_ms": {str(k): v for k, v in meds.items()},
                        "measures_ms": {str(k): v for k, v in each.items()}, "runs": runs, "chosen": best}
        rec.log({"scan": group, **table[group]})
        res["_commit"] = head_commit()  # DENETİM 948 Ş1: a measure takes only its own commit's scan
        with open(path, "w", encoding="utf-8", newline="\n") as f:
            json.dump(res, f, indent=1)
    print(json.dumps({k: v for k, v in res.items() if not k.startswith("_")}), flush=True)
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
        if r["rc"] != 0:  # A-11's fifth test: the English voice was not prepared and the stop said nothing
            raise Stop(f"piper.tr --check_ids failed (exit {r['rc']}); is piper-en-lessac-medium prepared? "
                       f"{r['stderr'].strip()[-400:]}")
        if not parse_result(r["stdout"])["ids_match"]:
            raise Stop(f"english_phoneme_ids.json does not match the voice's mapping: {r['stdout'][-400:]}")
    for q in order_:
        for it in qs.items(ctx, q):
            if it["agree"] and chosen(it):
                agree_item(ctx, it, rec)


def trial_pair(ctx, it, rec):
    """--trial of a pair: its agreement pass (--agreement) tries it when it has an agreement; a pair without one (S13's
    char model) runs each side once here, or a trial would run nothing and still exit 0 (2026-11-22)."""
    if it["agree"]:
        return
    for s in (it["tr"], it["ref"]):
        _run_side(it["key"], s, "trial", rec.log, ctx.machine)
    print(f"{it['key']}: done", flush=True)


def agree_item(ctx, it, rec):
    """One item's agreement: TR against its reference, or against its check when it has no speed reference (S2, S6)."""
    other = it["ref"] or it["check"]
    got = [_run_side(it["key"], s, "agreement", rec.log, ctx.machine, cmds)["results"]
           for s, cmds in ((it["tr"], it["tr_agree"]), (other, it["ref_agree"]))]
    reason = it["agree"](*got)
    # The agreement's own record (S1, S3: ε, agreeing positions, ties) goes with the verdict (B14).
    rec.agreement[it["key"]] = {"ok": reason is None, "reason": reason, **ctx.extra.pop(it["key"], {})}
    rec.save()
    if reason:
        raise Stop(f"{it['key']}: {it['tr']['name']} and {other['name']} disagree: {reason}")
    print(f"{it['key']}: agree", flush=True)


def base_items(items, trlang, base):
    """--base-trlang: each item's TR side against the same TR program run by the base build (main); the warm-up,
    the ABBA order and the load checks stay, the ratio is branch/base. Items sharing a TR side (S5's three
    references) give one base item, keyed by the TR side's key with "base" as its last part (S5/tr/base); a TR-only
    item gets "/base" after its key. Dost's custom runs have no base side."""
    out, seen = [], set()
    for it in items:
        if it["custom"] or it["tr"] is None or json.dumps(it["tr"]["cmds"]) in seen:
            continue
        seen.add(json.dumps(it["tr"]["cmds"]))
        key = (it["key"].rsplit("/", 1)[0] if it["ref"] is not None else it["key"]) + "/base"
        cmds = [[base, *c[1:]] if c and c[0] == trlang else c for c in it["tr"]["cmds"]]
        ref = dict(it["tr"], name="TR base", cmds=cmds)
        out.append({"key": key, "tr": it["tr"], "ref": ref, "agree": None, "tr_agree": None, "ref_agree": None,
                    "runs": it["runs"], "custom": None, "check": None})
    return out


def run_item(ctx, it, rec, trial=False):
    """One item: a custom run (Dost's screen), TR alone, or the ABBA pair; on the laptop the warm-up agrees (N3).
    trial (--trial): an item without a reference runs once, without the warm-up: the script, the models and the device are
    tried, the number is not used (the laptop's scripts; a trial of an item with a reference is the agreement pass)."""
    if it["custom"]:
        values, memory = [], []
        warm = 0 if trial else 1
        for i in range(1 if trial else it["runs"] + 1):  # the first is the warm-up, but not in a trial
            v, m = it["custom"]()
            rec.log({"question": it["key"], "index": "warmup" if i < warm else i - warm, "result": v, "memory": m})
            if i >= warm:
                values.append(v)
                memory.append(m)
        return {"reference": None, "memory": memory, "measures": {k: {"tr_ms": [v[k] for v in values],
                                                                        "median": statistics.median(v[k] for v in values)}
                                                                    for k in values[0]}}
    agreed = lambda: rec.agreement.get(it["key"], {}).get("ok")
    if it["ref"] is None:
        if it.get("agree") and not agreed():  # a check without a speed reference (S2, S6): agree first; the laptop does it here
            if ctx.machine != "laptop":
                raise Stop(f"{it['key']}: no agreement yet; run the agreement pass first (--agreement, O-4)")
            agree_item(ctx, it, rec)
        return run_alone(it["key"], it["tr"], rec.log, ctx.machine, 1 if trial else it["runs"], warm=not trial)
    tr, ref, agree = it["tr"], it["ref"], None
    if ctx.machine == "laptop" and it["agree"]:
        tr, ref, agree = dict(tr, warm=it["tr_agree"]), dict(ref, warm=it["ref_agree"]), it["agree"]
    elif it["agree"] and not agreed():
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
    """Draft rows from metrics.json: each measure's median, range, quartiles, verdict and the memory peaks (O-5); first
    the row's setting (N2, DENETİM 947): the machine's thread number and the scan's choice the run took (en-hizli.json)."""
    rec = Record(out)
    m = rec.metrics.get("_machine", {})
    setting = f"setting: machine threads {m.get('threads')} ({m.get('threads_source')})"
    try:
        with open(os.path.join(out, "en-hizli.json"), encoding="utf-8") as f:
            setting += f"; fastest settings {json.dumps({k: v for k, v in json.load(f).items() if not k.startswith('_')})}"
    except OSError:
        setting += "; no en-hizli.json (each reference with the machine's number)"
    print(setting)
    for key, r in rec.metrics.items():
        if key.startswith("_"):
            continue
        for name, m in r["measures"].items():
            line = f"{key} {name}: TR median {statistics.median(m['tr_ms']):.3f} range {min(m['tr_ms']):.3f}–{max(m['tr_ms']):.3f}"
            if r["reference"]:  # the reference's range too: a row writes both sides' median and range (TCB 22:45)
                line += (f" | {r['reference']} median {statistics.median(m['ref_ms']):.3f} range {min(m['ref_ms']):.3f}–"
                         f"{max(m['ref_ms']):.3f} | ratio q1 {m['q1']:.3f} median "
                         f"{m['median']:.3f} q3 {m['q3']:.3f} {m['verdict']}" + (" borderline" if m["borderline"] else ""))
            print(line)
        # Every field memory_of keeps that some run measured: Linux's VmHWM and RssAnon, Windows' peak working set
        # and pagefile (the laptop, E), the GPU's. The reference's peaks too, so a row compares the two sides' memory;
        # a side whose memory the record lacks says so instead of printing nothing (DENETİM 949 N2).
        sides = [("TR", r["memory"]["TR"] if isinstance(r["memory"], dict) else r["memory"])]
        if r.get("reference"):
            sides.append((r["reference"], r["memory"].get("E") if isinstance(r["memory"], dict) else None))
        for side, mem in sides:
            peaks = {k: max(x[k] for x in mem if x.get(k) is not None) for k in MEMORY_KEYS
                     if any(x.get(k) is not None for x in mem)} if mem else {}
            print(f"{key} memory {side} peaks: {peaks if peaks else '— (kayıtta yok)'}")
    return 0


def main():
    # The laptop's Python writes a redirected stdout in the locale's code page (cp1254); the texts are UTF-8.
    for stream in (sys.stdout, sys.stderr):
        stream.reconfigure(encoding="utf-8", errors="replace")
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--machine", choices=MACHINES)
    p.add_argument("--question", action="append", help="S1 … S14; repeat for several (default: the machine's list)")
    p.add_argument("--skip-done", action="store_true", help="skip questions already in the output folder")
    p.add_argument("--agreement", action="store_true", help="the agreement pass (O-4; Colab machines, before measuring)")
    p.add_argument("--trial", action="store_true", help="a trial: items without a reference run once without the warm-up "
                                                       "(the number is not used; Ahmet 2026-11-20)")
    p.add_argument("--work", help="prepare.sh's work folder: tools.json, models/, sounds/")
    p.add_argument("--out", default=os.environ.get("TR_RECORD_DIR"), help="output folder (default TR_RECORD_DIR)")
    p.add_argument("--rows", action="store_true", help="print draft rows from metrics.json in --out")
    p.add_argument("--fetch", action="store_true", help="download the models into --work (prepare.sh)")
    p.add_argument("--models", help="with --fetch: only these MODELS names, comma separated")
    p.add_argument("--key", action="append", help="only the items whose key starts with this (S1/qwen3-4b-q4km); repeat")
    p.add_argument("--epsilon-from", help="GPU machines: HIGHRAM's agreement.json, whose S1 and S3 ε they use (B14)")
    p.add_argument("--settings", help="the window's scan's choice (en-hizli.json): each reference's fastest setting (Ş1)")
    p.add_argument("--scan", action="append", choices=SCAN_GROUPS,
                   help="the window's short scan of a reference's fastest setting, into --out/en-hizli.json (Ş1)")
    p.add_argument("--epsilon", action="append", metavar="KEY=NUMBER",
                   help="ε of an item whose weights have no TR quantized path (Q8_0), given before the run (BC17)")
    p.add_argument("--make-sounds", action="store_true", help="make O-7's fixed sounds into --work/sounds (A-11)")
    p.add_argument("--base-trlang", help="a base trlang (main's build): each question's TR side against the same "
                                          "program run by it, ratio branch/base, keys <question>/base")
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
