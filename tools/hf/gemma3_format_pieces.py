"""Gemma 3'ün biçim parçalarının fikstürü: tests/data/gemma3_format_pieces/ (dil tanımı 16.2).

Gemma 3'ün yayımlanmış tokenizer.json'undan yalnız ailenin iki biçim kümesinin parçalarını alır (boşluk kümesi ve
desen kümesi; seçim kuralları internal/compute/tokenize/family.go'dakiyle aynı) ve onları gerçek kimlikleriyle küçük
bir parçalayıcının added_tokens'ına yazar. Dağarcık bu parçalar ve fikstürün testinin gerektirdiği parçalardır ("a",
"b", "▁" ve 256 bayt parçası); kimliklerin hepsi yayımlanmış dosyadakiyle aynı. Birleştirme yok, özel parça yok. Normalleştirici, ön
ayırıcı ve çözücü yayımlanmış dosyanınkiyle aynıdır. Yanına config.json (model_type gemma3_text) ve
tokenizer_config.json (tokenizer_class GemmaTokenizer) yazılır: klasör Gemma 3 ailesiyle açılır.

Betik iki kümenin sayısını ve karmasını hem kaynakta hem fikstürde yazar; family.go'nun değerlerinden ayrılırsa
durur (çıkış 1). Ağ gerekmez; kaynak dosya --source ile verilir. Belirlenimcidir: aynı kaynakla aynı baytlar.

  python -I tools/hf/gemma3_format_pieces.py --source G1B/tokenizer.json --out tests/data/gemma3_format_pieces
"""
import argparse
import hashlib
import json
import os
import re
import sys

# family.go'daki gemma3Whitespace ve gemma3Pattern (Colab 3a, 2026-11-13).
EXPECTED = {
    "whitespace": (92, "ec8b25bd363b59f46a2def2702b8327fa2ed1c50b7d8c829d73addf557ad7f1e"),
    "pattern": (59, "44b40d2140056d1c90b2ffc0ef450c18ab82b9994ead75383a8a2b4e6373e4c5"),
}
WHITESPACE = set(" ▁\t\n\r")
PATTERN = re.compile(r"^</?[a-z]+>$")
VOCAB = ["a", "b", "▁"] + ["<0x%02X>" % b for b in range(256)]


def format_sets(added):
    """family.go'nun formatSets'i: özel olmayan eklenmiş parçalardan iki küme."""
    sets = {"whitespace": [], "pattern": []}
    for t in added:
        if t.get("special") or not t["content"]:
            continue
        if set(t["content"]) <= WHITESPACE:
            sets["whitespace"].append(t)
        elif PATTERN.match(t["content"]):
            sets["pattern"].append(t)
    return sets


def set_hash(pieces):
    """family.go'nun setHash'i: kimliğe göre artan, "<kimlik>:<UTF-8 onaltılık>\\n", SHA-256."""
    h = hashlib.sha256()
    for t in sorted(pieces, key=lambda t: t["id"]):
        h.update(("%d:%s\n" % (t["id"], t["content"].encode("utf-8").hex())).encode("ascii"))
    return h.hexdigest()


def check(label, sets):
    ok = True
    for name, pieces in sets.items():
        count, digest = len(pieces), set_hash(pieces)
        want = EXPECTED[name]
        same = (count, digest) == want
        ok = ok and same
        print("%s %s: %d parça, karma %s; family.go %d %s; eşit: %s" % (label, name, count, digest, want[0], want[1], same))
    return ok


def build(source, out):
    src = json.load(open(source, encoding="utf-8"))
    sets = format_sets(src["added_tokens"])
    if not check("kaynak", sets):
        return 1
    model = dict(src["model"])
    vocab = {}
    for piece in VOCAB:
        if piece not in model["vocab"]:
            print("kaynakta dağarcık parçası yok:", json.dumps(piece))
            return 1
        vocab[piece] = model["vocab"][piece]
    pieces = sorted(sets["whitespace"] + sets["pattern"], key=lambda t: t["id"])
    clash = sorted(set(vocab.values()) & {t["id"] for t in pieces})
    if clash:
        print("dağarcık kimlikleri biçim parçalarıyla çakışıyor:", clash)
        return 1
    # Yayımlanmış dosyada eklenmiş parçalar modelin dağarcığında da aynı kimlikle durur; dağarcıkta olmayan eklenmiş
    # parçaya tokenizers yeni kimlik verir (koşu 4a: "\n\n" 108 yerine 261 oldu).
    for t in pieces:
        if model["vocab"].get(t["content"], t["id"]) != t["id"]:
            print("eklenmiş parçanın dağarcıktaki kimliği farklı:", json.dumps(t["content"]), t["id"])
            return 1
        vocab[t["content"]] = t["id"]
    model["vocab"] = dict(sorted(vocab.items(), key=lambda kv: kv[1]))
    model["merges"] = []
    model["unk_token"] = None
    tok = {
        "version": src.get("version", "1.0"),
        "truncation": None,
        "padding": None,
        "added_tokens": pieces,
        "normalizer": src.get("normalizer"),
        "pre_tokenizer": src.get("pre_tokenizer"),
        "post_processor": None,
        "decoder": src.get("decoder"),
        "model": model,
    }
    os.makedirs(out, exist_ok=True)
    files = {
        "tokenizer.json": tok,
        "config.json": {"model_type": "gemma3_text"},
        "tokenizer_config.json": {"tokenizer_class": "GemmaTokenizer"},
    }
    for name, value in files.items():
        with open(os.path.join(out, name), "w", encoding="utf-8", newline="") as f:
            json.dump(value, f, ensure_ascii=False, indent=2)
            f.write("\n")
    again = format_sets(json.load(open(os.path.join(out, "tokenizer.json"), encoding="utf-8"))["added_tokens"])
    if not check("fikstür", again):
        return 1
    for name in files:
        data = open(os.path.join(out, name), "rb").read()
        print("%s %d bayt sha256 %s" % (name, len(data), hashlib.sha256(data).hexdigest()))
    print("normalizer", json.dumps(tok["normalizer"], ensure_ascii=False))
    print("pre_tokenizer", json.dumps(tok["pre_tokenizer"], ensure_ascii=False))
    print("decoder", json.dumps(tok["decoder"], ensure_ascii=False))
    print("dağarcık", json.dumps(model["vocab"], ensure_ascii=False)[:200], "…")
    return 0


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--source", required=True, help="Gemma 3'ün yayımlanmış tokenizer.json'u")
    ap.add_argument("--out", required=True, help="fikstür klasörü")
    a = ap.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    sys.exit(build(a.source, a.out))


if __name__ == "__main__":
    main()
