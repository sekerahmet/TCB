# -*- coding: utf-8 -*-
"""Generates normalize.txt, the fixture of TestNormalizeMatchesUnicodeData.

Reference: Python's unicodedata; the header line records its Unicode version. Every line is
source;NFC;NFD;NFKC;NFKD, code points in lower-case hex, the columns of Unicode's NormalizationTest.txt. The
characters are assigned in that Unicode version, so the stability policy keeps the lines right in
later versions too.

Usage (from the repository root):
    python -X utf8 internal/lang/lexer/testdata/generate_normalize.py
"""

import os
import random
import unicodedata as u

HERE = os.path.dirname(os.path.abspath(__file__))
FORMS = ("NFC", "NFD", "NFKC", "NFKD")


def assigned(cp):
    return 0 <= cp < 0x110000 and not 0xD800 <= cp <= 0xDFFF and u.category(chr(cp)) not in ("Cn", "Co")


def hexes(s):
    return " ".join("%04x" % ord(c) for c in s)


def main():
    random.seed(1)
    characters = [chr(cp) for cp in range(0x110000) if assigned(cp)]
    marks = [c for c in characters if u.combining(c)]
    decomposing = [c for c in characters if u.decomposition(c) or 0xAC00 <= ord(c) <= 0xD7A3]
    bases = decomposing + [chr(cp) for cp in range(0x1100, 0x1200)] * 20 + list("aeiosxSıİşğ") * 20
    composites = []
    for c in characters:
        d = u.decomposition(c)
        if d and not d.startswith("<") and " " in d and u.normalize("NFC", c) == c:
            composites.append(tuple(chr(int(x, 16)) for x in d.split()))
    # Pairs whose low 16 bits are those of a composite: x/text composes them by mistake.
    truncated = []
    for a, b in composites:
        for plane in range(1, 0x11):
            for x, y in ((ord(a) & 0xFFFF | plane << 16, ord(b)), (ord(a), ord(b) & 0xFFFF | plane << 16)):
                if (x, y) != (ord(a), ord(b)) and assigned(x) and assigned(y):
                    truncated.append((chr(x), chr(y)))

    def mixed(length):
        return "".join(random.choice(bases if random.random() < 0.4 else marks) for _ in range(length))

    lines = ["", "e\u0301", "\u00E9", "\uFB01²", "\u32FF", "a\u0898\u0323", "a\u0323\u0898", "\u1100\u1161",
             "\u1100\u1161\u11A8", "\uAC00\u11A8", "\uAC01\u11A8", "\u0958", "\uFB1D", "\u212B", "\u1E9B\u0323",
             "\uFDFA", "\u0344", "\U0001D15E", "\U00011099\U000110BA", "\U00020041\u0300", "q\u0307\u0323",
             "a\u0301\u0301", "a\u034F\u0301"]
    for n in (30, 31, 32, 60, 61, 90):
        lines += ["e" + "\u0301" * n, "e" + "\u0301" * (n - 1) + "\u0323", "e" + "\u0301" * 30 + "\u034F" + "\u0301" * n]
    lines += random.sample(decomposing, 400)
    lines += [a + b for a, b in random.sample(truncated, 300)]
    lines += [a + random.choice(marks) + b for a, b in random.sample(truncated, 60)]
    lines += [a + b for a, b in random.sample(composites, 200)]
    lines += [mixed(random.randint(2, 8)) for _ in range(300)]
    lines += [random.choice(bases) + "".join(random.choice(marks) for _ in range(random.randint(31, 60))) for _ in range(20)]
    # The pair at the end sends the whole text down the character by character path.
    exact = "\U00020041\u0300"
    lines += [mixed(random.randint(2, 8)) + exact for _ in range(300)]
    lines += [s + exact for s in ("a\u0313\u0301", "a\u0323\u0301", "a\u0327\u0301\u0323")]
    with open(os.path.join(HERE, "normalize.txt"), "w", encoding="ascii", newline="\n") as f:
        f.write("# Unicode %s, Python unicodedata (generate_normalize.py)\n" % u.unidata_version)
        for s in lines:
            f.write(";".join([hexes(s)] + [hexes(u.normalize(form, s)) for form in FORMS]) + "\n")


main()
