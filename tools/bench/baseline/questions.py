"""Questions of the baseline measurement set: S1–S15's commands, agreements and measures, the machines' lists and
the models' places. run.py holds the method (order, verdict, memory, stopping) and runs what this file builds.

Every question gives items: one TR side against one reference (an ABBA pair), or one TR side alone (no reference).
An item's key ("S1/qwen3-4b-q4km") is its line in metrics.json; --skip-done skips the keys already there.
Agreement (O-4): on the Colab machines a separate pass runs each item's agreement commands once before any
measure; on the laptop the warm-up runs are the agreement runs (N3). The work folder (prepare.sh) holds
tools.json (the built programs), models/ (MODELS, each at its revision, sha256 checked) and sounds/ (O-7).
"""
import hashlib, http.client, json, os, re, shutil, socket, statistics, subprocess, sys, tempfile, threading, time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
TEXTS = os.path.join(HERE, "texts.json")
IDS_EN = os.path.join(HERE, "english_phoneme_ids.json")
CATALOG = os.path.join(ROOT, "tools", "assistant", "katalog.json")

# ---------------------------------------------------------------- models (Uygulama, "İndirmeler")

MODELS = {
    "qwen3-4b-q4km": ("Qwen/Qwen3-4B-GGUF", "bc640142c66e1fdd12af0bd68f40445458f3869b", ["Qwen3-4B-Q4_K_M.gguf"]),
    "qwen3-4b-q8": ("Qwen/Qwen3-4B-GGUF", "bc640142c66e1fdd12af0bd68f40445458f3869b", ["Qwen3-4B-Q8_0.gguf"]),
    "gemma3-4b-q4km": ("bartowski/google_gemma-3-4b-it-GGUF", "71506238f970075ca85125cd749c28b1b0eee84e",
                       ["google_gemma-3-4b-it-Q4_K_M.gguf"]),
    "gemma3-4b-qat-q40": ("AnteriorAI/gemma-3-4b-it-qat-q4_0-gguf", "e7ac0af23a1a9c8dc35f63ff24262971cfe68e47",
                          ["gemma-3-4b-it-q4_0.gguf"]),
    "qwen3-8b-q4km": ("Qwen/Qwen3-8B-GGUF", "7c41481f57cb95916b40956ab2f0b139b296d974", ["Qwen3-8B-Q4_K_M.gguf"]),
    "gemma3-12b-q4km": ("ggml-org/gemma-3-12b-it-GGUF", "ec0cbabd8dbff316f659876a50202295c3c4a314",
                        ["gemma-3-12b-it-Q4_K_M.gguf"]),
    "qwen3-4b-bf16": ("Qwen/Qwen3-4B", "1cfa9a7208912126459214e8b04321603b3df60c",
                      ["config.json", "generation_config.json", "model.safetensors.index.json", "tokenizer.json",
                       "tokenizer_config.json", "vocab.json", "merges.txt"]
                      + [f"model-0000{i}-of-00003.safetensors" for i in (1, 2, 3)]),
    # TR reads config, generation_config, model.safetensors and tokenizer.json; transformers' processor the rest.
    "whisper-small": ("openai/whisper-small", "973afd24965f72e36ca33b3055d56a652f456b4d",
                      ["config.json", "generation_config.json", "model.safetensors", "tokenizer.json", "tokenizer_config.json",
                       "preprocessor_config.json", "special_tokens_map.json", "added_tokens.json", "normalizer.json",
                       "vocab.json", "merges.txt"]),
    "faster-whisper-small": ("Systran/faster-whisper-small", "536b0662742c02347bc0e980a01041f333bce120",
                             ["config.json", "model.bin", "tokenizer.json", "vocabulary.txt"]),
    "ggml-small": ("ggerganov/whisper.cpp", "5359861c739e955e79d9a303bcbc70fb988958b1", ["ggml-small.bin"]),
    "silero-vad-16k": ("snakers4/silero-vad", "5cd7945676eb32225748052e2e6a0580e4686a08",
                       ["src/silero_vad/data/silero_vad_16k.safetensors"]),
    "mms-tts-tur": ("facebook/mms-tts-tur", "7e364479c307f06733ca865b0a5269e0209cf82d",
                    ["config.json", "model.safetensors", "vocab.json", "tokenizer_config.json"]),
    "mms-tts-eng": ("facebook/mms-tts-eng", "c71de0fe7204c83f1c10820a7d696d0b450048ba",
                    ["config.json", "model.safetensors", "vocab.json", "tokenizer_config.json"]),
    "piper-tr-dfki-medium": ("rhasspy/piper-voices", "e9ab671f2a0e50a701b34361136ba3e6a3a9eb0c",
                             ["tr/tr_TR/dfki/medium/tr_TR-dfki-medium.onnx", "tr/tr_TR/dfki/medium/tr_TR-dfki-medium.onnx.json"]),
    "piper-en-lessac-medium": ("rhasspy/piper-voices", "e9ab671f2a0e50a701b34361136ba3e6a3a9eb0c",
                               ["en/en_US/lessac/medium/en_US-lessac-medium.onnx",
                                "en/en_US/lessac/medium/en_US-lessac-medium.onnx.json"]),
}
# Hosts other than the Hugging Face hub: Silero VAD comes from the project's GitHub repository, as in Dost's catalog
# (tools/assistant/katalog.json "host": "github"; indirme.tr katalog_adresi: raw.githubusercontent.com).
GITHUB = {"silero-vad-16k"}
S1_MODELS = ["qwen3-4b-q4km", "qwen3-4b-q8", "gemma3-4b-q4km", "gemma3-4b-qat-q40", "qwen3-8b-q4km", "gemma3-12b-q4km"]
QUANTIZED_OFF = {"gemma3-4b-qat-q40"}  # "düz kip" (set S1)

# ---------------------------------------------------------------- machines (set; slices B–E)

GPU_MACHINES = {"l4gpu", "g4"}
FW_LIBS = os.environ.get("BASELINE_FW_LIBS")  # CUDA 12's library folders for faster-whisper only (q_whisper; BC3)
QUESTIONS = {
    "highram": ["S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10", "S13", "S14"],
    "l4gpu": ["S1", "S2", "S3", "S5", "S6", "S7", "S8", "S9", "S10", "S11", "S12", "S13", "S14", "S15"],
    "l4cpu": ["S1", "S2", "S3", "S5", "S6", "S7", "S8", "S9", "S10", "S14"],
    "g4": ["S1", "S2", "S3", "S5", "S6", "S7", "S8", "S9", "S10", "S11", "S12", "S13", "S14", "S15"],
    "laptop": ["S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10", "S13", "S14"],
}
SOUND_QUESTIONS = {"S5", "S8"}  # they wait for O-7's sounds; without them they go last (O-7)
PLAYER = "ffmpeg -nostdin -loglevel error -re -i {wav} -f null -"  # silent, real duration (S1's Dost measurement)


