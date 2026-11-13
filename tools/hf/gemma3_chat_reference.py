"""Gemma 3 sohbet biçiminin başvurusu: transformers'ın apply_chat_template(..., tokenize=True) kimlikleri (dil tanımı 24.7, 24.8).

İki iş yapar:
  --tokenizer-out DIR   tests/data/gemma3_format_pieces/'in parçalayıcısına Gemma 3'ün sohbet etiketlerini (<pad>, <eos>,
                        <bos>, <start_of_turn>, <end_of_turn>) yayımlanmış dosyadaki kimlikleriyle özel eklenmiş parça
                        olarak ekler. Biçim parçaları (16.2) fikstürdeki gibi kalır: klasör Gemma 3 ailesiyle açılır ve
                        biçimin "\\n"'i tek kimliktir. Belirlenimcidir ve yalnız JSON okur: aynı fikstürle aynı baytlar.
  --cases-out FILE      Durumları --tokenizer'daki parçalayıcıyla ve --template'teki şablonlarla koşar, her durumun
                        iletilerini, argümanlarını ve kimliklerini JSON olarak yazar. --check-ids verilirse etiketlerin
                        kimlikleri yayımlanmış tokenizer_config.json'un added_tokens_decoder'ıyla karşılaştırılır.

--template birden çok kez verilir: AD=DOSYA (ör. 1b=G1B/chat_template.json). Şablon dosyadan okunur, karması çıktıya
yazılır; şablon depoya kopyalanmaz. Ağ gerekmez.

  python -I tools/hf/gemma3_chat_reference.py --tokenizer-out tests/data/gemma3_chat
  python -I tools/hf/gemma3_chat_reference.py --tokenizer tests/data/gemma3_chat --check-ids G1B/tokenizer_config.json \
      --template 1b=G1B/chat_template.json --template 4b=G4B/chat_template.json \
      --cases-out tests/data/gemma3_chat/cases.json
"""
import argparse
import hashlib
import json
import os
import sys

FIXTURE = os.path.join("tests", "data", "gemma3_format_pieces")

# Gemma 3'ün sohbet etiketleri ve yayımlanmış tokenizer.json'daki kimlikleri (hepsi special: true).
TAGS = {"<pad>": 0, "<eos>": 1, "<bos>": 2, "<start_of_turn>": 105, "<end_of_turn>": 106}


def build_tokenizer(out):
    src = json.load(open(os.path.join(FIXTURE, "tokenizer.json"), encoding="utf-8"))
    model = dict(src["model"])
    vocab = dict(model["vocab"])
    added = list(src["added_tokens"])
    for content, i in TAGS.items():
        if content in vocab or i in vocab.values():
            print("fikstürde zaten var:", json.dumps(content), i)
            return 1
        vocab[content] = i
        added.append({"id": i, "content": content, "single_word": False, "lstrip": False, "rstrip": False,
                      "normalized": False, "special": True})
    model["vocab"] = dict(sorted(vocab.items(), key=lambda kv: kv[1]))
    tok = dict(src)
    tok["added_tokens"] = sorted(added, key=lambda t: t["id"])
    tok["model"] = model
    os.makedirs(out, exist_ok=True)
    files = {"tokenizer.json": tok}
    for name in ("config.json", "tokenizer_config.json"):
        files[name] = json.load(open(os.path.join(FIXTURE, name), encoding="utf-8"))
    for name, value in files.items():
        with open(os.path.join(out, name), "w", encoding="utf-8", newline="") as f:
            json.dump(value, f, ensure_ascii=False, indent=2)
            f.write("\n")
    for name in files:
        data = open(os.path.join(out, name), "rb").read()
        print("%s %d bayt sha256 %s" % (name, len(data), hashlib.sha256(data).hexdigest()))
    return 0


def check_ids(path):
    """Yayımlanmış tokenizer_config.json'un added_tokens_decoder'ı TAGS'ın kimliklerini ve special bayrağını taşımalı."""
    dec = json.load(open(path, encoding="utf-8"))["added_tokens_decoder"]
    ok = True
    for content, i in TAGS.items():
        t = dec.get(str(i))
        same = t is not None and t["content"] == content and t["special"] is True
        ok = ok and same
        print("kimlik %d %s: yayımlanmış %s; eşit: %s" % (i, content, json.dumps(t, ensure_ascii=False), same))
    return ok


def U(c):
    return {"role": "user", "content": c}


