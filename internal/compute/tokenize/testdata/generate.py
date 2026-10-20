# -*- coding: utf-8 -*-
"""Parçalayıcı paketinin sınama verisini üretir.

Hugging Face `tokenizers` kütüphanesiyle (0.23.x, yerel, ağ bağlantısız)
ornek_metin.txt üzerinde KÜÇÜK, sentetik parçalayıcılar eğitir ve aynı
kütüphanenin ürettiği "altın" çıktıları yazar. Go sınamaları bunlarla birebir
eşleşmek zorundadır.

Kullanım (depo kökünden):
    python -X utf8 internal/compute/tokenize/testdata/uret.py

Üretilen dosyalar:
    girdiler.json            - bütün sınamaların ortak girdi metinleri
    p_<ad>.json              - eğitilmiş parçalayıcılar (tokenizer.json biçimi)
    altin_<ad>.json          - her parçalayıcı için kodlama/çözme altın çıktıları
    bilesenler.json          - tek tek bileşenlerin (normalleştirici, ön bölücü,
                               çözücü) altın çıktıları
"""

import json
import os
import random

from tokenizers import (AddedToken, Regex, Tokenizer, decoders, models, normalizers as nz,
                        pre_tokenizers as pt, processors, trainers)

BURASI = os.path.dirname(os.path.abspath(__file__))

DESEN = r"[’']\p{L}+| ?\p{L}+| ?\p{N}| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+"
LLAMA3 = (r"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*"
          r"|\s*[\r\n]+|\s+(?!\S)|\s+")

OZEL = [
    "<|endoftext|>", "<|pad|>", "<|system|>", "<|user|>", "<|assistant|>", "<|end|>",
    "<|tool_call|>", "<|/tool_call|>", "<|tool_result|>", "<|/tool_result|>",
] + ["<|reserved_%d|>" % i for i in range(10, 16)]


def satir(x):
    return json.dumps(x, ensure_ascii=False, separators=(",", ":"))


def dosya_yaz(ad, metin):
    with open(os.path.join(BURASI, ad), "w", encoding="utf-8", newline="\n") as f:
        f.write(metin)


def liste_yaz(ad, nesne_alanlari):
    """{"a": [...], "b": [...]} biçimini her öğe tek satırda olacak şekilde yazar."""
    parcalar = []
    for anahtar, deger in nesne_alanlari.items():
        if isinstance(deger, list):
            ic = ",\n".join(satir(x) for x in deger)
            parcalar.append('%s:[\n%s\n]' % (json.dumps(anahtar), ic))
        else:
            parcalar.append("%s:%s" % (json.dumps(anahtar), satir(deger)))
    dosya_yaz(ad, "{\n" + ",\n".join(parcalar) + "\n}\n")


def ornek_satirlar():
    with open(os.path.join(BURASI, "ornek_metin.txt"), encoding="utf-8") as f:
        metin = f.read()
    return metin, metin.split("\n")


