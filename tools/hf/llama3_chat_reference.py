"""Llama 3.1 ve 3.2 sohbet biçiminin başvurusu: transformers'ın apply_chat_template(..., tokenize=True) kimlikleri.

İki iş yapar:
  --tokenizer-out DIR   Llama 3'ün etiketlerini özel eklenmiş parça olarak taşıyan küçük bir BPE parçalayıcısı kurar
                        (tokenizer.json, tokenizer_config.json). Belirlenimcidir: aynı sürümle aynı baytlar.
  --cases-out FILE      Durumları --tokenizer'daki parçalayıcıyla ve --template'teki şablonlarla koşar, her durumun
                        iletilerini, argümanlarını ve kimliklerini JSON olarak yazar.

--template birden çok kez verilir: AD=DOSYA (ör. 3.1=.../tokenizer_config.json). Şablon dosyadan okunur, karması
çıktıya yazılır; şablon depoya kopyalanmaz. Ağ gerekmez.

  python -I tools/hf/llama3_chat_reference.py --tokenizer-out tests/data/llama3_chat
  python -I tools/hf/llama3_chat_reference.py --tokenizer tests/data/llama3_chat \
      --template 3.1=L31/tokenizer_config.json --template 3.2=L32/tokenizer_config.json \
      --cases-out tests/data/llama3_chat/cases.json
"""
import argparse
import hashlib
import json
import os
import sys

# Llama 3'ün bu betikte kullanılan eklenmiş parçaları; yayımlanmış tokenizer.json'da 256 özel parçanın hepsi özeldir.
ADDED = ["<|begin_of_text|>", "<|end_of_text|>", "<|start_header_id|>", "<|end_header_id|>", "<|eom_id|>",
         "<|eot_id|>", "<|python_tag|>"]

# Llama 3'ün ön ayırıcı deseni (yayımlanmış tokenizer.json'un Split'i).
LLAMA3_PATTERN = (r"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*"
                  r"|\s*[\r\n]+|\s+(?!\S)|\s+")

CORPUS = [
    "Merhaba, nasılsın? Bugün hava çok güzel. Saat kaç? Yarın İstanbul'a gidiyorum.",
    "You are a helpful assistant. Answer briefly. What time is it? The weather is nice today.",
    "Cutting Knowledge Date: December 2023\nToday Date: 26 Jul 2024\n\nEnvironment: ipython\n",
    "Given the following functions, please respond with a JSON for a function call with its proper arguments "
    "that best answers the given prompt.\n\n",
    'Respond in the format {"name": function name, "parameters": dictionary of argument name and its value}.'
    "Do not use variables.\n\n",
    '{"name": "saat", "parameters": {"bolge": "Europe/Istanbul", "esik": 1e-05}}',
    "system\n\nuser\n\nassistant\n\nipython\n\n  boşluk  \t sekme \n",
    "ğüşıöçĞÜŞİÖÇ 0123456789 .,;:!?()[]{}\"'\\/+-*=<>_|@#$%^&~`",
]


def build_tokenizer(out):
    from tokenizers import AddedToken, Regex, Tokenizer, decoders, models, pre_tokenizers, processors, trainers
    tok = Tokenizer(models.BPE(byte_fallback=False, ignore_merges=True))
    tok.pre_tokenizer = pre_tokenizers.Sequence([
        pre_tokenizers.Split(Regex(LLAMA3_PATTERN), behavior="isolated", invert=False),
        pre_tokenizers.ByteLevel(add_prefix_space=False, trim_offsets=True, use_regex=False),
    ])
    tok.decoder = decoders.ByteLevel(add_prefix_space=True, trim_offsets=True, use_regex=True)
    trainer = trainers.BpeTrainer(vocab_size=600, min_frequency=1, show_progress=False,
                                  initial_alphabet=pre_tokenizers.ByteLevel.alphabet(), special_tokens=[])
    tok.train_from_iterator(CORPUS * 4, trainer=trainer)
    tok.add_special_tokens([AddedToken(t, special=True, normalized=False, single_word=False, lstrip=False, rstrip=False)
                            for t in ADDED])
    bos = tok.token_to_id("<|begin_of_text|>")
    # Yayımlanmış dosya gibi: kodlama BOS ekler (sınır parçası); biçim BOS'u kendisi taşır (24.5).
    tok.post_processor = processors.Sequence([
        processors.ByteLevel(add_prefix_space=True, trim_offsets=False, use_regex=True),
        processors.TemplateProcessing(single="<|begin_of_text|> $A", pair="<|begin_of_text|> $A <|begin_of_text|> $B:1",
                                      special_tokens=[("<|begin_of_text|>", bos)]),
    ])
    os.makedirs(out, exist_ok=True)
    tok.save(os.path.join(out, "tokenizer.json"))
    config = {"tokenizer_class": "PreTrainedTokenizerFast", "bos_token": "<|begin_of_text|>", "eos_token": "<|eot_id|>",
              "model_max_length": 131072, "clean_up_tokenization_spaces": False}
    with open(os.path.join(out, "tokenizer_config.json"), "w", encoding="utf-8", newline="") as f:
        json.dump(config, f, ensure_ascii=False, indent=2)
        f.write("\n")