class Ctx:
    """What the questions need: the machine, the thread number, the work folder and run.py's runner."""

    def __init__(self, rp, machine, threads, work, log, dry=False):
        self.rp, self.machine, self.threads, self.work, self.log = rp, machine, threads, work, log
        self.dry = dry  # builds the commands without running anything (run.py --self-check items)
        self.gpu = machine in GPU_MACHINES
        self.tr_device, self.torch_device = ("gpu", "cuda") if self.gpu else ("cpu", "cpu")
        with open(os.path.join(work, "tools.json"), encoding="utf-8") as f:
            self.tools = json.load(f)
        with open(TEXTS, encoding="utf-8") as f:
            self.texts = json.load(f)
        self.tmp = tempfile.mkdtemp(prefix="tmp-", dir=work)  # programs made for a run (char model), Dost's token folder
        self.extra = {}  # an agreement's own record by item key (S1, S3: ε, ties; S5: distances), kept in agreement.json
        self.seen_texts = {}  # S5: a reference's text by its command, run once per process for the other items
        self.epsilon_from = None  # GPU machines: HIGHRAM's agreement.json (run.py --epsilon-from; B14)
        self.epsilon_given = {}  # item key: ε written in the run script before the run (run.py --epsilon; BC17)
        self.settings = {}  # Ş1: each reference's fastest setting from the window's scan (run.py --settings)
        self.cmd_groups = {}  # K1 (H1e): a reference command's scan groups, kept where the setting is read (Ctx.ref)
        self.keys = None  # run.py --key: the items this run takes; a preparation only an unchosen item needs is skipped (BC28)

    def model(self, name, i=0):
        return os.path.join(self.work, "models", name, MODELS[name][2][i])

    def folder(self, name):
        return os.path.join(self.work, "models", name)

    def sound(self, name):
        return os.path.join(self.work, "sounds", name)

    def tr(self, program, *args):
        path = program if os.path.isabs(program) else os.path.join(HERE, program)
        return [self.tools["trlang"], path, *[str(a) for a in args]]

    def ref(self, name, binary=None, argv=(), also=()):
        # Ş1 (DENETİM 944): a reference whose fastest thread number the window's scan chose gets that number (0: the
        # library's own default); every other one the machine's number (O-1). K1 (H1e): the groups the command reads
        # are kept by the command, with the setting or without it (also: a group read beside, S3's -fa; DENETİM 960 Ş2).
        g = setting_group(name, [str(a) for a in argv])
        n = self.settings.get(g, {}).get("threads", self.threads) if g else self.threads
        cmd = self.rp.reference_cmd(name, n, binary=binary, argv=[str(a) for a in argv])
        groups = ({g} if g else set()) | set(also)
        if groups:
            self.cmd_groups.setdefault(tuple(cmd), set()).update(groups)
        return cmd

    def ngl(self):
        # İşlemci makinesinde de açık 0: llama.cpp'nin öntanımlısı -1, "bütün katmanlar GPU'da" (b12124 llama.h); L4'ün
        # llama.cpp'si CUDA'lıdır, l4cpu'da emsal GPU'da koşuyordu (BC20).
        return ["-ngl", "99" if self.gpu else "0"]


def scan_groups(ctx, it):
    """K1 (H1e): the scan groups an item's timed reference commands read; an item with none needs no scan."""
    out = set()
    for c in (it["ref"] or {}).get("cmds", []):
        out |= ctx.cmd_groups.get(tuple(c), set())
    return sorted(out)


def setting_group(name, argv):
    """Ş1: the reference whose setting the scan chooses, by the reference's own name; faster-whisper by compute type."""
    if name == "faster_whisper" and "--compute_type" in argv:
        return "faster-whisper " + argv[argv.index("--compute_type") + 1]
    if name == "llama-bench":  # N1 (DENETİM 947): reading (-n 0, S1 and S3) and generation (-p 0) have their own setting
        return "llama.cpp gen" if "-p" in argv and argv[argv.index("-p") + 1] == "0" else "llama.cpp read"
    return {"llama-completion": "llama.cpp read", "whisper-cli": "whisper.cpp", "piper": "onnxruntime"}.get(name)


def item(key, tr, ref=None, agree=None, tr_agree=None, ref_agree=None, alone_runs=5, custom=None, check=None, uses=None,
         kinds=None):
    # check: yalnız uyuşmada koşan başvuru (ref yokken): doğruluğun başvurusudur ama en hızlısı değildir, ABBA'da koşmaz ve
    # satıra oranı girmez (TCB 2026-11-20 00:15: transformers hiçbir soruda hız emsali değil; S2, S6).
    # uses: kalemin taraflarının dışında koşabilen komutlar (uyuşmanın kendi koştuğu emsaller, hazırlık, Dost'un komutu);
    # betiğin bağladığı her model kalemin bir komutunda geçer (BC28, run.py --self-check uses).
    return {"key": key, "tr": tr, "ref": ref, "agree": agree, "tr_agree": tr_agree, "ref_agree": ref_agree,
            "runs": alone_runs, "custom": custom, "check": check, "uses": uses or [], "kinds": kinds or {}}


def derived(span, what):
    """BC27: a measure computed from a longer clock span (run.py too_short compares the span, not the measure)."""
    return {"kind": "derived", "span": span, "what": what}


def not_duration(why):
    """BC27: a measure that is no duration of work (a count, a lateness, a rate); the shortest time does not apply."""
    return {"kind": "not_duration", "why": why}


def side(name, cmds, measure, parse=None, env=None, agree_parse=None, kinds=None, short_ok=None):
    # kinds: measure name -> derived(...) or not_duration(...); a measure not named is a duration (BC27, written before
    # the run). short_ok: why a measure under the shortest time does not stop this side (S4: only its memory is the row).
    s = {"name": name, "cmds": cmds, "measure": measure}
    if kinds:
        s["kinds"] = kinds
    if short_ok:
        s["short_ok"] = short_ok
    if parse:
        s["parse"] = parse
    if agree_parse:
        s["agree_parse"] = agree_parse
    if env:
        s["env"] = env
    return s


raw = lambda stdout: {"stdout": stdout}


def words(text):
    """Text for the agreement: lower case, punctuation off, single spaces (O-4's "same text")."""
    return " ".join(re.sub(r"[^\w\s']", " ", text.lower()).split())


def word_distance(a, b):
    """Levenshtein distance over words (words(): lower case, punctuation off, single spaces)."""
    h, r = words(a).split(), words(b).split()
    d = list(range(len(h) + 1))
    for i, rw in enumerate(r, 1):
        prev, d[0] = d[0], i
        for j, hw in enumerate(h, 1):
            prev, d[j] = d[j], min(d[j] + 1, d[j - 1] + 1, prev + (rw != hw))
    return d[len(h)]


def wer(hyp, ref):
    """Word error rate of hyp against ref (Levenshtein over words)."""
    return word_distance(hyp, ref) / max(len(words(ref).split()), 1)


# ---------------------------------------------------------------- S1–S3: language models

def parse_llama_bench(stdout):
    """llama-bench -o json: a list of tests; avg_ns is one test's mean time [hipotez: checked by name in A-11]."""
    rows = json.loads(stdout[stdout.index("["):])
    return {"avg_ns": rows[0]["avg_ns"], "n_prompt": rows[0]["n_prompt"], "n_gen": rows[0]["n_gen"],
            "n_depth": rows[0].get("n_depth"), "avg_ts": rows[0]["avg_ts"]}


