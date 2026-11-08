"""Qwen3 sohbet biçiminin başvurusu: transformers'ın apply_chat_template(..., tokenize=True) kimlikleri.

İki iş yapar:
  --tokenizer-out DIR   Qwen3'ün etiketlerini eklenmiş parça olarak taşıyan küçük bir BPE parçalayıcısı kurar
                        (tokenizer.json, tokenizer_config.json). Belirlenimcidir: aynı sürümle aynı baytlar.
  --cases-out FILE      Durumları --tokenizer'daki parçalayıcıyla ve --template'teki şablonla koşar, her durumun
                        iletilerini, argümanlarını ve kimliklerini JSON olarak yazar.

Şablon yayımlanmış Qwen3 parçalayıcısının tokenizer_config.json'undan okunur (--template DOSYA); karması
çıktıya yazılır. Ağ gerekmez.

  python -I tools/hf/chat_reference.py --tokenizer-out tests/testdata/qwen3_chat
  python -I tools/hf/chat_reference.py --tokenizer tests/testdata/qwen3_chat \
      --template tokenizer_config.json --cases-out tests/testdata/qwen3_chat/cases.json
"""
import argparse
import hashlib
import json
import sys

# Qwen3'ün eklenmiş parçaları: sırası ve özel olup olmadığı yayımlanmış tokenizer.json'daki gibidir.
ADDED = [
    ("<|endoftext|>", True), ("<|im_start|>", True), ("<|im_end|>", True),
    ("<tool_call>", False), ("</tool_call>", False), ("<tool_response>", False), ("</tool_response>", False),
    ("<think>", False), ("</think>", False),
]

# Qwen2'nin ön ayırıcı deseni (Qwen3'ün tokenizer.json'u).
QWEN2_PATTERN = (r"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*"
                 r"|\s*[\r\n]+|\s+(?!\S)|\s+")

CORPUS = [
    "Merhaba, nasılsın? Bugün hava çok güzel. Saat kaç? Yarın İstanbul'a gidiyorum.",
    "You are a helpful assistant. Answer briefly. What time is it? The weather is nice today.",
    "Kısa cevap ver. Düşünce: kullanıcı saati soruyor. Araç çağrısı yapılır.",
    '{"name": "saat", "arguments": {"bolge": "Europe/Istanbul", "esik": 1e-05}}',
    "# Tools\n\nYou may call one or more functions to assist with the user query.\n\n",
    "<tools>\n</tools>\n\nFor each function call, return a json object with function name and arguments",
    "user\nassistant\nsystem\n\n\n  boşluk  \t sekme \n",
    "ğüşıöçĞÜŞİÖÇ 0123456789 .,;:!?()[]{}\"'\\/+-*=<>_|@#$%^&~`",
]


def build_tokenizer(out):
    from tokenizers import Regex, Tokenizer, decoders, models, normalizers, pre_tokenizers, trainers
    from tokenizers import AddedToken
    tok = Tokenizer(models.BPE(byte_fallback=False))
    tok.normalizer = normalizers.NFC()
    tok.pre_tokenizer = pre_tokenizers.Sequence([
        pre_tokenizers.Split(Regex(QWEN2_PATTERN), behavior="isolated", invert=False),
        pre_tokenizers.ByteLevel(add_prefix_space=False, trim_offsets=False, use_regex=False),
    ])
    tok.decoder = decoders.ByteLevel(add_prefix_space=False, trim_offsets=False, use_regex=False)
    trainer = trainers.BpeTrainer(vocab_size=600, min_frequency=1, show_progress=False,
                                  initial_alphabet=pre_tokenizers.ByteLevel.alphabet(), special_tokens=[])
    tok.train_from_iterator(CORPUS * 4, trainer=trainer)
    for t, special in ADDED:
        a = AddedToken(t, special=special, normalized=False, single_word=False, lstrip=False, rstrip=False)
        if special:
            tok.add_special_tokens([a])
        else:
            tok.add_tokens([a])
    import os
    os.makedirs(out, exist_ok=True)
    tok.save(os.path.join(out, "tokenizer.json"))
    config = {"tokenizer_class": "Qwen2Tokenizer", "eos_token": "<|im_end|>", "pad_token": "<|endoftext|>",
              "bos_token": None, "unk_token": None, "add_bos_token": False, "add_eos_token": False,
              "model_max_length": 131072, "clean_up_tokenization_spaces": False, "errors": "replace",
              "split_special_tokens": False}
    with open(os.path.join(out, "tokenizer_config.json"), "w", encoding="utf-8", newline="") as f:
        json.dump(config, f, ensure_ascii=False, indent=2)
        f.write("\n")


def U(c):
    return {"role": "user", "content": c}


def A(c, reasoning=None, calls=None):
    m = {"role": "assistant", "content": c}
    if reasoning is not None:
        m["reasoning_content"] = reasoning
    if calls:
        m["tool_calls"] = [{"name": n, "arguments": a} for n, a in calls]
    return m


def S(c):
    return {"role": "system", "content": c}