def U(c):
    return {"role": "user", "content": c}


def A(c, call=None):
    m = {"role": "assistant", "content": c}
    if call:
        m["tool_calls"] = [{"type": "function", "function": {"name": call[0], "arguments": call[1]}}]
    return m


def S(c):
    return {"role": "system", "content": c}


def T(c):
    return {"role": "tool", "content": c}


SYS = "Kısa cevap ver."
# Araç şeması: ondalık sayı (Python'un 1e-05 yazımı), denetim karakteri, tırnak, ASCII dışı harf, boş nesne, iç içe nesne.
TOOLS = [
    {"type": "function", "function": {"name": "saat", "description": "Bölgedeki saati \"SS:DD\" verir.\tSekme, \u0001 denetim; ğüşİ.",
                                      "parameters": {"type": "object", "properties": {
                                          "bolge": {"type": "string", "description": "IANA adı"},
                                          "esik": {"type": "number", "minimum": 1e-05, "default": 0.5, "maximum": 1.5e+16},
                                          "adet": {"type": "integer", "maximum": 100}}, "required": ["bolge"]}}},
    {"type": "function", "function": {"name": "hava", "description": "Hava durumu", "parameters": {"type": "object", "properties": {}}}},
]
# Düz şema: ad ve parametreler üst düzeyde, `function` nesnesi yok (24.6: aracın adı `function.name` ya da `name`).
TOOLS_DUZ = [t["function"] for t in TOOLS]
CALL = ("saat", {"bolge": "Europe/İstanbul", "esik": 1e-05, "liste": [1, 2.5, True, None]})
DATE = "26 Jul 2024"