def forced_verdict(res, epsilon=None):
    """S1, S3's agreement (TCB's B14 decision): llama.cpp's 32 greedy tokens are read by TR in one pass; every position
    either agrees (llama.cpp's token is TR's most likely one) or is a tie: TR's best logit is less than ε above the logit
    of llama.cpp's token. ε is TR's own Q4 noise, the largest |logit| difference of its quantized and unquantized paths
    at the same positions (language_model.tr --mode forced); measured, recorded, not changed after."""
    eps = res.get("epsilon") if epsilon is None else epsilon
    pos = res["positions"]
    agree = [i for i, p in enumerate(pos) if p["llama"] == p["tr"]]
    ties = [{"i": i, "gap": p["gap"]} for i, p in enumerate(pos) if p["llama"] != p["tr"] and eps is not None and p["gap"] < eps]
    apart = [{"i": i, "gap": p["gap"], "llama": p["llama"], "tr": p["tr"]} for i, p in enumerate(pos)
             if p["llama"] != p["tr"] and (eps is None or p["gap"] >= eps)]
    out = {"positions": len(pos), "agree": len(agree), "ties": ties, "epsilon": eps, "apart": apart}
    if eps is None:
        out["reason"] = "no ε: on a GPU machine give HIGHRAM's agreement.json with --epsilon-from (B14; 15.41)"
    elif len(pos) != 32:
        out["reason"] = f"llama.cpp's answer gave {len(pos)} tokens after the prompt, not 32"
    elif apart:
        out["reason"] = f"{len(apart)} positions apart beyond ε {eps:.4g}: {apart[:5]}"
    else:
        out["reason"] = None
    return out


def device_checked(ctx, key, side, cmd, run, parsed):
    """Ş2: a command an agreement function runs itself (TR's forced read, S5's other float32 references) is checked
    like a side's run: its real device from its own output (run.py check_device)."""
    check = getattr(ctx.rp, "check_device", None)
    if check:
        check(key, side, cmd, run, parsed, ctx.log, "agreement")


def agree_forced(ctx, key, forced_cmd):
    """The agreement function of S1 and S3: TR reads llama-completion's output (--given) and forced_verdict judges."""
    def agree(tr, ref):
        path = os.path.join(ctx.tmp, "llama_completion.txt")
        with open(path, "w", encoding="utf-8", newline="") as f:
            f.write(ref[0]["stdout"])
        r = ctx.rp.run_once(forced_cmd + ["--given", path])
        if r["rc"] != 0:
            return f"TR's forced read failed (exit {r['rc']}): {r['stderr'].strip()[-400:]}"
        res = ctx.rp.parse_result(r["stdout"])
        device_checked(ctx, key, "TR forced", forced_cmd, r, res)
        given = getattr(ctx, "epsilon_given", {}).get(key)
        if res.get("epsilon_note") and given is not None:
            # TR has no quantized path for these weights (Q8_0), but llama.cpp on the processor quantizes the activations
            # to 8 bits: ε is not 0 there (BC17). The run script gives the same machine's measured q4km ε before the run
            # (TCB 2026-11-20 02:29); the record names it.
            eps, source = given, f"given before the run (--epsilon {key}={given}); {res['epsilon_note']}"
        elif res.get("epsilon_note"):  # no quantized path for these weights (Q8_0): ε is 0 by rule (TCB 17:57)
            eps, source = res["epsilon"], f"by rule: {res['epsilon_note']}"
        else:
            eps, source = highram_epsilon(ctx, key) if ctx.gpu else (None, "measured here, quantized and unquantized path")
        v = dict(forced_verdict(res, eps), epsilon_source=source)
        ctx.log({"agreement": key, "tr_greedy": tr[0].get("text"), **v})
        ctx.extra[key] = {k: v[k] for k in ("epsilon", "epsilon_source", "positions", "agree", "ties", "apart")}
        return v["reason"]
    return agree


def highram_epsilon(ctx, key):
    """B14 (TCB 15:19): a GPU machine has no quantized path; it takes the same model's ε that HIGHRAM measured on the
    same prompt and positions (its agreement.json, run.py --epsilon-from), and records where it came from."""
    if not ctx.epsilon_from:
        return None, None
    with open(ctx.epsilon_from, encoding="utf-8") as f:
        got = json.load(f).get(key) or {}
    eps = got.get("epsilon") if got.get("epsilon_source", "").startswith("measured here") else None
    return eps, f"HIGHRAM {ctx.epsilon_from} {key}" if eps is not None else None


def lm_agreement(ctx, key, g, quant, text):
    """S1, S3: agree function and the two sides' agreement commands (TR's own greedy text, llama-completion's)."""
    tr_cmd = lambda mode: ctx.tr("language_model.tr", "--model", g, "--mode", mode, "--text", text, "--device", ctx.tr_device,
                                 *quant)
    # 33 token, karşılaştırılan 32: llama-completion üretimden sonra boş satır basar ("\n\n"); cevap yeniden bölününce son
    # token onunla birleşebilir (G4: "**" + "\n\n" = 56177, BC9). Birleşme yalnız karşılaştırılmayan 33.'yü bozar.
    lc = ctx.ref("llama-completion", ctx.tools["llama_completion"], ["-m", g, "-p", text, "-n", 33, "-no-cnv", *ctx.ngl()])
    return agree_forced(ctx, key, tr_cmd("forced")), [tr_cmd("agree")], [lc]


def q_language(ctx, q):
    text = ctx.texts["language_model"]["tr"]
    lb = ctx.tools["llama_bench"]
    out = []
    if q == "S1":
        for m in S1_MODELS:
            g = ctx.model(m)
            quant = ["--quantized", "off"] if m in QUANTIZED_OFF else []
            tr = side("TR", [ctx.tr("language_model.tr", "--model", g, "--mode", "speed", "--text", text,
                                    "--device", ctx.tr_device, *quant)],
                      lambda r, runs: {"prompt_ms": r[0]["prompt_ms"], "gen_ms": r[0]["gen_ms"]})
            ref = side("llama.cpp", [ctx.ref("llama-bench", lb, ["-m", g, "-p", 512, "-n", 0, *ctx.ngl()]),
                                     ctx.ref("llama-bench", lb, ["-m", g, "-p", 0, "-n", 128, "-d", 512, *ctx.ngl()])],
                       lambda r, runs: {"prompt_ms": r[0]["avg_ns"] / 1e6, "gen_ms": r[1]["avg_ns"] / 1e6},
                       parse=parse_llama_bench, agree_parse=raw)
            out.append(item(f"S1/{m}", tr, ref, *lm_agreement(ctx, f"S1/{m}", g, quant, text)))
    if q == "S2":
        f = ctx.folder("qwen3-4b-bf16")
        tr_cmd = lambda mode: ctx.tr("language_model.tr", "--folder", f, "--dtype", "bf16", "--mode", mode, "--text", text,
                                     "--device", ctx.tr_device)
        ref_cmd = lambda mode: ctx.ref("lm", argv=["--folder", f, "--dtype", "bfloat16", "--mode", mode, "--text", text,
                                                    "--device", ctx.torch_device])
        pick = lambda r, runs: {"prompt_ms": r[0]["prompt_ms"], "gen_ms": r[0]["gen_ms"], "load_ms": r[0]["load_ms"]}
        same = lambda tr, ref: None if tr[0]["tokens"] == ref[0]["tokens"] else f"{tr[0]['tokens']} != {ref[0]['tokens']}"
        # Emsal yok (en hızlısı aranıyor: llama.cpp BF16); transformers yalnız uyuşmada (TCB 00:15).
        out.append(item("S2/qwen3-4b-bf16", side("TR", [tr_cmd("speed")], pick), None, same, [tr_cmd("agree")],
                        [ref_cmd("agree")], check=side("transformers", [ref_cmd("agree")], pick)))
    if q == "S3":
        g = ctx.model("qwen3-4b-q4km")
        tr = side("TR", [ctx.tr("language_model.tr", "--model", g, "--mode", "depth", "--text", text, "--device", ctx.tr_device)],
                  lambda r, runs: {f"depth{row['depth']}_ms": row["prompt_ms"] for row in r[0]["depths"]})
        fa = ctx.settings.get("llama.cpp S3", {}).get("flash_attn")  # Ş1: the scan's -fa (llama.cpp's default is auto)
        fa = [] if fa is None else ["-fa", fa]
        # K1 (H1e, DENETİM 960 Ş2): the -fa read here is the group "llama.cpp S3", kept with or without a setting
        ref = side("llama.cpp", [ctx.ref("llama-bench", lb, ["-m", g, "-p", 40, "-n", 0, "-d", d, *fa, *ctx.ngl()],
                                         also=["llama.cpp S3"]) for d in (700, 4100)],
                   lambda r, runs: {f"depth{x['n_depth']}_ms": x["avg_ns"] / 1e6 for x in r}, parse=parse_llama_bench,
                   agree_parse=raw)
        out.append(item("S3/qwen3-4b-q4km", tr, ref, *lm_agreement(ctx, "S3/qwen3-4b-q4km", g, [], text)))  # S1's measure
    if q == "S15":
        # The warm read (T1 GPU speed, DENETİM 974 Ş2): the process reads the same 512 tokens --reps times on an emptied
        # cache; cold_ms is the first, warm_ms the median of the rest. TR alone; with --base-trlang against the base build.
        g = ctx.model("qwen3-4b-q4km")
        tr = side("TR", [ctx.tr("language_model.tr", "--model", g, "--mode", "warm", "--text", text, "--device", ctx.tr_device)],
                  lambda r, runs: {"cold_ms": r[0]["cold_ms"], "warm_ms": r[0]["warm_ms"]})
        out.append(item("S15/qwen3-4b-q4km", tr))
    return out


