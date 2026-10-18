# -*- coding: utf-8 -*-
"""Bileşenlerin kullandığı karakter sınıflarını başvuru uygulamasından çıkarır.

Başvuru uygulamasının bazı bileşenleri (aksan silme, BERT temizleme ve
noktalama, rakam ayırma, sözcük karakteri, küçük harfe çevirme) farklı Unicode
sürümlerine ait tablolar kullanır. Go'nun kendi tablolarıyla yeni eklenen
karakterlerde fark çıkmaması için her sınıf, `tokenizers` kütüphanesi bütün
kod noktalarında yoklanarak çıkarılır ve ../karaktertablo.go dosyasına yazılır.

Kullanım (depo kökünden):
    python -X utf8 internal/hesap/parcalayici/testdata/karakter_tablolari_uret.py
"""

import os
import sys

from tokenizers import normalizers as nz, pre_tokenizers as pt, Regex
import tokenizers

BURASI = os.path.dirname(os.path.abspath(__file__))
HEDEF = os.path.join(os.path.dirname(BURASI), "karaktertablo.go")


def kod_noktalari():
    for c in range(0x110000):
        if not (0xD800 <= c <= 0xDFFF):
            yield c


def araliklar(kume):
    out = []
    for c in sorted(kume):
        if out and out[-1][1] == c - 1:
            out[-1][1] = c
        else:
            out.append([c, c])
    return out


