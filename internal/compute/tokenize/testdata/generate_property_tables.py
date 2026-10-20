# -*- coding: utf-8 -*-
"""Düzenli ifade motorunun Unicode özellik sınıflarını başvuru uygulamasından çıkarır.

Desenlerdeki \\p{Latin}, \\p{Alphabetic}, [[:punct:]] gibi sınıflar, başvuru
uygulamasının düzenli ifade motorunun kendi Unicode sürümüne ve kendi sınıf
tanımlarına göre çalışır (ör. [[:punct:]] ASCII simgelerini de kapsar). Go'nun
tabloları daha eski bir Unicode sürümünü yansıttığı için her sınıf, `tokenizers`
kütüphanesinde bütün kod noktalarında yoklanarak çıkarılır ve
../ozelliktablo.go dosyasına yazılır. Başvurunun tanımadığı adlar atlanır;
motor da onları tanımaz (açık hata).

Kullanım (depo kökünden):
    python -X utf8 internal/compute/tokenize/testdata/generate_property_tables.py
"""

import os
import subprocess
import sys

from tokenizers import pre_tokenizers as pt, Regex
import tokenizers

BURASI = os.path.dirname(os.path.abspath(__file__))
HEDEF = os.path.join(os.path.dirname(BURASI), "ozelliktablo.go")

# Yazı sistemleri (uzun adları).
YAZILAR = """Adlam Ahom Anatolian_Hieroglyphs Arabic Armenian Avestan Balinese Bamum Bassa_Vah Batak Bengali
Bhaiksuki Bopomofo Brahmi Braille Buginese Buhid Canadian_Aboriginal Carian Caucasian_Albanian Chakma Cham
Cherokee Chorasmian Common Coptic Cuneiform Cypriot Cypro_Minoan Cyrillic Deseret Devanagari Dives_Akuru Dogra
Duployan Egyptian_Hieroglyphs Elbasan Elymaic Ethiopic Garay Georgian Glagolitic Gothic Grantha Greek Gujarati
Gunjala_Gondi Gurmukhi Gurung_Khema Han Hangul Hanifi_Rohingya Hanunoo Hatran Hebrew Hiragana Imperial_Aramaic
Inherited Inscriptional_Pahlavi Inscriptional_Parthian Javanese Kaithi Kannada Katakana Katakana_Or_Hiragana
Kawi Kayah_Li Kharoshthi Khitan_Small_Script Khmer Khojki Khudawadi Kirat_Rai Lao Latin Lepcha Limbu Linear_A
Linear_B Lisu Lycian Lydian Mahajani Makasar Malayalam Mandaic Manichaean Marchen Masaram_Gondi Medefaidrin
Meetei_Mayek Mende_Kikakui Meroitic_Cursive Meroitic_Hieroglyphs Miao Modi Mongolian Mro Multani Myanmar
Nabataean Nag_Mundari Nandinagari New_Tai_Lue Newa Nko Nushu Nyiakeng_Puachue_Hmong Ogham Ol_Chiki Ol_Onal
Old_Hungarian Old_Italic Old_North_Arabian Old_Permic Old_Persian Old_Sogdian Old_South_Arabian Old_Turkic
Old_Uyghur Oriya Osage Osmanya Pahawh_Hmong Palmyrene Pau_Cin_Hau Phags_Pa Phoenician Psalter_Pahlavi Rejang
Runic Samaritan Saurashtra Sharada Shavian Siddham SignWriting Sinhala Sogdian Sora_Sompeng Soyombo Sundanese
Sunuwar Syloti_Nagri Syriac Tagalog Tagbanwa Tai_Le Tai_Tham Tai_Viet Takri Tamil Tangsa Tangut Telugu Thaana
Thai Tibetan Tifinagh Tirhuta Todhri Toto Tulu_Tigalari Ugaritic Unknown Vai Vithkuqi Wancho Warang_Citi Yezidi
Yi Zanabazar_Square""".split()