MEMORY_RUNS = 2  # a question that measures only memory (S4): its runs after the warm-up, written before the run
MEMORY_SHORT = "S4 measures memory: the load time is not written, the memory stays (TCB 2026-11-23 10:33)"


def q_catalog(ctx, q):
    with open(CATALOG, encoding="utf-8") as f:
        entries = json.load(f)["entries"]
    text = ctx.texts["language_model"]["tr"]
    path = {"whisper-small": ctx.folder("whisper-small"), "silero-vad-16k": ctx.model("silero-vad-16k"),
            "mms-tts-tur": ctx.folder("mms-tts-tur"), "mms-tts-eng": ctx.folder("mms-tts-eng"),
            "piper-tr-dfki-medium": ctx.model("piper-tr-dfki-medium")}
    out = []
    for e in entries:
        p = path.get(e["id"]) or ctx.model(e["id"])
        for m in e["memory"]["measured"]:
            cmd = ctx.tr("catalog_memory.tr", "--catalog", CATALOG, "--entry", e["id"], "--path", p, "--context", m["context"],
                         "--text", text, "--device", ctx.tr_device)
            # S4 yalnız belleği ölçer: bir ısınma + 2 koşu (Ahmet 2026-11-22 20:45: bellek için 5 koşu gerekmez).
            # BC27: a load under the shortest time (Silero VAD, 0–1.3 ms) does not stop it; the row is the memory.
            out.append(item(f"S4/{e['id']}/{m['context']}", side("TR", [cmd], lambda r, runs: {"load_ms": r[0]["load_ms"]},
                                                                 short_ok=MEMORY_SHORT), alone_runs=MEMORY_RUNS))
    return out


# ---------------------------------------------------------------- S5: speech to text

def q_whisper(ctx, q):
    out = []
    for lang in ("tr", "en"):
        wav = ctx.sound(f"stt_{lang}.wav")
        source = ctx.texts["speech_to_text"][lang]
        tr = side("TR", [ctx.tr("whisper.tr", "--model", ctx.folder("whisper-small"), "--audio", wav, "--language", lang,
                                "--device", ctx.tr_device)], lambda r, runs: {"transcribe_ms": r[0]["transcribe_ms"]})
        # faster-whisper's ctranslate2 wants CUDA 12's cuBLAS and cuDNN; a GPU machine without them gets them from the run
        # script, which names their folders in BASELINE_FW_LIBS. Only faster-whisper's process sees them: in one process
        # with torch's own cuDNN the versions mix (B, G4: "cuDNN sublibrary version mismatch"; BC3).
        # The folders go before the process's own LD_LIBRARY_PATH, never instead of it: Colab finds the driver's libcuda
        # through it, and without it ctranslate2 said "CUDA driver version is insufficient" (G4, third cell).
        path = ":".join(x for x in (FW_LIBS, os.environ.get("LD_LIBRARY_PATH")) if x)
        libs = ["env", f"LD_LIBRARY_PATH={path}"] if FW_LIBS else []
        fw = lambda ct: libs + ctx.ref("faster_whisper", argv=["--model", ctx.folder("faster-whisper-small"), "--audio", wav,
                                                                "--compute_type", ct, "--device", ctx.torch_device, "--language", lang])
        pick = lambda r, runs: {"transcribe_ms": r[0]["transcribe_ms"]}
        refs = [("faster-whisper-float32", side("faster-whisper float32", [fw("float32")], pick)),
                ("faster-whisper-int8", side("faster-whisper int8", [fw("int8")], pick)),
                ("whisper.cpp", side("whisper.cpp f16", [ctx.ref("whisper-cli", ctx.tools["whisper_cli"],
                                                                 ["-m", ctx.model("ggml-small"), "-f", wav, "-l", lang, "-nt",
                                                                  *([] if ctx.gpu else ["-ng"])])],
                                     lambda r, runs: {"transcribe_ms": whisper_cpp_ms(runs[0]["stderr"])},
                                     parse=lambda s: {"text": s.strip()}))]
        # transformers hız emsali değil (TCB 2026-11-20 00:15), kendi kalemi yok; float32'si B15'in sınırını kurar ve her
        # kalemin uyuşmasında bir kez koşar (s5_agreement).
        hf = ctx.ref("whisper_hf", argv=["--model", ctx.folder("whisper-small"), "--audio", wav, "--device", ctx.torch_device,
                                         "--language", lang])
        f32 = {"faster-whisper-float32": refs[0][1]["cmds"][0], "transformers": hf}
        for name, ref in refs:
            key = f"S5/{lang}/{name}"
            out.append(item(key, tr, ref, s5_agreement(ctx, key, name, f32, source), tr["cmds"], ref["cmds"],
                            uses=list(f32.values())))
    return out


F32_REFERENCES = ("faster-whisper-float32", "transformers")  # B15: only these set S5's bound


def s5_verdict(tr, texts, source):
    """S5's agreement (TCB's B15 decision): TR's word distance to each float32 reference is at most the largest
    distance of the other float32 references to it. whisper.cpp f16 and faster-whisper int8 do not set the bound
    (another precision; whisper.cpp's made-up "Altyazı M.K." would widen it); their distances are recorded. Every
    distance (TR to each reference, the references among themselves) and every WER to the source text is recorded."""
    names = sorted(texts)
    f32 = [n for n in F32_REFERENCES if n in texts]
    tr_distance = {n: word_distance(tr, texts[n]) for n in names}
    among = {f"{a} / {b}": word_distance(texts[a], texts[b]) for i, a in enumerate(names) for b in names[i + 1:]}
    bound = {n: max((word_distance(texts[n], texts[o]) for o in f32 if o != n), default=0) for n in f32}
    over = {n: {"tr": tr_distance[n], "bound": bound[n]} for n in f32 if tr_distance[n] > bound[n]}
    out = {"tr_distance": tr_distance, "among_references": among, "bound": bound,
           "wer": {"TR": wer(tr, source), **{n: wer(texts[n], source) for n in names}}}
    if len(f32) < len(F32_REFERENCES):
        out["reason"] = f"float32 references missing: {sorted(set(F32_REFERENCES) - set(f32))}"
    elif over:
        out["reason"] = f"TR's word distance beyond the float32 references' own: {over}; TR {tr!r}"
    else:
        out["reason"] = None
    return out