def main():
    aksan = nz.StripAccents()
    bert_temiz = nz.BertNormalizer(clean_text=True, handle_chinese_chars=False, strip_accents=False, lowercase=False)
    bert_aksan = nz.BertNormalizer(clean_text=False, handle_chinese_chars=False, strip_accents=True, lowercase=False)
    nfd = nz.NFD()
    bert_pt = pt.BertPreTokenizer()
    rakam = pt.Digits(individual_digits=True)
    bosluk_pt = pt.Whitespace()
    bosluk_bol = pt.WhitespaceSplit()
    onig_w = pt.Split(Regex(r"\W"), "removed")
    onig_d = pt.Split(Regex(r"\d"), "removed")
    kucuk = nz.Lowercase()

    kumeler = {k: set() for k in ["aksan", "bertKontrol", "bertAksan", "bertNoktalama", "rakam",
                                  "sozcuk", "bosluk", "ifadeSozcuk", "ifadeRakam"]}
    kucuk_istisna = {}
    for c in kod_noktalari():
        ch = chr(c)
        s = "a" + ch + "b"
        if aksan.normalize_str(s) == "ab":
            kumeler["aksan"].add(c)
        if bert_temiz.normalize_str(s) == "ab":
            kumeler["bertKontrol"].add(c)
        if nfd.normalize_str(ch) == ch and bert_aksan.normalize_str(s) == "ab":
            kumeler["bertAksan"].add(c)
        bp = bert_pt.pre_tokenize_str(s)
        bb = bosluk_bol.pre_tokenize_str(s)
        if len(bb) == 2:
            kumeler["bosluk"].add(c)
        elif len(bp) == 3 and bp[1][0] == ch:
            kumeler["bertNoktalama"].add(c)
        if len(rakam.pre_tokenize_str(s)) == 3:
            kumeler["rakam"].add(c)
        if len(bosluk_pt.pre_tokenize_str(s)) == 1:
            kumeler["sozcuk"].add(c)
        if len(onig_w.pre_tokenize_str(s)) != 2:
            kumeler["ifadeSozcuk"].add(c)
        if len(onig_d.pre_tokenize_str(s)) == 2:
            kumeler["ifadeRakam"].add(c)
        k = kucuk.normalize_str(ch)
        if k != ch.lower():
            kucuk_istisna[c] = k
    # Harfler ve rakamlar her zaman sözcük karakteridir ('a', 'b' sınamasında görünmez).
    for c in range(ord("0"), ord("9") + 1):
        kumeler["sozcuk"].add(c)
        kumeler["ifadeSozcuk"].add(c)
    for c in list(range(ord("a"), ord("z") + 1)) + list(range(ord("A"), ord("Z") + 1)) + [ord("_")]:
        kumeler["sozcuk"].add(c)
        kumeler["ifadeSozcuk"].add(c)

    # Düzenli ifade motorunun genel kategori sınıfları (\p{L} vb.): desen
    # "removed" davranışıyla uygulanır; kalan parçaların dışında kalan
    # konumlar eşleşen karakterlerdir.
    kategoriler = ["L", "Lu", "Ll", "Lt", "Lm", "Lo", "M", "Mn", "Mc", "Me", "N", "Nd", "Nl", "No",
                   "P", "Pc", "Pd", "Ps", "Pe", "Pi", "Pf", "Po", "S", "Sm", "Sc", "Sk", "So",
                   "Z", "Zs", "Zl", "Zp", "C", "Cc", "Cf", "Co", "Cs", "Cn"]
    tum = list(kod_noktalari())
    onig_kat = {}
    for kat in kategoriler:
        bolucu = pt.Split(Regex(r"\p{%s}" % kat), "removed")
        kume = set()
        for i in range(0, len(tum), 4096):
            parca = tum[i:i + 4096]
            s = "".join(chr(c) for c in parca)
            kalan = [False] * len(parca)
            for _, (b, e) in bolucu.pre_tokenize_str(s):
                for k in range(b, e):
                    kalan[k] = True
            for k, c in enumerate(parca):
                if not kalan[k]:
                    kume.add(c)
        onig_kat[kat] = kume
    # \s (boşluk) sınıfı
    bolucu = pt.Split(Regex(r"\s"), "removed")
    kume = set()
    for i in range(0, len(tum), 4096):
        parca = tum[i:i + 4096]
        kalan = [False] * len(parca)
        for _, (b, e) in bolucu.pre_tokenize_str("".join(chr(c) for c in parca)):
            for k in range(b, e):
                kalan[k] = True
        kume.update(c for k, c in enumerate(parca) if not kalan[k])
    onig_kat["Bosluk"] = kume

    with open(HEDEF, "w", encoding="utf-8", newline="\n") as f:
        f.write("// Kod üretildi: testdata/karakter_tablolari_uret.py ile (başvuru kütüphanesi %s). ELLE DEĞİŞTİRMEYİN.\n\n"
                % tokenizers.__version__)
        f.write("package parcalayici\n\n")
        for ad, kume in kumeler.items():
            f.write("var tablo%s%s = karakterTablosu{" % (ad[0].upper(), ad[1:]))
            f.write(", ".join("{0x%X, 0x%X}" % (a, b) for a, b in araliklar(kume)))
            f.write("}\n\n")
        f.write("// ifadeKategori düzenli ifade motorunun \\p{..} genel kategori tablolarıdır.\n")
        f.write("var ifadeKategori = map[string]karakterTablosu{\n")
        for kat, kume in onig_kat.items():
            f.write('\t"%s": {%s},\n' % (kat.lower(), ", ".join("{0x%X, 0x%X}" % (a, b) for a, b in araliklar(kume))))
        f.write("}\n\n")
        f.write("// kucukHarfIstisnalari, tek karakterlik küçük harfe çevirmenin Unicode'un\n")
        f.write("// tam eşlemesinden ayrıldığı (sürüm farkı) kod noktalarıdır.\n")
        f.write("var kucukHarfIstisnalari = map[rune]string{")
        f.write(", ".join("0x%X: %s" % (c, '"' + "".join("\\U%08X" % ord(x) for x in v) + '"')
                          for c, v in sorted(kucuk_istisna.items())))
        f.write("}\n")
    for ad, kume in kumeler.items():
        print(ad, len(kume), file=sys.stderr)
    print("küçük harf istisnası", len(kucuk_istisna), file=sys.stderr)


if __name__ == "__main__":
    main()
