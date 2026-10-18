# -*- coding: utf-8 -*-
"""Parçalayıcı paketinin sınama verisini üretir.

Bu betik Hugging Face `tokenizers` kütüphanesiyle (0.23.x) ornek_metin.txt
üzerinde KÜÇÜK, sentetik bir bayt düzeyi BPE parçalayıcısı eğitir ve aynı
kütüphanenin ürettiği "altın" çıktıları JSON olarak yazar. Go sınamaları bu
dosyalarla birebir eşleşmek zorundadır.

Kullanım (depo kökünden):
    python pkg/parcalayici/testdata/uret.py

Üretilen dosyalar:
    kucuk_parcalayici.json  - eğitilmiş parçalayıcı (tokenizer.json biçimi)
    altin.json              - metin -> id ve id -> metin altın çıktıları
"""

import json
import os
import random

from tokenizers import AddedToken, Regex, Tokenizer, decoders, models, pre_tokenizers, trainers

BURASI = os.path.dirname(os.path.abspath(__file__))

DESEN = r"[’']\p{L}+| ?\p{L}+| ?\p{N}| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+"

OZEL = [
    "<|endoftext|>", "<|pad|>", "<|system|>", "<|user|>", "<|assistant|>", "<|end|>",
    "<|tool_call|>", "<|/tool_call|>", "<|tool_result|>", "<|/tool_result|>",
] + ["<|reserved_%d|>" % i for i in range(10, 16)]


def egit():
    with open(os.path.join(BURASI, "ornek_metin.txt"), encoding="utf-8") as f:
        metin = f.read()
    satirlar = metin.split("\n")

    tok = Tokenizer(models.BPE())
    tok.pre_tokenizer = pre_tokenizers.Sequence([
        pre_tokenizers.Split(Regex(DESEN), behavior="isolated", invert=False),
        pre_tokenizers.ByteLevel(add_prefix_space=False, trim_offsets=True, use_regex=False),
    ])
    tok.decoder = decoders.ByteLevel()
    egitici = trainers.BpeTrainer(
        vocab_size=1000,
        min_frequency=1,
        special_tokens=OZEL,
        initial_alphabet=pre_tokenizers.ByteLevel.alphabet(),
        show_progress=False,
    )
    tok.train_from_iterator(satirlar, egitici)

    # Eğitimden sonra eklenen, farklı seçenekli ek tokenlar (seçenek yollarını sınamak için).
    tok.add_special_tokens([
        AddedToken("<|maske|>", lstrip=True, rstrip=True, special=True),
        AddedToken("ĠÖzel", special=True),
    ])
    tok.add_tokens([
        AddedToken("[KELİME]", single_word=True, normalized=True, special=False),
        AddedToken("çaycı", normalized=False, special=False),
        AddedToken("ğüş", normalized=True, special=False),
    ])
    return tok, satirlar, metin


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
    ]
    sonuc = []
    for _ in range(sayi):
        n = rnd.randint(1, 25)
        sonuc.append("".join(rnd.choice(parcalar) for _ in range(n)))
    # Rastgele kod noktaları (vekil çiftler hariç).
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


def main():
    tok, satirlar, metin = egit()
    tok.save(os.path.join(BURASI, "kucuk_parcalayici.json"), pretty=True)
    # Kaydedilen dosyayı yeniden yükle: altın çıktılar dosyanın kendisinden üretilsin.
    tok = Tokenizer.from_file(os.path.join(BURASI, "kucuk_parcalayici.json"))

    girdiler = [metin] + satirlar + kenar_durumlar() + rastgele_metinler(400)
    tekil = []
    gorulen = set()
    for g in girdiler:
        if g not in gorulen:
            gorulen.add(g)
            tekil.append(g)

    kodlama = []
    for g in tekil:
        tok.encode_special_tokens = False
        ids = tok.encode(g).ids
        tok.encode_special_tokens = True
        ids_duz = tok.encode(g).ids
        kodlama.append({"metin": g, "idler": ids, "idler_duz": ids_duz})
    tok.encode_special_tokens = False

    boyut = tok.get_vocab_size(with_added_tokens=True)
    rnd = random.Random(777)
    cozme = []
    diziler = [[], [0], [boyut + 3], [boyut - 1, boyut, boyut + 1000]]
    for k in kodlama[:60]:
        diziler.append(k["idler"])
    # Çok baytlı karakterleri bölen, ters sıralanan ve karışık diziler.
    for k in kodlama[:200]:
        ids = k["idler"]
        if len(ids) > 2:
            a = rnd.randint(0, len(ids) - 1)
            b = rnd.randint(a, len(ids))
            diziler.append(ids[a:b])
            diziler.append(list(reversed(ids)))
    for _ in range(300):
        n = rnd.randint(1, 20)
        diziler.append([rnd.randint(0, boyut + 5) for _ in range(n)])
    for d in diziler:
        for atla in (False, True):
            cozme.append({"idler": d, "ozel_atla": atla, "metin": tok.decode(d, skip_special_tokens=atla)})

    # Her kayıt tek satırda: dosya küçük kalır, farklar okunur olur.
    def satir(x):
        return json.dumps(x, ensure_ascii=False, separators=(",", ":"))

    with open(os.path.join(BURASI, "altin.json"), "w", encoding="utf-8", newline="\n") as f:
        f.write('{"sozluk_boyutu":%d,\n"kodlama":[\n' % boyut)
        f.write(",\n".join(satir(k) for k in kodlama))
        f.write('\n],\n"cozme":[\n')
        f.write(",\n".join(satir(c) for c in cozme))
        f.write("\n]}\n")
    print("sözlük:", boyut, "kodlama:", len(kodlama), "çözme:", len(cozme))


if __name__ == "__main__":
    main()
