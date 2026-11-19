"""Baseline measurement set runner: every question, every side (TR and its references), every run in its own process.

Usage (from the repository root):
  python -I tools/bench/baseline/run.py --machine highram --question S1 [--skip-done] [--out DIR]
  python -I tools/bench/baseline/run.py --rows --out DIR      (draft rows from metrics.json)
  python -I tools/bench/baseline/run.py --self-check NAME     (the checks behavior_baseline_test.go runs)

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
MACHINES = ["highram", "l4", "g4", "laptop"]
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

def run_pair(question, tr, ref, agree, log, machine):
    """Warm-up (the agreement run) and the ABBA runs of TR against one reference.

    tr and ref are dicts {"name", "cmd", "env", "measure"}: measure(result_json, run) -> ms (smaller is better).
    agree(tr_json, ref_json) returns None or the reason the outputs disagree."""
    sides = {"TR": tr, "E": ref}
    first = {}
    for key in order()["warmup"]:
        first[key] = _run_side(question, sides[key], "warmup", log, machine)
    reason = agree(first["TR"]["result"], first["E"]["result"])
    if reason:
        raise Stop(f"{question}: {tr['name']} and {ref['name']} disagree: {reason}")
    times = {"TR": [], "E": []}
    for i, key in enumerate(order()["runs"]):
        r = _run_side(question, sides[key], i, log, machine)
        times[key].append(sides[key]["measure"](r["result"], r["run"]))
    ratios = [a / b for a, b in zip(times["TR"], times["E"])]
    return {"reference": ref["name"], "tr_ms": times["TR"], "ref_ms": times["E"], "ratios": ratios,
            **verdict(ratios)}


def _run_side(question, side, index, log, machine):
    if machine == "laptop":
        load = foreign_load()
        log({"question": question, "side": side["name"], "index": index, "foreign_load": load})
        if load > FOREIGN_LOAD_LIMIT:
            raise Stop(f"{question}: foreign load {load:.0f} % before the run")
    log({"question": question, "side": side["name"], "index": index, "processes_before": process_list()})
    run = run_once(side["cmd"], side.get("env"))
    log({"question": question, "side": side["name"], "index": index, "processes_after": process_list(),
         **{k: v for k, v in run.items() if k not in ("stdout", "stderr")}})
    if run["rc"] != 0:
        raise Stop(f"{question}: {side['name']} failed (exit {run['rc']}): {run['stderr'].strip()[-400:]}")
    return {"run": run, "result": parse_result(run["stdout"])}


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
    if name == "agreement":
        say = lambda text: [sys.executable, "-I", "-c", f"print('BASELINE ' + {json.dumps(json.dumps({'text': text}))})"]
        side = lambda n, t: {"name": n, "cmd": say(t), "measure": lambda res, run: run["wall_ms"]}
        same = lambda a, b: None if a["text"] == b["text"] else f"{a['text']!r} != {b['text']!r}"
        run_pair("check", side("tr", "a"), side("ref", "b"), same, lambda rec: None, "check")
        return {"stopped": False}
    raise SystemExit(f"unknown self check {name!r}")


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--machine", choices=MACHINES)
    p.add_argument("--question", action="append", help="S1 … S14; repeat for several (default: the machine's list)")
    p.add_argument("--skip-done", action="store_true", help="skip questions already in the output folder")
    p.add_argument("--out", default=os.environ.get("TR_RECORD_DIR"), help="output folder (default TR_RECORD_DIR)")
    p.add_argument("--rows", action="store_true", help="print draft rows from metrics.json in --out")
    p.add_argument("--self-check", nargs="+", metavar="NAME", help=argparse.SUPPRESS)
    a = p.parse_args()
    if a.self_check:
        try:
            print(json.dumps(_self_check(a.self_check[0], a.self_check[1] if len(a.self_check) > 1 else None)))
        except Stop as e:
            print(json.dumps({"stopped": True, "reason": str(e)}))
            return STOP
        return 0
    p.error("the questions are added in the next steps of slice A")


if __name__ == "__main__":
    sys.exit(main())