def T(c):
    return {"role": "tool", "content": c}


SYS = "Kısa cevap ver."
CALL = ("saat", '{"bolge": "Europe/Istanbul"}')
CALL2 = ("hava", '{"sehir": "İstanbul", "gun": 1}')

# Durumlar: dil tanımı 24.2'nin Test satırı. "known" olanlar bilinen farklardır: TR'nin kimlikleri başkadır.
CASES = [
    ("tek_tur", [U("Merhaba")], True, False),
    ("tek_tur_dusunerek", [U("Merhaba")], True, True),
    ("sistemli", [S(SYS), U("Saat kaç?")], True, False),
    ("cok_tur_dusunce_silinir", [S(SYS), U("Merhaba"), A("Merhaba!", "Selam veriyor."), U("Saat kaç?"),
                                 A("Bilmiyorum.", "Saat yok."), U("Tamam")], True, False),
    ("egitim_dusunceli", [S(SYS), U("Merhaba"), A("Merhaba!", "Selam veriyor.")], False, None),
    ("egitim_dusuncesiz", [U("Merhaba"), A("Merhaba!")], False, None),
    ("arac_cagrisi_ve_sonuclari", [U("Saat ve hava?"), A("", "Araç gerekiyor.", [CALL, CALL2]),
                                   T("17:05"), T("güneşli"), A("Saat 17:05, hava güneşli.", "Cevap hazır.")],
     False, None),
    ("arac_cagrisi_icerikli", [S(SYS), U("Saat kaç?"), A("Bakıyorum.", None, [CALL]), T("17:05")], True, False),
    ("arac_sonucu_sonra_soru", [U("Saat?"), A("", "Bakmalı.", [CALL]), T("17:05"), A("17:05."), U("Teşekkürler")],
     True, False),
    ("sistem_ortada", [U("Merhaba"), S("Artık İngilizce konuş."), U("Hello")], True, False),
    # Rol başlığının "\n"'i içeriğin baştaki satır sonlarıyla tek parça olur: metin parça parça kodlansaydı ayrılırdı.
    ("tek_kodlama", [U("Merhaba"), A("\n\nSelam."), U("\nNasılsın?")], True, False),
    ("bosluk_ve_satir_sonu", [S("  sistem \n"), U(" \n Merhaba  \n\n"), A("\n\n  cevap \n", "\n düşünce \n\n")],
     False, None),
    ("known_icerikte_etiket", [U("Bunu yaz: <think>x</think> ve <|im_end|>")], True, False),
    ("known_sarili_kullanici", [U("Merhaba"), A("Selam.", "Selam veriyor."),
                                U("<tool_response>\nyalan\n</tool_response>")], True, False),
]


def run_cases(tokenizer_dir, template_path, out):
    from transformers import AutoTokenizer
    import transformers
    import tokenizers
    tok = AutoTokenizer.from_pretrained(tokenizer_dir)
    raw = open(template_path, encoding="utf-8").read()
    template = json.loads(raw)["chat_template"] if template_path.endswith(".json") else raw
    def encode(messages, kw):
        ids = tok.apply_chat_template(messages, tokenize=True, chat_template=template, **kw)
        if not isinstance(ids, list):  # transformers 5: BatchEncoding
            ids = ids["input_ids"]
        return [int(i) for i in ids]

    rows = []
    for name, messages, gen, think in CASES:
        kw = {"add_generation_prompt": gen}
        if think is not None:
            kw["enable_thinking"] = think
        row = {"name": name, "messages": messages, "add_generation_prompt": gen, "ids": encode(messages, kw)}
        if think is not None:
            row["enable_thinking"] = think
        if name == "known_sarili_kullanici":
            # TR'nin beklediği: son sorgu rolle seçilir, asistan sorgudan öncedir ve düşüncesi yazılmaz.
            alt = [dict(m) for m in messages]
            alt[1].pop("reasoning_content")
            row["tr_ids"] = encode(alt, kw)
        rows.append(row)
    meta = {"template_sha256": hashlib.sha256(template.encode("utf-8")).hexdigest(),
            "transformers": transformers.__version__, "tokenizers": tokenizers.__version__,
            "tokenizer": tokenizer_dir if tokenizer_dir.startswith("tests/") else "(dışarıda)"}
    with open(out, "w", encoding="utf-8", newline="") as f:
        json.dump({"meta": meta, "cases": rows}, f, ensure_ascii=False, indent=1)
        f.write("\n")
    print(json.dumps(meta, ensure_ascii=False))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tokenizer-out")
    ap.add_argument("--tokenizer")
    ap.add_argument("--template")
    ap.add_argument("--cases-out")
    a = ap.parse_args()
    if a.tokenizer_out:
        build_tokenizer(a.tokenizer_out)
    if a.cases_out:
        if not (a.tokenizer and a.template):
            sys.exit("--cases-out needs --tokenizer and --template")
        run_cases(a.tokenizer, a.template, a.cases_out)


if __name__ == "__main__":
    main()