# Yazı sistemlerinin dört harfli kısa adları.
YAZI_KISA = """Adlm Aghb Ahom Arab Armi Armn Avst Bali Bamu Bass Batk Beng Bhks Bopo Brah Brai Bugi Buhd Cakm Cans
Cari Cher Chrs Copt Qaac Cpmn Cprt Cyrl Deva Diak Dogr Dsrt Dupl Egyp Elba Elym Ethi Gara Geor Glag Gong Gonm
Goth Gran Grek Gujr Gukh Guru Hang Hani Hano Hatr Hebr Hira Hluw Hmng Hmnp Hrkt Hung Ital Java Kali Kana Kawi
Khar Khmr Khoj Kits Knda Krai Kthi Lana Laoo Latn Lepc Limb Lina Linb Lisu Lyci Lydi Mahj Maka Mand Mani Marc
Medf Mend Merc Mero Mlym Modi Mong Mroo Mtei Mult Mymr Nagm Nand Narb Nbat Newa Nkoo Nshu Ogam Olck Onao Orkh
Orya Osge Osma Ougr Palm Pauc Perm Phag Phli Phlp Phnx Plrd Prti Qaai Rjng Rohg Runr Samr Sarb Saur Sgnw Shaw
Shrd Sidd Sind Sinh Sogd Sogo Sora Soyo Sund Sunu Sylo Syrc Tagb Takr Tale Talu Taml Tang Tavt Telu Tfng Tglg
Thaa Thai Tibt Tirh Tnsa Todr Toto Tutg Ugar Vaii Vith Wara Wcho Xpeo Xsux Yezi Yiii Zanb Zinh Zyyy Zzzz""".split()

# İkili özellikler (uzun ve kısa adlarıyla).
OZELLIKLER = """ASCII_Hex_Digit AHex Alphabetic Alpha Bidi_Control Bidi_C Bidi_Mirrored Bidi_M Cased Case_Ignorable CI
Changes_When_Casefolded CWCF Changes_When_Casemapped CWCM Changes_When_Lowercased CWL Changes_When_NFKC_Casefolded
CWKCF Changes_When_Titlecased CWT Changes_When_Uppercased CWU Dash Default_Ignorable_Code_Point DI Deprecated Dep
Diacritic Dia Emoji Emoji_Component EComp Emoji_Modifier EMod Emoji_Modifier_Base EBase Emoji_Presentation EPres
Extended_Pictographic ExtPict Extender Ext Grapheme_Base Gr_Base Grapheme_Extend Gr_Ext Grapheme_Link Gr_Link
Hex_Digit Hex Hyphen ID_Compat_Math_Continue ID_Compat_Math_Start ID_Continue IDC ID_Start IDS IDS_Binary_Operator
IDSB IDS_Trinary_Operator IDST IDS_Unary_Operator IDSU Ideographic Ideo InCB Join_Control Join_C
Logical_Order_Exception LOE Lowercase Lower Math Modifier_Combining_Mark MCM Noncharacter_Code_Point NChar
Other_Alphabetic OAlpha Other_Default_Ignorable_Code_Point ODI Other_Grapheme_Extend OGr_Ext Other_ID_Continue
OIDC Other_ID_Start OIDS Other_Lowercase OLower Other_Math OMath Other_Uppercase OUpper Pattern_Syntax Pat_Syn
Pattern_White_Space Pat_WS Prepended_Concatenation_Mark PCM Quotation_Mark QMark Radical Regional_Indicator RI
Sentence_Terminal STerm Soft_Dotted SD Terminal_Punctuation Term Unified_Ideograph UIdeo Uppercase Upper
Variation_Selector VS White_Space WSpace Space XID_Continue XIDC XID_Start XIDS""".split()

# Genel kategoriler (kısa ve uzun adları) ve motorun ek adları.
KATEGORILER = """L Lu Ll Lt Lm Lo LC L& M Mn Mc Me N Nd Nl No P Pc Pd Ps Pe Pi Pf Po S Sm Sc Sk So Z Zs Zl Zp C Cc Cf
Co Cs Cn Letter Uppercase_Letter Lowercase_Letter Titlecase_Letter Cased_Letter Modifier_Letter Other_Letter Mark
Combining_Mark Nonspacing_Mark Spacing_Mark Enclosing_Mark Number Decimal_Number Letter_Number Other_Number
Punctuation Connector_Punctuation Dash_Punctuation Open_Punctuation Close_Punctuation Initial_Punctuation
Final_Punctuation Other_Punctuation Symbol Math_Symbol Currency_Symbol Modifier_Symbol Other_Symbol Separator
Space_Separator Line_Separator Paragraph_Separator Other Control Format Private_Use Surrogate Unassigned
Any Assigned ASCII Alnum Blank Cntrl Digit Graph Print Punct Word XDigit Newline""".split()