def s5_agreement(ctx, key, name, f32_cmds, source):
    """The agreement function of one S5 item: its reference's text, the other float32 references' texts (run here
    once per process and kept by command) and s5_verdict."""
    def agree(tr_r, ref_r):
        texts = {name: ref_r[0]["text"]}
        for other, cmd in f32_cmds.items():
            if other not in texts:
                if tuple(cmd) not in ctx.seen_texts:
                    r = ctx.rp.run_once(cmd)
                    if r["rc"] != 0:
                        return f"{other} failed (exit {r['rc']}): {r['stderr'].strip()[-400:]}"
                    parsed = ctx.rp.parse_result(r["stdout"])
                    device_checked(ctx, key, other, cmd, r, parsed)
                    ctx.seen_texts[tuple(cmd)] = parsed["text"]
                texts[other] = ctx.seen_texts[tuple(cmd)]
        v = s5_verdict(tr_r[0]["text"], texts, source)
        ctx.log({"agreement": key, **v})
        ctx.extra[key] = {k: v[k] for k in ("tr_distance", "among_references", "bound", "wer")}
        return v["reason"]
    return agree


def whisper_cpp_ms(stderr):
    """whisper.cpp's time without loading: total time − load time [hipotez: the timing lines are checked in A-11]."""
    total = float(re.search(r"total time\s*=\s*([\d.]+) ms", stderr).group(1))
    load = float(re.search(r"load time\s*=\s*([\d.]+) ms", stderr).group(1))
    return total - load


# ---------------------------------------------------------------- S6, S7: text to speech

def agree_samples(tr, ref):
    """S6, S7: at noise 0 the same number of samples per sentence and the same rate (O-4); MMS's random calls (A-4)."""
    a, b = [s["samples"] for s in tr[0]["sentences"]], [s["samples"] for s in ref[0]["sentences"]]
    if a != b or tr[0]["sample_rate"] != ref[0]["sample_rate"]:
        return f"samples {a} at {tr[0]['sample_rate']} Hz != {b} at {ref[0]['sample_rate']} Hz"
    calls = ref[0].get("random_calls")
    if calls is not None and sum(calls.values()) != 2 * len(a):
        return f"transformers draws {calls} random tensors, TR two per sentence ({2 * len(a)})"
    return None


def q_voice(ctx, q):
    texts = ["--texts", TEXTS]
    pick = lambda r, runs: {"total_ms": r[0]["total_ms"], "first_ms": r[0]["first_ms"]}
    out = []
    for lang in ("tr", "en"):
        if q == "S6":
            m = ctx.folder(f"mms-tts-{'tur' if lang == 'tr' else 'eng'}")
            tr_cmd = ctx.tr("mms.tr", "--model", m, *texts, "--language", lang, "--device", ctx.tr_device)
            ref_argv = ["--model", m, *texts, "--device", ctx.torch_device, "--language", lang]
            # Emsal yok (en hızlısı aranıyor: sherpa-onnx'in MMS ONNX modelleri); transformers yalnız uyuşmada (TCB 00:15).
            counted = [ctx.ref("mms", argv=ref_argv + ["--count_random"])]
            out.append(item(f"S6/{lang}", side("TR", [tr_cmd], pick), None, agree_samples, [tr_cmd], counted,
                            check=side("transformers", counted, pick)))
        else:
            voice = ctx.model("piper-tr-dfki-medium" if lang == "tr" else "piper-en-lessac-medium")
            ids = IDS_EN if lang == "en" else os.path.join(ctx.work, "ids_tr.json")
            tr_cmd = ctx.tr("piper.tr", "--model", voice, *texts, "--language", lang, "--ids", ids, "--device", ctx.tr_device)
            ref_cmd = ctx.ref("piper", argv=["--model", voice, "--ids", ids, *texts, "--device", ctx.torch_device, "--language", lang])
            out.append(item(f"S7/{lang}", side("TR", [tr_cmd], pick), side("onnxruntime", [ref_cmd], pick),
                            agree_samples, [tr_cmd], [ref_cmd], uses=[ids_tr_cmd(ctx)] if lang == "tr" else None))
            # En hızlı emsal (TCB 00:31, Ahmet): işlemcide onnxruntime'ın kendi varsayılanı (intra_op 0) ve 6 iş parçacığı da
            # koşar (dizüstünde 2026-11-19'da 6 iş parçacığı TR'den öndeydi, TTS-2026-11-19-LAPTOP-S1-1); satır üçünü yazar.
            # With the window's scan (Ş1) the main item already runs onnxruntime's fastest setting.
            if not ctx.gpu and "onnxruntime" not in ctx.settings:
                for name, n in (("ort-default", 0), ("ort-6", 6)):
                    cmd = ctx.rp.reference_cmd("piper", n, argv=[str(x) for x in ["--model", voice, "--ids", ids, *texts,
                                                                                   "--device", "cpu", "--language", lang]])
                    out.append(item(f"S7/{lang}/{name}", side("TR", [tr_cmd], pick), side(f"onnxruntime {name}", [cmd], pick),
                                    agree_samples, [tr_cmd], [cmd]))
    return out


def ids_tr_cmd(ctx):
    """The command that writes the Turkish S7 ids (piper.tr --write_ids) into the work folder."""
    return ctx.tr("piper.tr", "--model", ctx.model("piper-tr-dfki-medium"), "--texts", TEXTS, "--language", "tr",
                  "--write_ids", os.path.join(ctx.work, "ids_tr.json"), "--device", ctx.tr_device)


def wants(ctx, key):
    """Whether this run takes the item `key`: every item without --key, else a key that names it or a part of it."""
    return not ctx.keys or any(key.startswith(k) or k.startswith(key) for k in ctx.keys)


def piper_ids_tr(ctx):
    """The Turkish S7 ids from TR's phonemizer (piper.tr --write_ids), once per machine before S7/tr; a run of S7/en
    alone does not write them and needs no Turkish voice (BC28)."""
    path = os.path.join(ctx.work, "ids_tr.json")
    if not ctx.dry and not os.path.exists(path) and wants(ctx, "S7/tr"):
        r = ctx.rp.run_once(ids_tr_cmd(ctx))
        if r["rc"] != 0:
            raise ctx.rp.Stop(f"S7: piper.tr --write_ids failed: {r['stderr'][-400:]}")


# ---------------------------------------------------------------- S8–S10: Dost

def parse_olcu(stdout):
    """Dost's `OLCU` turn lines (tools/assistant/README.md, "Ölçü"): one dict of numbers per turn."""
    turns = []
    for line in stdout.splitlines():
        if line.startswith("OLCU "):
            turns.append({k: _number(v) for k, v in (p.split("=", 1) for p in line.split()[1:] if "=" in p)})
    return {"turns": turns}


def logged_turns(ctx, key, stdout):
    """parse_olcu, and every run's turns go to the log with the heard questions (Dost's "SÖZ:" lines): which turn is
    missing is read from the record (BC6)."""
    r = parse_olcu(stdout)
    heard = [line[len("SÖZ:"):].strip() for line in stdout.splitlines() if line.startswith("SÖZ:")]
    ctx.log({"question": key, "olcu_turns": len(r["turns"]), "heard": heard, "turns": r["turns"]})
    return r