def A(c):
    return {"role": "assistant", "content": c}


def S(c):
    return {"role": "system", "content": c}


def T(c):
    return {"role": "tool", "content": c}


SYS = "Kısa cevap ver."

# Durumlar: dil tanımı 24.7 ve 24.8'in Test satırları. (ad, iletiler, üretim istemi).
# "ref_" ile başlayanlar şablonun TR'nin hata verdiği ya da bilerek ayrıldığı davranışıdır: kimlikleri ya da hatası
# kayıttır, TR'nin beklentisi değil.
CASES = [
    ("tek_tur", [U("Merhaba")], True),
    ("sistemli", [S(SYS), U("Saat kaç?")], True),
    # Sistem iletisi kırpılmaz; kullanıcının ve asistanın içeriği kırpılır.
    ("sistem_bosluklu", [S("  sistem \n"), U(" \n Merhaba  \t\n"), A("\n\n  cevap \n")], False),
    ("cok_tur", [S(SYS), U("Merhaba"), A("Merhaba!"), U("Saat kaç?"), A("Bilmiyorum."), U("Tamam")], True),
    ("egitim", [U("Merhaba"), A("Merhaba! Nasılsın?")], False),
    # Python'un strip()'i U+001C–U+001F'yi de boşluk sayar; U+2003 Unicode boşluğudur.
    ("py_bosluk", [U("\u001fMerhaba\u001c  "), A("\u2003cevap\u001e\u001d")], False),
    # İçerikte "\n\n" ailenin biçim parçasıdır (16.2).
    ("icerik_satir", [U("bir\n\niki\nüç"), A("dört\n\n\nbeş")], False),
    ("bos_liste", [], True),
    ("cevap_tek", [U("Merhaba"), A("Merhaba! Nasılsın?")], False),
    ("cevap_cok", [S(SYS), U("Merhaba"), A("Merhaba!"), U("Saat kaç?"), A("Bilmiyorum.")], False),
    # İçerikte yazılan etiket: transformers onu etiketin kimliğiyle kodlar, TR metin bırakır (24.2'deki bilinen fark).
    ("ref_icerik_etiket", [U("Bunu yaz: <start_of_turn> ve <end_of_turn>")], True),
    ("ref_art_arda_kullanici", [U("Merhaba"), U("Saat kaç?")], True),
    ("ref_asistanla_baslayan", [A("Merhaba."), U("Saat kaç?")], True),
    ("ref_ortada_sistem", [U("Merhaba"), A("Merhaba!"), S("Artık İngilizce konuş."), U("Hello")], True),
    ("ref_yalniz_sistem", [S(SYS)], True),
    ("ref_arac_sonucu", [U("Saat kaç?"), T("17:05")], True),
]


def run_cases(tokenizer_dir, templates, out):
    from transformers import PreTrainedTokenizerFast
    import transformers
    import tokenizers
    # Gemma'nın tokenizer sınıfı değil, aynı tokenizer.json'la genel hızlı parçalayıcı: başvuru dosyanın kendisidir.
    tok = PreTrainedTokenizerFast(tokenizer_file=os.path.join(tokenizer_dir, "tokenizer.json"), bos_token="<bos>",
                                  eos_token="<eos>", pad_token="<pad>")

    def encode(template, messages, gen):
        ids = tok.apply_chat_template(messages, tokenize=True, chat_template=template, add_generation_prompt=gen)
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
        meta.setdefault("mask_in_template", {})[label] = "<mask>" in template
        for name, messages, gen in CASES:
            row = {"name": name, "template": label, "messages": messages, "add_generation_prompt": gen}
            try:
                row["ids"] = encode(template, messages, gen)
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
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--tokenizer-out")
    ap.add_argument("--tokenizer")
    ap.add_argument("--check-ids", help="yayımlanmış tokenizer_config.json")
    ap.add_argument("--template", action="append", default=[], help="AD=DOSYA")
    ap.add_argument("--cases-out")
    a = ap.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    if a.tokenizer_out:
        if build_tokenizer(a.tokenizer_out) != 0:
            sys.exit(1)
    if a.check_ids and not check_ids(a.check_ids):
        sys.exit(1)
    if a.cases_out:
        if not (a.tokenizer and a.template):
            sys.exit("--cases-out needs --tokenizer and --template")
        run_cases(a.tokenizer, [t.split("=", 1) for t in a.template], a.cases_out)


if __name__ == "__main__":
    main()