# generate_char_tables.py'nin ifadeKategori'ye yazdığı genel kategoriler.
KISA_KATEGORILER = """L Lu Ll Lt Lm Lo M Mn Mc Me N Nd Nl No P Pc Pd Ps Pe Pi Pf Po S Sm Sc Sk So Z Zs Zl Zp C Cc
Cf Co Cs Cn""".split()

# [[:ad:]] biçimindeki POSIX sınıfları.
POSIX = "alnum alpha ascii blank cntrl digit graph lower print punct space upper xdigit word".split()


def anahtar(ad):
    return "".join(c for c in ad.lower() if c not in " _-")


def kod_noktalari():
    for c in range(0x110000):
        if not (0xD800 <= c <= 0xDFFF):
            yield c


TUM = list(kod_noktalari())
PARCALAR = [TUM[i:i + 4096] for i in range(0, len(TUM), 4096)]
METINLER = ["".join(chr(c) for c in p) for p in PARCALAR]


def kume(desen):
    """Desenin (tek karakterlik sınıf) eşlediği kod noktaları; başvuru deseni
    tanımıyorsa None."""
    try:
        bolucu = pt.Split(Regex(desen), "removed")
    except Exception:
        return None
    out = set()
    for parca, s in zip(PARCALAR, METINLER):
        kalan = [False] * len(parca)
        for _, (b, e) in bolucu.pre_tokenize_str(s):
            for k in range(b, e):
                kalan[k] = True
        out.update(c for k, c in enumerate(parca) if not kalan[k])
    return frozenset(out)


def yorungeler():
    """Büyük/küçük harf duyarsız eşleşmede her karakterin eşdeğerleri:
    (?i)\\x{c} deseninin eşlediği (c dışındaki) karakterler. Eşdeğeri olan
    karakterler, büyük/küçük harf dönüşümüyle değişen karakterlerdir; aday
    kümesinin kapalı olduğu ayrıca doğrulanır."""
    aday = kume(r"\p{Changes_When_Casemapped}") | kume(r"\p{Changes_When_Casefolded}")
    sinif = "(?i)[%s]" % "".join(r"\x{%X}" % c for c in sorted(aday))
    kapanis = kume(sinif)
    if not kapanis <= aday:
        raise SystemExit("aday kümesi kapalı değil: %s" % sorted(hex(c) for c in kapanis - aday)[:20])
    liste = sorted(aday)
    # Karakterler arasına ayırıcı konur: çok karakterli katlamalar (ör. "ﬅ" ->
    # "st") yan yana gelen iki adayı birlikte eşlemesin.
    s = "\x01".join(chr(c) for c in liste)
    out = {}
    for c in liste:
        bolucu = pt.Split(Regex(r"(?i)\x{%X}" % c), "removed")
        kalan = [False] * len(s)
        for _, (b, e) in bolucu.pre_tokenize_str(s):
            for k in range(b, e):
                kalan[k] = True
        es = [liste[k] for k in range(len(liste)) if not kalan[2 * k] and liste[k] != c]
        if es:
            out[c] = es
    simetrik = all(c in out.get(d, []) for c, es in out.items() for d in es)
    return out, simetrik


def araliklar(k):
    out = []
    for c in sorted(k):
        if out and out[-1][1] == c - 1:
            out[-1][1] = c
        else:
            out.append([c, c])
    return out