def _number(v):
    if v in ("true", "false"):
        return v == "true"
    try:
        return int(v)
    except ValueError:
        try:
            return float(v)
        except ValueError:
            return v


def parse_sse(text):
    """The `data:` JSON of each server-sent event frame (tools/assistant/contract.md, "Olay akışı")."""
    events = []
    for frame in text.replace("\r\n", "\n").split("\n\n"):
        data = [line[len("data:"):].strip() for line in frame.split("\n") if line.startswith("data:")]
        if data:
            events.append(json.loads("\n".join(data)))
    return events


def dost_args(ctx, lang):
    """Dost's models: the catalog's default language model (its first) and the language's first voice (yuva.tr)."""
    with open(CATALOG, encoding="utf-8") as f:
        catalog = json.load(f)
    language = next(l for l in catalog["languages"] if l["code"] == lang)
    voice = language["voices"][0]
    voice_path = ctx.model(voice) if voice.startswith("piper") else ctx.folder(voice)
    args = ["--dil_modeli", ctx.model("qwen3-4b-q4km"), "--whisper", ctx.folder("whisper-small"),
            "--vad", ctx.model("silero-vad-16k"), "--ses_modeli", voice_path, "--konuşma_dili", lang,
            "--katalog", CATALOG, "--bağlam", 8192]
    if lang != "tr":
        args += ["--sistem", language["system_prompt"]]
    return [str(a) for a in args]


def turn_measures(r, runs):
    t = r[0]["turns"]
    if len(t) != 10:
        raise RuntimeError(f"{len(t)} OLCU turns, want 10")
    med = lambda k: statistics.median(x[k] for x in t)
    return {"first_sound_ms": med("ilk_ses_ms"), "first_sound_worst_ms": max(x["ilk_ses_ms"] for x in t),
            "answer_ms": med("cevap_ms"), "text_ms": med("yazi_ms"), "first_token_ms": med("ilk_parca_ms")}


def screen_cmd(ctx, lang):
    """Dost's screen mode (S9, S10) with the silent player: the answers are not played on the machine's speaker (BC23: on
    the laptop S9's trial spoke every answer aloud, four times)."""
    return ctx.tr(os.path.join(ROOT, "tools", "assistant", "asistan.tr"), *dost_args(ctx, lang)) + ["--çalıcı", PLAYER]


def q_dost(ctx, q):
    out = []
    for lang in ("tr", "en"):
        base = ctx.tr(os.path.join(ROOT, "tools", "assistant", "asistan.tr"), *dost_args(ctx, lang))
        screen = screen_cmd(ctx, lang)
        if q == "S8":
            cmd = base + ["--kip", "canlı", "--sahte_mikrofon", ctx.sound(f"turn_{lang}.wav"), "--çalıcı", PLAYER,
                          "--en_çok_tur", "10", "--cihaz", ctx.tr_device]
            out.append(item(f"S8/{lang}", side("TR", [cmd], turn_measures, parse=lambda s, k=f"S8/{lang}": logged_turns(ctx, k, s)),
                            alone_runs=3))
        elif q == "S9":
            out.append(item(f"S9/{lang}", None, alone_runs=3, custom=lambda ctx=ctx, base=screen, lang=lang: screen_turns(ctx, base, lang),
                            uses=[screen], kinds={"tokens_per_second": not_duration("a rate: Dost's metrics event, tokens per second")}))
        elif q == "S10":
            for mode in (["warm"] if ctx.machine == "laptop" else ["cold", "warm"]):
                out.append(item(f"S10/{lang}/{mode}", None, alone_runs=3,
                                custom=lambda ctx=ctx, base=screen, mode=mode: screen_ready(ctx, base, mode), uses=[screen]))
    return out


class Screen:
    """Dost's screen mode as a client: the token file stays in a temporary folder, never in a log (A-8)."""

    def __init__(self, ctx, base):
        self.dir = tempfile.mkdtemp(prefix="dost-", dir=ctx.tmp)
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            self.port = s.getsockname()[1]
        self.token_file = os.path.join(self.dir, "token")
        cmd = base + ["--kip", "canlı", "--ekran", str(self.port), "--belirteç_dosyası", self.token_file,
                      "--ayar", os.path.join(self.dir, "ayar.json")]
        self.start = time.perf_counter()
        self.proc = subprocess.Popen(cmd, cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, stdin=subprocess.DEVNULL)
        self.sampler = ctx.rp.Sampler(self.proc)
        self.events, self.lock = [], threading.Lock()

    def token(self, timeout=120):
        end = time.time() + timeout
        while time.time() < end:
            if os.path.exists(self.token_file) and os.path.getsize(self.token_file) > 0:
                with open(self.token_file, encoding="utf-8") as f:
                    return f.read().strip()
            if self.proc.poll() is not None:
                raise RuntimeError(f"Dost exited {self.proc.returncode}: {self.proc.stderr.read()[-400:]}")
            time.sleep(0.05)
        raise RuntimeError("no token file")

    def listen(self, token):
        req = urllib.request.Request(f"http://127.0.0.1:{self.port}/events", headers={"Authorization": f"Bearer {token}"})
        resp = urllib.request.urlopen(req, timeout=3600)

        def read():
            buf = ""
            try:
                for line in resp:
                    buf += line.decode("utf-8")
                    if buf.endswith("\n\n"):
                        stamp = (time.perf_counter() - self.start) * 1000
                        with self.lock:
                            self.events += [(stamp, e) for e in parse_sse(buf)]
                        buf = ""
            except (OSError, http.client.HTTPException):
                pass  # the stream ends when the run stops Dost (Windows: connection reset); wait() reads the events
        threading.Thread(target=read, daemon=True).start()

    def wait(self, test, timeout=1800, after=0):
        end = time.time() + timeout
        while time.time() < end:
            with self.lock:
                for stamp, e in self.events[after:]:
                    if test(e):
                        return stamp, e
            if self.proc.poll() is not None:
                raise RuntimeError(f"Dost exited {self.proc.returncode}")
            time.sleep(0.02)
        raise RuntimeError("timed out waiting for an event")

    def send(self, token, kind, data):
        body = json.dumps({"v": 2, "command_id": os.urandom(8).hex(), "type": kind, "data": data}).encode()
        req = urllib.request.Request(f"http://127.0.0.1:{self.port}/command", data=body, method="POST",
                                     headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json",
                                              "Origin": f"http://127.0.0.1:{self.port}"})
        return json.load(urllib.request.urlopen(req, timeout=60))

    def close(self):
        self.proc.terminate()
        try:
            self.proc.wait(timeout=30)
        except subprocess.TimeoutExpired:
            self.proc.kill()
        self.sampler.stop()
        shutil.rmtree(self.dir, ignore_errors=True)
        return {"vmhwm_kb": self.sampler.hwm_kb, "rss_anon_sampled_kb": self.sampler.anon_kb,
                "peak_working_set_kb": self.sampler.ws_kb, "peak_pagefile_kb": self.sampler.pagefile_kb,
                "gpu_peak_mib": self.sampler.gpu_mib}


ready = lambda e: e.get("type") == "state" and e["data"].get("state") in ("idle", "listening")