def kenar_durumlar():
    return [
        "", " ", "  ", "   ", "\t", "\n", "\n\n", "\r\n", " \n ", "\t \t", "a", " a", "a ", "a  b",
        "a \n b", "a\n\nb", "a \t\n  b", "Ankara'ya", "Ankara’ya", "3'te", " 3'te", "'", "’", "''", "'a",
        " 'ya", "!’ya", "a'1", "x'y'z", "İıIi ĞğÜüŞşÖöÇç", "ğüşıöçİ", "2024", " 2024", "1.250,75",
        "%12,5", "₺450", "😀", " 😀😀", "👍🏽", "🇹🇷", "❤️", "é", "café", " ", "a b",
        "a　b", "a b", "a b", "\u0085", "a​b", "\u000b\u000c", "x\u000by",
        "<|endoftext|>", "a<|endoftext|>b", "<|user|>Merhaba<|end|><|assistant|>Selam!<|end|>",
        "<|end", "<|end|", "<|<|end|>|>", "<|endoftext|><|endoftext|>", " <|pad|> ",
        "metin <|maske|>   devam", "  <|maske|>", "<|maske|>  ", "a<|maske|>b",
        "[KELİME]", " [KELİME] ", "a[KELİME]", "[KELİME]b", "_[KELİME]_", "1[KELİME]", "ğ[KELİME]",
        "²[KELİME]", "[KELİME]½", "‍[KELİME]", "‿[KELİME]", "Ⅻ[KELİME]", "[KELİME]ͅ",
        "́[KELİME]", "٣[KELİME]", "-[KELİME].", "[KELİME][KELİME]",
        "çaycı", "çaycılar", "Çaycı", "ğüş", "ağüşa", "ĠÖzel", "Özel", " Özel",
        "Sekme\tile\tayrılmış", "son boşluk   ", "   baş boşluk", "çok    boşluk    var",
        "a" * 300, " " * 300, "!" * 300, "=" * 1000 + "x", "ğ" * 200,
        "http://ornek.com/a?b=c&d=e", "e-posta@alan.com", "x_y-z.w", "(a)[b]{c}",
        "Merhaba dünya!\nNasılsın?\n\tİyiyim.   \n",
        # Normalleştirme ve büyük/küçük harf durumları
        "ŞEHİR İÇİ IŞIK", "ΣΊΣΥΦΟΣ", "Straße", "ﬁ ﬂ ﬀ", "Ⅻ ① ² ½", "ｆｕｌｌ ｗｉｄｔｈ", "㍿ ㌀",
        "가각힣", "각", "Việt Nam", "ậ", "ậ", "ÅΩ",
        "क़क़", "é́", "ཱི̈́", "Å", "ǅ ǆ Ǆ", "İ̇", "İ",
        "it's I'M we'LL they'Re", "ﬃx", "Ω≈ç√∫", "中文字符测试", "日本語テキスト", "한국어 텍스트",
        "Привет мир", "مرحبا بالعالم", "שלום", "สวัสดี", "­ yumuşak tire",
        "don't do not . ?", "## ##a", "</w>", "▁ ▁▁ a▁b", "<0x41>", "<unk>", "[UNK]",
        "abc123def4567", "12345678", "x1y22z333", "\r\r\n\n", " \r\n x", "\n\r",
    ]


