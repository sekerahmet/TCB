import sys, time

def ms(t): return int((time.perf_counter() - t) * 1000)

def dongu():
    t = time.perf_counter(); toplam = 0; i = 0
    while i < 3000000:
        toplam += i; i += 1
    print("dongu", toplam, ms(t))

def fib(n): return n if n < 2 else fib(n - 1) + fib(n - 2)

def fib_():
    t = time.perf_counter(); s = fib(27); print("fib", s, ms(t))

def sirala():
    t = time.perf_counter(); x = 42; l = []; i = 0
    while i < 300000:
        x = (x * 1103515245 + 12345) % 2147483648; l.append(x); i += 1
    s = sorted(l); print("sirala", s[0] + s[150000] + s[299999], ms(t))

def metin():
    t = time.perf_counter(); toplam = 0; i = 0
    while i < 200000:
        s = "kelime" + str(i); toplam += len(s); i += 1
    print("metin", toplam, ms(t))

def sozluk():
    t = time.perf_counter(); d = {}; i = 0
    while i < 200000:
        d["k" + str(i)] = i; i += 1
    toplam = 0; i = 0
    while i < 200000:
        toplam += d["k" + str(i)]; i += 1
    print("sozluk", toplam, ms(t))

def matris_dongu():
    n = 120
    a = [[(i * j) % 7 for j in range(n)] for i in range(n)]
    b = [[(i + j) % 5 for j in range(n)] for i in range(n)]
    t = time.perf_counter(); c = []; i = 0
    while i < n:
        satir = []; j = 0
        while j < n:
            s = 0; k = 0
            while k < n:
                s += a[i][k] * b[k][j]; k += 1
            satir.append(s); j += 1
        c.append(satir); i += 1
    print("matris_dongu", sum(c[i][i] for i in range(n)), ms(t))

def matris_yerlesik():
    import numpy as np
    n = 512; a = np.random.rand(n, n); b = np.random.rand(n, n)
    t = time.perf_counter(); c = a @ b; c2 = c @ b
    print("matris_yerlesik", "-", ms(t))

{"dongu": dongu, "fib": fib_, "sirala": sirala, "metin": metin, "sozluk": sozluk,
 "matris_dongu": matris_dongu, "matris_yerlesik": matris_yerlesik}[sys.argv[1]]()