def screen_turns(ctx, base, lang):
    """S9: the written chat's first token and tokens/s from the `metrics` event, the S8 questions as text."""
    s = Screen(ctx, base)
    try:
        token = s.token()
        s.listen(token)
        s.wait(ready)
        rows = []
        for question in ctx.texts["voice_turn"][lang]:
            n = len(s.events)
            s.send(token, "send_text", {"text": question})
            _, e = s.wait(lambda e: e.get("type") == "metrics" and "first_token_ms" in e["data"], after=n)
            rows.append(e["data"])
            s.wait(ready, after=n)
    finally:
        memory = s.close()
    return {"first_token_ms": statistics.median(r["first_token_ms"] for r in rows),
            "tokens_per_second": statistics.median(r["tokens_per_second"] for r in rows)}, memory


def cold_files(base):
    """S10's cold start (TCB 17:45): the files the screen opens: trlang, every file and folder the command names (the
    models, the catalog) and lib's and tools/assistant's TR files."""
    paths = []
    for a in base:
        if os.path.isfile(a):
            paths.append(a)
        elif os.path.isdir(a):
            paths += [os.path.join(d, f) for d, _, fs in os.walk(a) for f in fs]
    for folder in (os.path.join(ROOT, "lib"), os.path.join(ROOT, "tools", "assistant")):
        paths += [os.path.join(d, f) for d, _, fs in os.walk(folder) for f in fs if f.endswith(".tr")]
    return sorted(set(paths))