def rastgele_metinler(sayi, tohum=12345):
    rnd = random.Random(tohum)
    parcalar = [
        "ev", "evler", "kitap", "İstanbul", "Ankara", "ışık", "Öğretmen", "çiçek", "güneş", "şehir",
        "'da", "'ya", "’nin", "’e", "'", "’", " ", "  ", "   ", "\t", "\n", "\r", " ", "　",
        "0", "7", "42", "3,14", "2024", "½", "²", "٣", ".", ",", "!", "?", "...", "—", "-", "(", ")",
        "\"", "«", "»", "“", "”", "😀", "🎉", "👍🏽", "🇹🇷", "́", "‍", "ß", "ﬁ", "Ω", "ж", "中文",
        "ا", "<|end|>", "<|user|>", "<|", "|>", "<|maske|>", "[KELİME]", "çaycı", "ğüş", "_", "#", "$",
        "@", "€", "₺", "%", "&", "*", "+", "=", "/", "\\", "~", "^", "`", "|", "<", ">", "{", "}",
        "a", "b", "z", "A", "Z", "ğ", "ü", "ş", "ı", "ö", "ç", "İ", "Ğ", "Ü", "Ş", "Ö", "Ç", "â", "î", "û",
        "'S", "'ll", "'RE", "▁", "##", "가", "é", "Å", "ǅ",
    ]
    sonuc = []
    for _ in range(sayi):
        n = rnd.randint(1, 25)
        sonuc.append("".join(rnd.choice(parcalar) for _ in range(n)))
    for _ in range(sayi // 4):
        n = rnd.randint(1, 12)
        s = []
        for _ in range(n):
            while True:
                kn = rnd.choice([rnd.randint(0, 0x7F), rnd.randint(0x80, 0x7FF), rnd.randint(0x800, 0xFFFF),
                                 rnd.randint(0x10000, 0x10FFFF)])
                if not (0xD800 <= kn <= 0xDFFF):
                    break
            s.append(chr(kn))
        sonuc.append("".join(s))
    return sonuc


# ---------------------------------------------------------------------------
# Parçalayıcılar
# ---------------------------------------------------------------------------

def ek_tokenlar(tok):
    tok.add_special_tokens([
        AddedToken("<|maske|>", lstrip=True, rstrip=True, special=True),
        AddedToken("ĠÖzel", special=True),
    ])
    tok.add_tokens([
        AddedToken("[KELİME]", single_word=True, normalized=True, special=False),
        AddedToken("çaycı", normalized=False, special=False),
        AddedToken("ğüş", normalized=True, special=False),
    ])


def bayt_bpe(satirlar):
    """(a) Kullanıcınınki gibi: Split(desen) + ByteLevel, BPE, ByteLevel çözücü."""
    tok = Tokenizer(models.BPE())
    tok.pre_tokenizer = pt.Sequence([
        pt.Split(Regex(DESEN), behavior="isolated", invert=False),
        pt.ByteLevel(add_prefix_space=False, trim_offsets=True, use_regex=False),
    ])
    tok.decoder = decoders.ByteLevel()
    tok.train_from_iterator(satirlar, trainers.BpeTrainer(
        vocab_size=1000, min_frequency=1, special_tokens=OZEL,
        initial_alphabet=pt.ByteLevel.alphabet(), show_progress=False))
    ek_tokenlar(tok)
    return tok


def bayt_yedekli(json_metni, bilinmeyen="<unk>"):
    """Bir BPE parçalayıcısının sözlüğüne <0x00>..<0xFF> ekleyip byte_fallback'i açar."""
    d = json.loads(json_metni)
    v = d["model"]["vocab"]
    n = max(v.values()) + 1
    for b in range(256):
        k = "<0x%02X>" % b
        if k not in v:
            v[k] = n
            n += 1
    d["model"]["byte_fallback"] = True
    d["model"]["unk_token"] = bilinmeyen
    # Sonradan eklenen ek tokenların kimlikleri sözlük boyutundan başlar.
    for t in d["added_tokens"]:
        if t["content"] not in v:
            t["id"] = n
            n += 1
    return json.dumps(d, ensure_ascii=False)


def metaspace_bpe(satirlar):
    """(b) Metaspace ön bölücü + Metaspace çözücü, byte_fallback=true BPE."""
    tok = Tokenizer(models.BPE(unk_token="<unk>"))
    tok.normalizer = nz.NFKC()
    tok.pre_tokenizer = pt.Metaspace(replacement="▁", prepend_scheme="always", split=True)
    tok.decoder = decoders.Metaspace(replacement="▁", prepend_scheme="always", split=True)
    tok.train_from_iterator(satirlar, trainers.BpeTrainer(
        vocab_size=700, min_frequency=2, special_tokens=["<unk>", "<s>", "</s>"],
        limit_alphabet=70, show_progress=False))
    tok = Tokenizer.from_str(bayt_yedekli(tok.to_str()))
    return tok


def llama_tarzi(metaspace_tok):
    """(b2) Aynı model, Llama-2 tarzı zincir: Prepend+Replace normalleştirici,
    ön bölücü yok, Replace+ByteFallback+Fuse+Strip çözücü, BOS ekleyen şablon."""
    d = json.loads(metaspace_tok.to_str())
    d["normalizer"] = json.loads(nz.Sequence([nz.Prepend("▁"), nz.Replace(" ", "▁")]).__getstate__())
    d["pre_tokenizer"] = None
    d["decoder"] = json.loads(decoders.Sequence([
        decoders.Replace("▁", " "), decoders.ByteFallback(), decoders.Fuse(), decoders.Strip(content=" ", left=1, right=0),
    ]).__getstate__())
    tok = Tokenizer.from_str(json.dumps(d, ensure_ascii=False))
    tok.post_processor = processors.TemplateProcessing(
        single="<s> $A", pair="<s> $A <s> $B:1", special_tokens=[("<s>", tok.token_to_id("<s>"))])
    return tok


def kelime_bpe(satirlar):
    """(c) Whitespace ön bölücü, Lowercase normalleştirici, sözcük sonu ekli BPE,
    BPEDecoder ve BOS/EOS ekleyen TemplateProcessing."""
    tok = Tokenizer(models.BPE(unk_token="[UNK]", end_of_word_suffix="</w>"))
    tok.normalizer = nz.Sequence([nz.NFC(), nz.Lowercase(), nz.Strip()])
    tok.pre_tokenizer = pt.Whitespace()
    tok.decoder = decoders.BPEDecoder(suffix="</w>")
    tok.train_from_iterator(satirlar, trainers.BpeTrainer(
        vocab_size=800, min_frequency=1, special_tokens=["[UNK]", "<s>", "</s>", "<pad>"],
        end_of_word_suffix="</w>", limit_alphabet=80, show_progress=False))
    tok.post_processor = processors.TemplateProcessing(
        single="<s> $A </s>", pair="<s> $A </s> $B:1 </s>:1",
        special_tokens=[("<s>", tok.token_to_id("<s>")), ("</s>", tok.token_to_id("</s>"))])
    return tok


def onekli_bpe(satirlar):
    """(c2) Alt sözcük önekli (##) BPE, WhitespaceSplit + Digits, NFKD + StripAccents."""
    tok = Tokenizer(models.BPE(unk_token="[UNK]", continuing_subword_prefix="##"))
    tok.normalizer = nz.Sequence([nz.NFKD(), nz.StripAccents()])
    tok.pre_tokenizer = pt.Sequence([pt.WhitespaceSplit(), pt.Digits(individual_digits=True), pt.Punctuation()])
    tok.decoder = decoders.WordPiece(prefix="##")
    tok.train_from_iterator(satirlar, trainers.BpeTrainer(
        vocab_size=700, min_frequency=1, special_tokens=["[UNK]"], continuing_subword_prefix="##",
        limit_alphabet=70, show_progress=False))
    return tok


def wordpiece(satirlar):
    """(d) BERT tarzı: BertNormalizer, BertPreTokenizer, WordPiece, BertProcessing."""
    tok = Tokenizer(models.WordPiece(unk_token="[UNK]"))
    tok.normalizer = nz.BertNormalizer(lowercase=False)
    tok.pre_tokenizer = pt.BertPreTokenizer()
    tok.decoder = decoders.WordPiece()
    tok.train_from_iterator(satirlar, trainers.WordPieceTrainer(
        vocab_size=700, special_tokens=["[PAD]", "[UNK]", "[CLS]", "[SEP]", "[MASK]"], show_progress=False))
    tok.post_processor = processors.BertProcessing(("[SEP]", tok.token_to_id("[SEP]")), ("[CLS]", tok.token_to_id("[CLS]")))
    return tok


def unigram(satirlar):
    """(e) Unigram: NFKC, Metaspace, Unigram modeli, Metaspace çözücü."""
    tok = Tokenizer(models.Unigram())
    tok.normalizer = nz.NFKC()
    tok.pre_tokenizer = pt.Metaspace()
    tok.decoder = decoders.Metaspace()
    tok.train_from_iterator(satirlar, trainers.UnigramTrainer(
        vocab_size=500, special_tokens=["<unk>", "<s>", "</s>"], unk_token="<unk>", show_progress=False))
    tok.post_processor = processors.RobertaProcessing(("</s>", tok.token_to_id("</s>")), ("<s>", tok.token_to_id("<s>")))
    return tok


def gpt2_tarzi(satirlar):
    """(f) GPT-2 tarzı: ByteLevel(add_prefix_space, use_regex) ön bölücü."""
    tok = Tokenizer(models.BPE())
    tok.pre_tokenizer = pt.ByteLevel(add_prefix_space=True, use_regex=True)
    tok.decoder = decoders.ByteLevel()
    tok.post_processor = processors.ByteLevel(trim_offsets=True)
    tok.train_from_iterator(satirlar, trainers.BpeTrainer(
        vocab_size=900, special_tokens=["<|endoftext|>"], initial_alphabet=pt.ByteLevel.alphabet(),
        show_progress=False))
    return tok


def llama3_tarzi(gpt2_tok):
    """(g) Aynı model, Llama-3 tarzı Split deseni + ByteLevel(use_regex=false)."""
    d = json.loads(gpt2_tok.to_str())
    d["pre_tokenizer"] = json.loads(pt.Sequence([
        pt.Split(Regex(LLAMA3), behavior="isolated"),
        pt.ByteLevel(add_prefix_space=False, use_regex=False),
    ]).__getstate__())
    d["normalizer"] = json.loads(nz.NFC().__getstate__())
    return Tokenizer.from_str(json.dumps(d, ensure_ascii=False))


# ---------------------------------------------------------------------------
# Altın çıktılar
# ---------------------------------------------------------------------------

def altin_uret(tok, girdiler, rnd):
    kodlama = []
    for i, g in enumerate(girdiler):
        tok.encode_special_tokens = False
        e = tok.encode(g)
        ozelsiz = tok.encode(g, add_special_tokens=False).ids
        tok.encode_special_tokens = True
        duz = tok.encode(g).ids
        tok.encode_special_tokens = False
        kodlama.append([i, e.ids, duz, ozelsiz])
    cift = []
    for _ in range(60):
        a, b = rnd.randrange(len(girdiler)), rnd.randrange(len(girdiler))
        e = tok.encode(girdiler[a], girdiler[b])
        e2 = tok.encode(girdiler[a], girdiler[b], add_special_tokens=False)
        cift.append([a, b, e.ids, e.type_ids, e.special_tokens_mask, e2.ids, e2.type_ids])
    boyut = tok.get_vocab_size(with_added_tokens=True)
    diziler = [[], [0], [boyut + 3], [boyut - 1, boyut, boyut + 1000]]
    for k in kodlama[:60]:
        ids = k[1]
        diziler.append(ids)
        if len(ids) > 2:
            a = rnd.randint(0, len(ids) - 1)
            diziler.append(ids[a:rnd.randint(a, len(ids))])
            diziler.append(list(reversed(ids)))
    for _ in range(120):
        diziler.append([rnd.randint(0, boyut + 5) for _ in range(rnd.randint(1, 20))])
    cozme = []
    for d in diziler:
        cozme.append([d, tok.decode(d, skip_special_tokens=False), tok.decode(d, skip_special_tokens=True)])
    return {"sozluk_boyutu": boyut, "kodlama": kodlama, "cift": cift, "cozme": cozme}


def bilesen_altin(girdiler):
    """Tek tek bileşenlerin altın çıktıları."""
    normlar = [nz.NFC(), nz.NFD(), nz.NFKC(), nz.NFKD(), nz.Lowercase(), nz.StripAccents(),
               nz.Strip(left=True, right=False), nz.Strip(left=False, right=True), nz.Nmt(),
               nz.Replace(Regex(r"\s+"), " "), nz.Replace("ı", "i"), nz.Prepend("▁"), nz.ByteLevel(),
               nz.BertNormalizer(), nz.BertNormalizer(clean_text=True, handle_chinese_chars=False, strip_accents=False, lowercase=False),
               nz.Sequence([nz.NFD(), nz.StripAccents(), nz.Lowercase()])]
    onbolucular = [
        pt.Whitespace(), pt.WhitespaceSplit(), pt.BertPreTokenizer(), pt.Punctuation(),
        pt.Punctuation(behavior="removed"), pt.Punctuation(behavior="merged_with_previous"),
        pt.Punctuation(behavior="merged_with_next"), pt.Punctuation(behavior="contiguous"),
        pt.Digits(individual_digits=True), pt.Digits(individual_digits=False), pt.CharDelimiterSplit("a"),
        pt.Metaspace(), pt.Metaspace(prepend_scheme="first"), pt.Metaspace(prepend_scheme="never"),
        pt.Metaspace(split=False), pt.ByteLevel(), pt.ByteLevel(add_prefix_space=False),
        pt.ByteLevel(add_prefix_space=False, use_regex=False),
        pt.Sequence([pt.Split(Regex(r"\d"), "isolated"), pt.Metaspace(prepend_scheme="first")]),
        pt.Sequence([pt.Split(Regex(r"\d"), "isolated"), pt.ByteLevel(add_prefix_space=True, use_regex=False)]),
    ]
    desenler = [
        DESEN, LLAMA3,
        r"(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+",
        r"\d+(?=\D)", r"(?<=a)b", r"(?<!x)y+?", r"[[:alpha:]]+", r"\b\w+\b", r"^\s*\S+$", r"(?>a+)b", r"a++b",
        r"x{2,3}?", r"\p{Lu}\p{Ll}*", r"\p{Han}+|\p{Greek}+|\p{Cyrillic}+", r"[^\p{L}\s]+", r".", r"\h+",
        r"(?i)ışık|İSTANBUL|straße", r"[a-zçğıöşü&&[^aeıioöuü]]+", r"\s", r"\S+", r"\W+",
        r"a*", r"(?:ab|a)(?:bc|c)", r"\p{P}+", r"\p{^L}+", r"[\x{1F600}-\x{1F64F}]", r"ç|\x41",
        r"(\p{L})(?=\p{N})", r"$", r"^", r"(?m:a.b)", r"[^\n]+", r"\A.|.\z", r"\.\.\.|…",
    ]
    for d in desenler:
        for dav in ["isolated", "removed", "merged_with_previous", "merged_with_next", "contiguous"]:
            if d in (DESEN, LLAMA3) or dav == "isolated" or d in (r"\d+(?=\D)", r"\s", r"\p{P}+", r"a*"):
                onbolucular.append(pt.Split(Regex(d), dav))
        if d in (r"\s", r"\p{P}+", r"\d+(?=\D)"):
            onbolucular.append(pt.Split(Regex(d), "removed", invert=True))
            onbolucular.append(pt.Split(Regex(d), "isolated", invert=True))
    onbolucular.append(pt.Split(" ", "removed"))
    onbolucular.append(pt.Split("ğ", "merged_with_next"))
    onbolucular.append(pt.Split("ab", "contiguous"))

    kayit = {"normalizer": [], "pre_tokenizer": [], "decoder": []}
    for n in normlar:
        kayit["normalizer"].append([json.loads(n.__getstate__()), [n.normalize_str(g) for g in girdiler]])
    for o in onbolucular:
        kayit["pre_tokenizer"].append([json.loads(o.__getstate__()), [[p for p, _ in o.pre_tokenize_str(g)] for g in girdiler]])
    cozuculer = [decoders.ByteLevel(), decoders.Metaspace(), decoders.Metaspace(prepend_scheme="never"),
                 decoders.WordPiece(), decoders.WordPiece(cleanup=False), decoders.BPEDecoder(),
                 decoders.ByteFallback(), decoders.Fuse(), decoders.Strip(content=" ", left=1, right=0),
                 decoders.Strip(content="a", left=2, right=0), decoders.Strip(content="b", left=0, right=2), decoders.Replace("▁", " "),
                 decoders.Sequence([decoders.Replace("▁", " "), decoders.ByteFallback(), decoders.Fuse(),
                                    decoders.Strip(content=" ", left=1, right=0)])]
    rnd = random.Random(99)
    havuz = ["a", "▁", "▁a", "##b", "c</w>", "<0x41>", "<0xC4>", "<0x9F>", "<0xFF>", " ", "  x ", "Ġ", "ĠdÃ¼nya",
             "Ã", "aa", "do", "n't", ".", "?", "'s", "▁▁b", "ğ", "😀", "</w>", "##", "<0xZZ>", "<0x4>"]
    token_listeleri = [[], ["a"], ["▁▁a▁b", "▁c", "▁"]] + [[rnd.choice(havuz) for _ in range(rnd.randint(1, 8))] for _ in range(150)]
    for c in cozuculer:
        kayit["decoder"].append([json.loads(c.__getstate__()), token_listeleri, [c.decode(t) for t in token_listeleri]])
    return kayit


def main():
    metin, satirlar = ornek_satirlar()
    girdiler = []
    gorulen = set()
    for g in [metin] + satirlar + kenar_durumlar() + rastgele_metinler(160):
        if g not in gorulen:
            gorulen.add(g)
            girdiler.append(g)
    liste_yaz("girdiler.json", {"girdiler": girdiler})

    ms = metaspace_bpe(satirlar)
    g2 = gpt2_tarzi(satirlar)
    parcalayicilar = {
        "bayt_bpe": bayt_bpe(satirlar),
        "metaspace_bpe": ms,
        "llama_tarzi": llama_tarzi(ms),
        "kelime_bpe": kelime_bpe(satirlar),
        "onekli_bpe": onekli_bpe(satirlar),
        "wordpiece": wordpiece(satirlar),
        "unigram": unigram(satirlar),
        "gpt2_tarzi": g2,
        "llama3_tarzi": llama3_tarzi(g2),
    }
    for ad, tok in parcalayicilar.items():
        yol = os.path.join(BURASI, "p_%s.json" % ad)
        tok.save(yol, pretty=True)
        tok = Tokenizer.from_file(yol)  # altın çıktılar kaydedilen dosyadan üretilsin
        liste_yaz("altin_%s.json" % ad, altin_uret(tok, girdiler, random.Random(777)))
        print(ad, tok.get_vocab_size(with_added_tokens=True))

    kayit = bilesen_altin(girdiler[1:140])  # tam metin hariç, dosya küçük kalsın
    satirlar_ = []
    for tur, liste in kayit.items():
        for x in liste:
            satirlar_.append(satir([tur] + x))
    dosya_yaz("bilesenler.json", "[\n" + ",\n".join(satirlar_) + "\n]\n")


if __name__ == "__main__":
    main()