def main():
    # Genel kategoriler karaktertablo.go'daki ifadeKategori'dedir (aynı
    # yöntemle çıkarılır); onlarla aynı kümeyi veren adlar tablo yerine
    # kategori anahtarına bağlanır.
    kategori = {}
    for kat in KISA_KATEGORILER:
        kategori.setdefault(kume(r"\p{%s}" % kat), kat.lower())

    tablolar = []          # benzersiz kümeler
    sira = {}              # küme -> tablo sırası

    def kaydet(hedef_tablo, hedef_kat, a, k):
        if k in kategori:
            hedef_kat[a] = kategori[k]
            return
        if k not in sira:
            sira[k] = len(tablolar)
            tablolar.append(k)
        hedef_tablo[a] = sira[k]

    adlar, adlar_kat = {}, {}    # \p{ad} anahtarı -> tablo sırası / kategori
    posix, posix_kat = {}, {}    # [[:ad:]] -> tablo sırası / kategori
    atlanan = []
    for ad in YAZILAR + YAZI_KISA + OZELLIKLER + KATEGORILER:
        a = anahtar(ad)
        if a in adlar or a in adlar_kat or ad in KISA_KATEGORILER:
            continue
        k = kume(r"\p{%s}" % ad)
        if k is None:
            atlanan.append(ad)
            continue
        kaydet(adlar, adlar_kat, a, k)
    for ad in POSIX:
        k = kume("[[:%s:]]" % ad)
        if k is None:
            atlanan.append("[[:%s:]]" % ad)
            continue
        kaydet(posix, posix_kat, ad, k)

    katlama, simetrik = yorungeler()
    if not simetrik:
        print("uyarı: harf katlama eşdeğerliği simetrik değil", file=sys.stderr)

    def harita(f, ad, aciklama, m, deger):
        f.write("// %s\n" % aciklama)
        f.write("var %s = map[string]%s{\n" % (ad, deger))
        for a in sorted(m):
            f.write('\t"%s": %s,\n' % (a, ('"%s"' % m[a]) if deger == "string" else m[a]))
        f.write("}\n\n")

    with open(HEDEF, "w", encoding="utf-8", newline="\n") as f:
        f.write("// Code generated by testdata/generate_property_tables.py (başvuru kütüphanesi %s); DO NOT EDIT.\n\n"
                % tokenizers.__version__)
        f.write("package parca\n\n")
        f.write("// ifadeOzellikTablolari düzenli ifade motorunun \\p{..} ve [[:..:]] sınıflarının\n")
        f.write("// (genel kategori olmayan) tablolarıdır.\n")
        f.write("var ifadeOzellikTablolari = [...]karakterTablosu{\n")
        for k in tablolar:
            f.write("\t{%s},\n" % ", ".join("{0x%X, 0x%X}" % (a, b) for a, b in araliklar(k)))
        f.write("}\n\n")
        harita(f, "ifadeOzellikAdlari", "ifadeOzellikAdlari \\p{ad} adlarının (küçük harf; boşluk, '_' ve '-' atılmış) tablo sırasıdır.",
               adlar, "uint16")
        harita(f, "ifadeOzellikKategori", "ifadeOzellikKategori bir genel kategoriyle aynı kümeyi veren \\p{ad} adlarıdır.",
               adlar_kat, "string")
        harita(f, "ifadePosixAdlari", "ifadePosixAdlari [[:ad:]] sınıflarının tablo sırasıdır.", posix, "uint16")
        harita(f, "ifadePosixKategori", "ifadePosixKategori bir genel kategoriyle aynı kümeyi veren [[:ad:]] sınıflarıdır.",
               posix_kat, "string")
        f.write("// ifadeKatlama büyük/küçük harf duyarsız eşleşmede her karakterin\n")
        f.write("// eşdeğerleridir ((?i) ile karakterin kendisi dışında eşlediği karakterler).\n")
        f.write("var ifadeKatlama = map[rune][]rune{\n")
        for c in sorted(katlama):
            f.write("\t0x%X: {%s},\n" % (c, ", ".join("0x%X" % d for d in katlama[c])))
        f.write("}\n")
    # Hizalamayı Go biçimlendiricisine bırak.
    subprocess.run(["gofmt", "-w", HEDEF], check=True)
    print("ad", len(adlar) + len(adlar_kat), "posix", len(posix) + len(posix_kat), "tablo", len(tablolar), file=sys.stderr)
    print("başvurunun tanımadığı adlar:", " ".join(atlanan), file=sys.stderr)


if __name__ == "__main__":
    main()
