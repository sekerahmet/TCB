# -*- coding: utf-8 -*-
"""Unicode normalleştirme (NFC/NFD/NFKC/NFKD) tablolarını üretir.

Go standart kütüphanesinde Unicode normalleştirme verisi yoktur. Ayrıştırma
(decomposition), birleşim sınıfı (canonical combining class) ve birleştirme
bilgisi Python'un `unicodedata` modülünden alınır; ancak başvuru uygulaması
(`tokenizers`) daha eski bir Unicode sürümünün tablolarını kullandığından,
her kod noktası kütüphanede yoklanır ve yalnızca kütüphanenin de bildiği
ayrıştırmalar / birleşim sınıfları tabloya girer. Sonuç ../normtablo.go'dur.

Kullanım (depo kökünden):
    python -X utf8 internal/hesap/parca/testdata/normtablo_uret.py
"""

import os
import sys
import unicodedata

from tokenizers import normalizers as nz
import tokenizers

BURASI = os.path.dirname(os.path.abspath(__file__))
HEDEF = os.path.join(os.path.dirname(BURASI), "normtablo.go")


def main():
    NFD, NFKD, NFC = nz.NFD(), nz.NFKD(), nz.NFC()
    ayristirma = []
    for kn in range(0x110000):
        if 0xD800 <= kn <= 0xDFFF or 0xAC00 <= kn <= 0xD7A3:
            continue  # vekiller ve Hangıl heceleri (algoritmik) hariç
        c = chr(kn)
        d = unicodedata.decomposition(c)
        if not d:
            continue
        parca = d.split()
        if parca[0].startswith("<"):
            if NFKD.normalize_str(c) == c:
                continue  # kütüphane bu uyumluluk ayrıştırmasını bilmiyor
            bayrak = "k"
            parca = parca[1:]
        else:
            if NFD.normalize_str(c) == c:
                continue  # kütüphane bu kanonik ayrıştırmayı bilmiyor
            ic = "".join(chr(int(x, 16)) for x in parca)
            bayrak = "c" if len(parca) == 2 and NFC.normalize_str(ic) == c else "d"
        ayristirma.append("%X %s %s" % (kn, bayrak, " ".join(parca)))

    # Birleşim sınıfı: kütüphane karakteri sıfır sınıflı (başlangıç) sayıyorsa 0.
    # Sınama: "a" + U+0345 (240) + m, 0 < ccc(m) < 240 ise yer değiştirir;
    #         "a" + m + U+0334 (1), ccc(m) > 1 ise yer değiştirir.
    def kutuphane_sinifi(kn, s):
        m = chr(kn)
        if s < 240:
            return s if NFD.normalize_str("aͅ" + m) == "a" + m + "ͅ" else 0
        return s if NFD.normalize_str("a" + m + "̴") == "a̴" + m else 0

    siniflar = []
    bas, onceki, son = None, 0, 0
    sifirlanan = 0
    for kn in range(0x110001):
        s = 0
        if kn < 0x110000 and not (0xD800 <= kn <= 0xDFFF):
            s = unicodedata.combining(chr(kn))
            if s:
                k = kutuphane_sinifi(kn, s)
                if k == 0:
                    sifirlanan += 1
                s = k
        if bas is not None and (s != onceki or kn != son + 1):
            siniflar.append("%X %X %d" % (bas, son, onceki))
            bas = None
        if s != 0:
            if bas is None:
                bas = kn
            son = kn
            onceki = s

    with open(HEDEF, "w", encoding="utf-8", newline="\n") as f:
        f.write("// Kod üretildi: testdata/normtablo_uret.py ile (Unicode %s, başvuru kütüphanesi %s). ELLE DEĞİŞTİRMEYİN.\n\n"
                % (unicodedata.unidata_version, tokenizers.__version__))
        f.write("package parcalayici\n\n")
        f.write("// ayristirmaVerisi satır biçimi: \"<kod> <c|d|k> <kod> <kod>...\"\n")
        f.write("// c: kanonik ve birincil birleşik, d: kanonik (yeniden birleştirilmez), k: uyumluluk.\n")
        f.write("const ayristirmaVerisi = `" + "\n".join(ayristirma) + "`\n\n")
        f.write("// birlesimSinifiVerisi satır biçimi: \"<ilk> <son> <sınıf>\"\n")
        f.write("const birlesimSinifiVerisi = `" + "\n".join(siniflar) + "`\n")
    print("ayrıştırma:", len(ayristirma), "sınıf aralığı:", len(siniflar), "sıfırlanan sınıf:", sifirlanan, file=sys.stderr)


if __name__ == "__main__":
    main()
