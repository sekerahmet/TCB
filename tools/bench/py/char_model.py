"""examples/07_karakter_modeli.tr ile aynı model ve eğitim (karşılaştırma için).

Aynı veri, aynı karakter sözlüğü sırası, aynı boyutlar (V, T=64, D=64, H=4, 2 katman),
aynı toplu iş örnekleme (aynı sözde rastgele üreteç) ve aynı iyileştirici ayarları.
Kullanım (proje kökünden):  python tools/bench/py/karakter_modeli.py [adım] [iş parçacığı]
"""
import sys, time
import torch
import torch.nn.functional as F

ADIM = int(sys.argv[1]) if len(sys.argv) > 1 else 300
if len(sys.argv) > 2:
    torch.set_num_threads(int(sys.argv[2]))
torch.manual_seed(1)

# examples/07_karakter_modeli.tr'deki BELGELER listesiyle aynı dosyalar, aynı sırayla.
BELGELER = [f"docs/guide/{a}.md" for a in (
    "01_baslangic", "02_anahtar_kelimeler", "03_zincirleme_metotlar", "04_yapay_zeka_ve_veri",
    "05_hata_ayiklayici", "06_vscode_kisayollari", "07_testler", "08_bicimlendirme")] + [
    f"docs/spec/{a}.md" for a in (
    "01_program", "02_sozcuk_yapisi", "03_tipler", "04_operatorler", "05_deyimler",
    "06_fonksiyonlar", "07_yapilar", "08_hata_yonetimi", "09_ertele", "10_eszamanlilik",
    "11_moduller", "12_testler", "13_yerlesikler", "14_sinirlamalar", "15_tensorler",
    "16_veri_parcalayici", "17_kodlar")]
metin = "".join(open(b, encoding="utf-8").read() for b in BELGELER)
harfler, harf_no, veri = [], {}, []
for h in metin:
    if h not in harf_no:
        harf_no[h] = len(harfler)
        harfler.append(h)
    veri.append(harf_no[h])
veri_t = torch.tensor(veri)

V, T, D, H, Dh, B, KATMAN = len(harfler), 64, 64, 4, 16, 16, 2
parametreler = []


def parametre(*sekil, olcek):
    p = (torch.randn(*sekil) * olcek).requires_grad_()
    parametreler.append(p)
    return p


def sabit(n, deger):
    p = torch.full((n,), float(deger), requires_grad=True)
    parametreler.append(p)
    return p


E, P = parametre(V, D, olcek=0.1), parametre(T, D, olcek=0.1)
katmanlar = []
for _ in range(KATMAN):
    katmanlar.append(dict(
        Wq=parametre(D, D, olcek=0.1), Wk=parametre(D, D, olcek=0.1), Wv=parametre(D, D, olcek=0.1),
        Wo=parametre(D, D, olcek=0.1), W1=parametre(D, 4 * D, olcek=0.1), W2=parametre(4 * D, D, olcek=0.05),
        g1=sabit(D, 1), s1=sabit(D, 0), g2=sabit(D, 1), s2=sabit(D, 0)))
gs, ss = sabit(D, 1), sabit(D, 0)
Wu = parametre(D, V, olcek=0.1)
print(f"veri: {len(veri)} karakter, sözlük: {V} karakter, parametre: {sum(p.numel() for p in parametreler)}")

maske = torch.triu(torch.full((T, T), -1e9), diagonal=1)
konum = torch.arange(T)


def ileri(g, b):
    x = E[g] + P[konum]
    for L in katmanlar:
        a = F.layer_norm(x, (D,), L["g1"], L["s1"])
        q = (a @ L["Wq"]).view(b, T, H, Dh).permute(0, 2, 1, 3)
        k = (a @ L["Wk"]).view(b, T, H, Dh).permute(0, 2, 1, 3)
        v = (a @ L["Wv"]).view(b, T, H, Dh).permute(0, 2, 1, 3)
        dikkat = torch.softmax(q @ k.transpose(-1, -2) * 0.25 + maske, dim=-1)
        y = (dikkat @ v).permute(0, 2, 1, 3).reshape(b, T, D)
        x = x + y @ L["Wo"]
        m = F.layer_norm(x, (D,), L["g2"], L["s2"])
        x = x + F.gelu(m @ L["W1"], approximate="tanh") @ L["W2"]
    return F.layer_norm(x, (D,), gs, ss) @ Wu


tohum = 12345


def rastgele_tam(n):
    global tohum
    tohum = (tohum * 1103515245 + 12345) % 2147483648
    return tohum % n


def toplu_is():
    gir, hed = [], []
    for _ in range(B):
        bas = rastgele_tam(len(veri) - T - 1)
        gir.append(veri[bas:bas + T])
        hed += veri[bas + 1:bas + T + 1]
    return torch.tensor(gir), torch.tensor(hed)


opt = torch.optim.AdamW(parametreler, lr=0.003, weight_decay=0.0)
bas = time.perf_counter()
for adim in range(ADIM):
    g, h = toplu_is()
    opt.zero_grad()
    kayip = F.cross_entropy(ileri(g, B).reshape(-1, V), h)
    kayip.backward()
    opt.step()
    if adim % 50 == 0 or adim == ADIM - 1:
        print(f"adım {adim}: kayıp = {kayip.item():.4f}")
sure = (time.perf_counter() - bas) * 1000
print(f"eğitim süresi: {sure:.0f} ms ({sure / ADIM:.2f} ms/adım, {torch.get_num_threads()} iş parçacığı)")

baglam = veri[:T]
uretilen = ""
with torch.no_grad():
    for _ in range(120):
        son = ileri(torch.tensor([baglam]), 1)[0, T - 1]
        en_iyi = int(son.argmax())
        uretilen += harfler[en_iyi]
        baglam = baglam[1:] + [en_iyi]
print("model şunu yazdı:", uretilen)