def cached_bytes(paths):
    """The page cache's bytes of the files (Linux): each file is mapped and mincore(2) says which pages are resident.
    Colab has no util-linux fincore (G4, second cell: "No such file or directory: 'fincore'")."""
    import ctypes
    libc = ctypes.CDLL(None, use_errno=True)
    libc.mmap.restype = ctypes.c_void_p
    libc.mmap.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_long]
    libc.mincore.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_void_p]
    libc.munmap.argtypes = [ctypes.c_void_p, ctypes.c_size_t]
    page, total, failed = os.sysconf("SC_PAGE_SIZE"), 0, ctypes.c_void_p(-1).value
    for p in paths:
        size = os.path.getsize(p)
        if size == 0:
            continue
        fd = os.open(p, os.O_RDONLY)
        try:
            addr = libc.mmap(None, size, 1, 1, fd, 0)  # PROT_READ, MAP_SHARED
            if addr in (None, failed):
                raise OSError(ctypes.get_errno(), f"mmap {p}")
            try:
                vec = (ctypes.c_ubyte * ((size + page - 1) // page))()
                if libc.mincore(ctypes.c_void_p(addr), size, vec) != 0:
                    raise OSError(ctypes.get_errno(), f"mincore {p}")
                total += sum(b & 1 for b in bytes(vec)) * page
            finally:
                libc.munmap(ctypes.c_void_p(addr), size)
        finally:
            os.close(fd)
    return total


def cold_dropped(before, after):
    """The drop worked when at most a tenth of the files' cached bytes is left."""
    return after <= before // 10


def drop_files(ctx, base):
    """S10 cold: sync, then every file the screen opens leaves the page cache (posix_fadvise DONTNEED; no root). Colab's
    /proc/sys is read-only, so the whole cache cannot be dropped (B, G4). The cached bytes before and after go to the log;
    if they did not fall, the measure stops (TCB 17:45)."""
    paths = cold_files(base)
    before = cached_bytes(paths)
    subprocess.run(["sync"], check=True)
    for p in paths:
        fd = os.open(p, os.O_RDONLY)
        try:
            os.posix_fadvise(fd, 0, 0, os.POSIX_FADV_DONTNEED)
        finally:
            os.close(fd)
    after = cached_bytes(paths)
    ctx.log({"question": "S10 cold", "files": len(paths), "cached_before_bytes": before, "cached_after_bytes": after})
    if not cold_dropped(before, after):
        raise ctx.rp.Stop(f"S10 cold: the files' cache fell from {before} to {after} bytes only (fadvise)")


def screen_ready(ctx, base, mode):
    """S10: from the start to the screen's ready state; cold after the screen's files leave the page cache (drop_files:
    the files, not the whole system cache)."""
    if mode == "cold":
        drop_files(ctx, base)
    s = Screen(ctx, base)
    try:
        s.listen(s.token())
        stamp, _ = s.wait(ready)
    finally:
        memory = s.close()
    return {"ready_ms": stamp}, memory


# ---------------------------------------------------------------- S11–S14: tensor and runtime

def ms_per_step(stdout):
    """The last "<n> ms/adım" of rgn512.tr's or rgn512.py's log (tools/gpu/COLAB.md)."""
    found = re.findall(r"([\d.]+) ms/adım", stdout)
    if not found:
        raise ValueError("no ms/adım line in the training log")
    return {"ms_per_step": float(found[-1])}


def q_runtime(ctx, q):
    out = []
    if q == "S11":
        data = os.path.join(ctx.work, "rgn")
        for precision in ("bf16", "fp32"):
            tr = side("TR", [ctx.tr(os.path.join(ROOT, "tools", "gpu", "rgn512.tr"), "--veri", data, "--step", 30, "--logger", 30,
                                    "--hassasiyet", precision, "--device", "gpu")],
                      lambda r, runs: r[0], parse=ms_per_step)
            if precision == "fp32":  # rgn512.py has no float32 mode (defter 5)
                out.append(item("S11/fp32", tr))
                continue
            for mode in ("eager", "compile"):
                cmd = [sys.executable, "-I", os.path.join(ROOT, "tools", "gpu", "rgn512.py"), "train", "--data",
                       os.path.join(data, "train.npy"), "--out", os.path.join(ctx.tmp, f"rgn_{mode}"), "--steps", "30"]
                ref = side(f"rgn512.py {mode}", [cmd + (["--no_compile"] if mode == "eager" else [])], lambda r, runs: r[0],
                           parse=ms_per_step)
                out.append(item(f"S11/bf16/{mode}", tr, ref))
    if q == "S12":
        for dtype in ("float32", "bf16"):
            for kernel in ("tr", "vendor"):
                tr_cmd = lambda fill: ctx.tr("matmul.tr", "--dtype", dtype, "--kernel", kernel, "--fill", fill, "--device", ctx.tr_device)
                ref_cmd = lambda fill: ctx.ref("matmul", argv=["--dtype", dtype, "--fill", fill, "--device", ctx.torch_device])
                pick = lambda r, runs: {"median_ms": r[0]["median_ms"]}
                same = lambda tr, ref: None if tr[0]["corner"] == ref[0]["corner"] else f"corner {tr[0]['corner']} != {ref[0]['corner']}"
                out.append(item(f"S12/{dtype}/{kernel}", side("TR", [tr_cmd("random")], pick), side("torch", [ref_cmd("random")], pick),
                                same, [tr_cmd("ones")], [ref_cmd("ones")]))
    if q == "S13":
        out += q_targets(ctx)
    if q == "S14":
        out.append(item("S14/empty", side("TR", [ctx.tr("empty.tr")], lambda r, runs: runs[0]["wall_ms"], parse=raw)))
    return out


def last_ms(stdout):
    """tools/bench's programs end with "<name> <value> <ms>" (tools/bench/run.py)."""
    parts = stdout.strip().splitlines()[-1].split()
    return {"value": parts[1], "ms": float(parts[-1])}


# S13'ün saf hesap işlerinin boyu (--n, iki tarafa aynı), koşudan önce yazılı: tools/bench/run.py'nin boyunda Go dizüstünde
# 0–1,2 ms sürdü (saat adımı ~0,5 ms; fib'de 0 ms, oran sıfıra bölündü; 2026-11-22 penceresi), ölçü anlamsızdı. Bu boylarda Go
# dizüstünde en kısa ölçülebilir sürenin (BC27, 20 ms) en az üç katı sürer [prova 2026-11-23, sayısı ölçü değil], TR bir koşuda
# yarım dakikadan az. Uyuşma iki tarafın bütün sonuç değerini karşılaştırır; matris_dongu'da değer iz ve bütün öğelerin
# toplamıdır ("<iz>/<toplam>"): her iç çarpım sonuca girer (DENETİM 953 Ş2).
PURE_SIZES = {"dongu": 300_000_000, "fib": 37, "matris_dongu": 480}


def q_targets(ctx):
    """S13: the roadmap's targets: pure compute TR/Go, the character model TR/PyTorch, the 100 Hz loop."""
    loop = item("S13/loop_100hz", side("TR", [ctx.tr("loop_100hz.tr", "--device", ctx.tr_device)],
                                       lambda r, runs: {"missed": r[0]["missed"], "worst_late_ms": r[0]["worst_late_ms"]},
                                       kinds={"missed": not_duration("a count of late turns"),
                                              "worst_late_ms": not_duration("a turn's lateness past its deadline (TR_monotonic_ns), "
                                                                            "not a work's time; the loop runs 1000 turns of 10 ms")}))
    if ctx.gpu:  # GPUs: only the loop (set S13, C-2)
        return [loop]
    out = []
    for prog, n in PURE_SIZES.items():
        tr = side("TR", [ctx.tr(os.path.join(ROOT, "tools", "bench", "tr", f"{prog}.tr"), "--n", n)], lambda r, runs: r[0]["ms"],
                  parse=last_ms)
        go = side("Go", [[ctx.tools["go_compare"], prog, "--n", str(n)]], lambda r, runs: r[0]["ms"], parse=last_ms)
        same = lambda a, b: None if a[0]["value"] == b[0]["value"] else f"{a[0]['value']} != {b[0]['value']}"
        out.append(item(f"S13/{prog}", tr, go, same, tr["cmds"], go["cmds"]))
    bench = _bench_run()
    programs = [bench.model_program(ctx.tmp, n) for n in (bench.MODEL_STEPS, 0)]
    step = lambda r, runs: (runs[0]["wall_ms"] - runs[1]["wall_ms"]) / bench.MODEL_STEPS
    # BC27: a step is read from the two runs' wall difference (MODEL_STEPS steps), not from one step's clock.
    span = {"ms": derived(lambda r, runs: runs[0]["wall_ms"] - runs[1]["wall_ms"],
                          f"the {bench.MODEL_STEPS}-step run's wall minus the 0-step run's")}
    py = [sys.executable, "-I", os.path.join(ROOT, "tools", "bench", "py", "char_model.py")]
    out.append(item("S13/char_model", side("TR", [ctx.tr(p) for p in programs], step, parse=raw, kinds=span),
                    side("PyTorch", [py + [str(n), str(ctx.threads)] for n in (bench.MODEL_STEPS, 0)], step, parse=raw,
                         env=dict(os.environ, PYTHONIOENCODING="utf-8"), kinds=span)))
    out.append(loop)
    return out


def _bench_run():
    """tools/bench/run.py by its path: python -I puts no script folder on sys.path and both files are run.py (A-6, N2)."""
    import importlib.util
    spec = importlib.util.spec_from_file_location("bench_run", os.path.join(ROOT, "tools", "bench", "run.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


BUILDERS = {"S1": q_language, "S2": q_language, "S3": q_language, "S15": q_language, "S4": q_catalog, "S5": q_whisper, "S6": q_voice,
            "S7": q_voice, "S8": q_dost, "S9": q_dost, "S10": q_dost, "S11": q_runtime, "S12": q_runtime, "S13": q_runtime,
            "S14": q_runtime}


def items(ctx, q):
    if q == "S7":
        piper_ids_tr(ctx)
    return BUILDERS[q](ctx, q)


# ---------------------------------------------------------------- O-7: the fixed sounds (A-11, HIGHRAM, once)

SOUNDS = {  # name: (program, model, texts set, language, gap between sentences in s)
    "stt_tr.wav": ("piper.tr", "piper-tr-dfki-medium", "speech_to_text", "tr", 0.3),
    "stt_en.wav": ("mms.tr", "mms-tts-eng", "speech_to_text", "en", 0.3),
    "turn_tr.wav": ("piper.tr", "piper-tr-dfki-medium", "voice_turn", "tr", 48.0),
    "turn_en.wav": ("mms.tr", "mms-tts-eng", "voice_turn", "en", 48.0),
}


SOUND_RATE = 16000  # Whisper's rate (lib/audio.tr _WHISPER_HIZI); Piper speaks at 22 050 Hz, MMS at 16 000 Hz


def make_sounds(ctx):
    """Makes O-7's sounds with TR's voices (Piper noise 0, MMS the model's noise with the fixed seed) and their sha256.

    Every sound is 16 kHz mono 16-bit: ffmpeg resamples the voice's WAV (TR has no resampler; lib/audio.tr's message
    gives the same ffmpeg command). Dost's fake microphone reads any rate through ffmpeg; S5's programs need 16 kHz."""
    os.makedirs(ctx.sound(""), exist_ok=True)
    r = ctx.rp.run_once(["ffmpeg", "-version"])
    ctx.log({"sounds_ffmpeg": r["stdout"].splitlines()[0] if r["rc"] == 0 and r["stdout"] else r["stderr"][-200:]})
    sums = {}
    for name, (program, model, group, lang, gap) in SOUNDS.items():
        m = ctx.model(model) if program == "piper.tr" else ctx.folder(model)
        noise = "zero" if program == "piper.tr" else "model"
        voice_wav = ctx.sound(f"voice_{name}")
        r = ctx.rp.run_once(ctx.tr(program, "--model", m, "--texts", TEXTS, "--set", group, "--language", lang,
                                   "--noise", noise, "--wav", voice_wav, "--gap_s", gap))
        if r["rc"] != 0:
            raise ctx.rp.Stop(f"sound {name}: {r['stderr'][-400:]}")
        # Dost's sounds (S8) end with the same silence as between the questions: the fake microphone's file ends, and
        # Dost closes a turn only after silence, so without it the last question had no turn (B, G4: 9 of 10; BC6).
        pad = ["-af", f"apad=pad_dur={gap}"] if group == "voice_turn" else []
        r = ctx.rp.run_once(["ffmpeg", "-nostdin", "-loglevel", "error", "-y", "-i", voice_wav, *pad, "-ar", str(SOUND_RATE),
                             "-ac", "1", "-c:a", "pcm_s16le", ctx.sound(name)])
        if r["rc"] != 0:
            raise ctx.rp.Stop(f"sound {name}: ffmpeg failed: {r['stderr'][-400:]}")
        os.remove(voice_wav)
        sums[name] = sha256(ctx.sound(name))
    with open(ctx.sound("sounds.json"), "w", encoding="utf-8", newline="") as f:
        json.dump(sums, f, indent=1)
    return sums


def sounds_ok(ctx):
    """O-7: the sounds are there and their sha256 is the one written when they were made."""
    try:
        with open(ctx.sound("sounds.json"), encoding="utf-8") as f:
            sums = json.load(f)
    except OSError:
        return False
    return set(sums) == set(SOUNDS) and all(os.path.exists(ctx.sound(n)) and sha256(ctx.sound(n)) == s for n, s in sums.items())


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()