# Durumlar: dil tanımı 24.5 ve 24.6'nın Test satırları. (ad, iletiler, üretim başlığı, tarih, araçlar).
# "ref_" ile başlayanlar şablonun TR'nin hata verdiği davranışıdır: kimlikleri ya da hatası kayıttır, TR'nin beklentisi değil.
CASES = [
    ("tek_tur", [U("Merhaba")], True, DATE, None),
    ("tarih", [U("Merhaba")], True, "05 Mar 2025", None),
    ("sistemli", [S(SYS), U("Saat kaç?")], True, DATE, None),
    ("sistemsiz_egitim", [U("Merhaba"), A("Merhaba!")], False, DATE, None),
    ("cok_tur", [S(SYS), U("Merhaba"), A("Merhaba!"), U("Saat kaç?"), A("Bilmiyorum."), U("Tamam")], True, DATE, None),
    ("sistem_ortada", [U("Merhaba"), S("Artık İngilizce konuş."), U("Hello")], True, DATE, None),
    ("bosluk", [S("  sistem \n"), U(" \n Merhaba  \t\n"), A("\n\n  cevap \n")], False, DATE, None),
    # Python'un strip()'i U+001C–U+001F'yi de boşluk sayar; U+2003 Unicode boşluğudur.
    ("py_bosluk", [U("\u001fMerhaba\u001c  "), A(" cevap\u001e")], False, DATE, None),
    ("arac_tanimi", [S(SYS), U("  Saat kaç?\n")], True, DATE, TOOLS),
    ("arac_tanimi_sistemsiz", [U("Saat kaç?")], True, DATE, TOOLS),
    ("arac_cagrisi_ve_sonucu", [U("Saat kaç?"), A("", CALL), T("17:05"), A("Saat 17:05.")], False, DATE, TOOLS),
    ("arac_sonucu_tirnakli", [U("Saat kaç?"), A("", CALL), T(' {"saat": "17:05"} \n')], True, DATE, TOOLS),
    ("cevap_icerik", [U("Merhaba"), A("Merhaba! Nasılsın?")], False, DATE, None),
    ("cevap_cagri", [U("Saat kaç?"), A("", CALL)], False, DATE, TOOLS),
    ("ref_arac_ilk_asistan", [S(SYS), A("Merhaba."), U("Saat kaç?")], True, DATE, TOOLS),
    ("ref_icerik_ve_cagri", [U("Saat kaç?"), A("Bakıyorum.", CALL)], False, DATE, TOOLS),
    ("ref_arac_kullanicisiz", [S(SYS)], True, DATE, TOOLS),
    ("ref_iki_cagri", [U("Saat kaç?"), {"role": "assistant", "content": "", "tool_calls": [
        {"type": "function", "function": {"name": "saat", "arguments": {}}},
        {"type": "function", "function": {"name": "hava", "arguments": {}}}]}], False, DATE, TOOLS),
    # H7176'nın önerdiği yazım: içerik çağrıdan önceki ayrı bir asistan iletisinde, çağrının içeriği boş.
    ("icerik_sonra_cagri", [U("Saat kaç?"), A("Bakıyorum."), A("", CALL)], False, DATE, TOOLS),
    ("cevap_cagri_duz", [U("Saat kaç?"), A("", CALL)], False, DATE, TOOLS_DUZ),
]


def run_cases(tokenizer_dir, templates, out):
    from transformers import AutoTokenizer
    import transformers
    import tokenizers
    tok = AutoTokenizer.from_pretrained(tokenizer_dir)

    def encode(template, messages, kw):
        ids = tok.apply_chat_template(messages, tokenize=True, chat_template=template, **kw)
        if not isinstance(ids, list):  # transformers 5: BatchEncoding
            ids = ids["input_ids"]
        return [int(i) for i in ids]

    rows = []
    meta = {"templates": {}, "transformers": transformers.__version__, "tokenizers": tokenizers.__version__,
            "tokenizer": tokenizer_dir if tokenizer_dir.startswith("tests/") else "(dışarıda)"}
    for label, path in templates:
        raw = open(path, encoding="utf-8").read()
        template = json.loads(raw)["chat_template"] if path.endswith(".json") else raw
        meta["templates"][label] = hashlib.sha256(template.encode("utf-8")).hexdigest()
        for name, messages, gen, date, tools in CASES:
            kw = {"add_generation_prompt": gen, "date_string": date}
            if tools:
                kw["tools"] = tools
            row = {"name": name, "template": label, "messages": messages, "add_generation_prompt": gen, "date": date}
            if tools:
                row["tools"] = tools
            try:
                row["ids"] = encode(template, messages, kw)
            except Exception as e:  # şablonun raise_exception'ı
                row["raises"] = str(e)
            rows.append(row)
    # Her durum tek satır: kimlik listeleri uzundur, satır başına bir kimlik dosyayı okunmaz yapar.
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write('{"meta": ' + json.dumps(meta, ensure_ascii=False) + ',\n "cases": [\n')
        f.write(",\n".join("  " + json.dumps(r, ensure_ascii=False) for r in rows))
        f.write("\n ]\n}\n")
    print(json.dumps(meta, ensure_ascii=False))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tokenizer-out")
    ap.add_argument("--tokenizer")
    ap.add_argument("--template", action="append", default=[], help="AD=DOSYA")
    ap.add_argument("--cases-out")
    a = ap.parse_args()
    if a.tokenizer_out:
        build_tokenizer(a.tokenizer_out)
    if a.cases_out:
        if not (a.tokenizer and a.template):
            sys.exit("--cases-out needs --tokenizer and --template")
        run_cases(a.tokenizer, [t.split("=", 1) for t in a.template], a.cases_out)


if __name__ == "__main__":
    main()
