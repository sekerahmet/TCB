package files

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// listeleYerlesik klasörü_listele([klasör = "."], [kalıp = "*"]) yerleşiğidir: kalıba uyan
// dosya ve klasörleri sıralı bir Yol listesi olarak verir.
func listeleYerlesik(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.Cagri(codes.G121)
	if len(args) > 2 {
		return nil, interp.ArgumanSayisiHatasi(codes.G121, "0, 1 ya da 2", len(args), "", "klasör", "klasör, kalıp")
	}
	taban, desen := ".", "*"
	if len(args) > 0 {
		t, err := yolOku(codes.G121, 1, args[0])
		if err != nil {
			return nil, err
		}
		taban = t
	}
	if len(args) > 1 {
		s, ok := args[1].(*interp.StringVal)
		if !ok {
			return nil, interp.ArgumanTipHatasi(codes.G121, 2, args[1], codes.A202)
		}
		desen = s.Val
	}
	k, err := kalipDerle(desen)
	if err != nil {
		return nil, fmt.Errorf("%s: kalıp %q geçersiz: %v", fn, desen, err)
	}
	cozulen, bakilanlar := m.YolCoz(filepath.FromSlash(taban))
	bilgi, err := os.Stat(cozulen)
	if err != nil {
		return nil, interp.OkumaHatasi(fn, taban, "listelenemedi", err, bakilanlar)
	}
	if !bilgi.IsDir() {
		return nil, fmt.Errorf("%s: %q bir dosya, klasör değil; listelenecek klasörü verin", fn, taban)
	}
	g := &gezgin{k: k, gorulen: map[string]bool{}}
	m.Blocking(func() { g.gez(cozulen, "", 0) })
	if g.okunamayan > 0 {
		m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %s: %d alt klasör okunamadı ve atlandı (ilki: %q: %s)\n",
			fn, g.okunamayan, g.ilkOkunamayan, turkish.DosyaHatasiNedeni(g.ilkOkunamayan, g.ilkHata)))
	}
	tabanYol := temizle(taban)
	yollar := make([]string, len(g.bulunan))
	for i, rel := range g.bulunan {
		if tabanYol == "." {
			yollar[i] = rel
		} else {
			yollar[i] = temizle(filepath.Join(filepath.FromSlash(tabanYol), filepath.FromSlash(rel)))
		}
	}
	sort.Strings(yollar)
	sonuc := make([]interp.Value, len(yollar))
	for i, y := range yollar {
		sonuc[i] = &Yol{yol: y}
	}
	return &interp.ListVal{Elements: sonuc}, nil
}

// gezgin kalıbı bir klasör ağacında eşler. Yorumlayıcı kilidi bırakılmışken çalışır; TR
// değerlerine dokunmaz.
type gezgin struct {
	k             *kalip
	bulunan       []string // tabana göre, / ile
	gorulen       map[string]bool
	okunamayan    int
	ilkOkunamayan string
	ilkHata       error
}

// gez klasor'ü (tabana göre yolu rel) kalıbın i. bileşeninden başlayarak eşler.
func (g *gezgin) gez(klasor, rel string, i int) {
	b := g.k.bilesenler[i]
	son := i == len(g.k.bilesenler)-1
	if b.cift {
		if son {
			g.hepsi(klasor, rel) // sondaki **: alttaki her şey
			return
		}
		g.gez(klasor, rel, i+1) // sıfır klasör
	}
	girdiler, err := os.ReadDir(klasor)
	if err != nil {
		g.okunamadi(klasor, err)
		return
	}
	for _, e := range girdiler {
		ad := e.Name()
		alt, altRel := filepath.Join(klasor, ad), path.Join(rel, ad)
		if b.cift {
			// ** gizli klasörlere ve klasör bağlantılarının içine girmez (döngü olmaz).
			if e.IsDir() && ad[0] != '.' {
				g.gez(alt, altRel, i)
			}
			continue
		}
		if !b.eslesir(ad) {
			continue
		}
		if son {
			g.ekle(altRel)
		} else if klasorMu(alt, e) {
			g.gez(alt, altRel, i+1)
		}
	}
}

// hepsi klasörün altındaki gizli olmayan her şeyi ekler (bağlantıların içine girmeden).
func (g *gezgin) hepsi(klasor, rel string) {
	girdiler, err := os.ReadDir(klasor)
	if err != nil {
		g.okunamadi(klasor, err)
		return
	}
	for _, e := range girdiler {
		ad := e.Name()
		if ad[0] == '.' {
			continue
		}
		altRel := path.Join(rel, ad)
		g.ekle(altRel)
		if e.IsDir() {
			g.hepsi(filepath.Join(klasor, ad), altRel)
		}
	}
}

// klasorMu girdi bir klasör mü? Açıkça adı yazılmış (ya da tek bir bileşen kalıbıyla
// eşlenmiş) bir klasör bağlantısının içine girilebilir: orada döngü yoktur.
func klasorMu(yol string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	b, err := os.Stat(yol)
	return err == nil && b.IsDir()
}

func (g *gezgin) ekle(rel string) {
	if !g.gorulen[rel] {
		g.gorulen[rel] = true
		g.bulunan = append(g.bulunan, rel)
	}
}

func (g *gezgin) okunamadi(klasor string, err error) {
	g.okunamayan++
	if g.okunamayan == 1 {
		g.ilkOkunamayan, g.ilkHata = filepath.ToSlash(klasor), err
	}
}
