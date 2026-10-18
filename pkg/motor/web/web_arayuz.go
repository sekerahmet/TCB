package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Adlı argümanlar; adları sözlüktedir (pkg/sozluk, W9xx).
var (
	adPort      = baglama.KodAd(sozluk.W + 900)
	adArkaPlan  = baglama.KodAd(sozluk.W + 901)
	adGenislik  = baglama.KodAd(sozluk.W + 910)
	adYukseklik = baglama.KodAd(sozluk.W + 911)
	adPencereAc = baglama.KodAd(sozluk.W + 912)
	adAltBilgi  = baglama.KodAd(sozluk.W + 920)
	adRenk      = baglama.KodAd(sozluk.W + 921)
	adTip       = baglama.KodAd(sozluk.W + 922)
)

const (
	VAL_WEB_SERVER vm.ValueType = "WebSunucu"
	VAL_GUI_APP    vm.ValueType = "GörselArayüz"
)

// ============================================================================
// 1. WEB SUNUCU VE REST API MOTORU
// ============================================================================

type routeHandler struct {
	Method  string
	Path    string
	Handler vm.Value
}

type WebServerVal struct {
	Port       int
	Routes     []routeHandler
	StaticDirs map[string]string
	server     *http.Server
	mu         sync.RWMutex
	vmRef      *vm.VM
}

func (w *WebServerVal) Type() vm.ValueType { return VAL_WEB_SERVER }
func (w *WebServerVal) Inspect() string {
	return fmt.Sprintf("<WebSunucu http://localhost:%d (%d rota)>", w.Port, len(w.Routes))
}
func (w *WebServerVal) IsTruthy() bool { return true }

// Web sunucusu ve görsel arayüz üyeleri (adları sözlükte, pkg/sozluk Y8xx, Y9xx, Y00x).
const (
	uyeBaslat           = sozluk.Y + 6 // eş yazımı (yalnızca arayüzde): göster
	uyeArkaPlandaBaslat = sozluk.Y + 7
	uyeDurdur           = sozluk.Y + 8 // eş yazımı: kapat
	uyeKaydet           = sozluk.Y + 5

	uyeAdres        = sozluk.Y + 800 // eş yazımı: url
	uyePort         = sozluk.Y + 801
	uyeAl           = sozluk.Y + 802 // eş yazımı: get
	uyeGonder       = sozluk.Y + 803 // eş yazımı: post
	uyeRotaEkle     = sozluk.Y + 804 // eş yazımı: rota
	uyeStatikKlasor = sozluk.Y + 805

	uyeKartEkle   = sozluk.Y + 900 // eş yazımı: metrik_ekle
	uyeBaslikEkle = sozluk.Y + 901
	uyeMetinEkle  = sozluk.Y + 902 // eş yazımı: bilgi_ekle
	uyeTabloEkle  = sozluk.Y + 903
	uyeGrafikEkle = sozluk.Y + 904
	uyeFormEkle   = sozluk.Y + 905
	uyeButonEkle  = sozluk.Y + 906
	uyeHTMLKaydet = sozluk.Y + 907
	uyeHTMLUret   = sozluk.Y + 908
)

func (w *WebServerVal) GetProperty(kod sozluk.Kod, prop string) (vm.Value, error) {
	if kod == uyeBaslat && sozluk.EsYazimi(uyeBaslat, prop) {
		kod = sozluk.Yok // 'göster' yalnızca görsel arayüzün eş yazımıdır
	}
	switch kod {
	case uyeAdres:
		return &vm.StringVal{Val: fmt.Sprintf("http://localhost:%d", w.Port)}, nil
	case uyePort:
		return &vm.IntVal{Val: int64(w.Port)}, nil

	case uyeAl:
		return &vm.BuiltinVal{
			Name: "sunucu.al",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("sunucu.al(yol, fonksiyon) 2 argüman bekler")
				}
				w.mu.Lock()
				w.Routes = append(w.Routes, routeHandler{
					Method:  "GET",
					Path:    args[0].Inspect(),
					Handler: args[1],
				})
				w.mu.Unlock()
				return w, nil
			},
		}, nil

	case uyeGonder:
		return &vm.BuiltinVal{
			Name: "sunucu.gönder",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("sunucu.gönder(yol, fonksiyon) 2 argüman bekler")
				}
				w.mu.Lock()
				w.Routes = append(w.Routes, routeHandler{
					Method:  "POST",
					Path:    args[0].Inspect(),
					Handler: args[1],
				})
				w.mu.Unlock()
				return w, nil
			},
		}, nil

	case uyeRotaEkle:
		return &vm.BuiltinVal{
			Name: "sunucu.rota_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("sunucu.rota_ekle(yol, [metod], fonksiyon) en az 2 argüman bekler")
				}
				path := args[0].Inspect()
				method := "ALL"
				var handler vm.Value
				if len(args) == 2 {
					handler = args[1]
				} else {
					method = strings.ToUpper(args[1].Inspect())
					handler = args[2]
				}
				w.mu.Lock()
				w.Routes = append(w.Routes, routeHandler{
					Method:  method,
					Path:    path,
					Handler: handler,
				})
				w.mu.Unlock()
				return w, nil
			},
		}, nil

	case uyeStatikKlasor:
		return &vm.BuiltinVal{
			Name: "sunucu.statik_klasör",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("sunucu.statik_klasör(url_yolu, klasör_yolu) 2 argüman bekler")
				}
				w.mu.Lock()
				w.StaticDirs[args[0].Inspect()] = args[1].Inspect()
				w.mu.Unlock()
				return w, nil
			},
		}, nil

	case uyeArkaPlandaBaslat:
		return &vm.BuiltinVal{
			Name: "sunucu.arka_planda_başlat",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				w.vmRef = v
				if err := w.startServer(true); err != nil {
					return nil, err
				}
				return &vm.StringVal{Val: fmt.Sprintf("http://localhost:%d", w.Port)}, nil
			},
		}, nil

	case uyeBaslat:
		return &vm.BuiltinVal{
			Name: "sunucu.başlat",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				w.vmRef = v
				bg := false
				if b, ok := baglama.AdliAl(named, adArkaPlan); ok && b.IsTruthy() {
					bg = true
				}
				if err := w.startServer(bg); err != nil {
					return nil, err
				}
				return &vm.StringVal{Val: fmt.Sprintf("http://localhost:%d", w.Port)}, nil
			},
		}, nil

	case uyeDurdur:
		return &vm.BuiltinVal{
			Name: "sunucu.durdur",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if w.server != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					_ = w.server.Shutdown(ctx)
				}
				return vm.TRUE, nil
			},
		}, nil
	}
	return nil, fmt.Errorf("WebSunucu üzerinde %q özelliği veya metodu bulunamadı", prop)
}

func (w *WebServerVal) startServer(background bool) error {
	mux := http.NewServeMux()

	for prefix, dir := range w.StaticDirs {
		mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(dir))))
	}

	// Not: CORS izni (Access-Control-Allow-Origin) verilmez, Host ve başka siteden gelen
	// istekler guardLocalHTTP ile denetlenir (bkz. http_koruma.go).
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		w.mu.RLock()
		var matched *routeHandler
		for i := range w.Routes {
			r := &w.Routes[i]
			if r.Path == req.URL.Path && (r.Method == "ALL" || r.Method == req.Method) {
				matched = r
				break
			}
		}
		w.mu.RUnlock()

		if matched == nil {
			rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			rw.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(rw, `{"hata": "Rota bulunamadı", "yol": %q}`, req.URL.Path)
			return
		}

		// İstek (Request) sözlüğünü hazırla
		reqDict := vm.NewDictVal()
		reqDict.Set("yol", &vm.StringVal{Val: req.URL.Path})
		reqDict.Set("metod", &vm.StringVal{Val: req.Method})

		queryDict := vm.NewDictVal()
		for k, vals := range req.URL.Query() {
			if len(vals) > 0 {
				queryDict.Set(k, &vm.StringVal{Val: vals[0]})
			}
		}
		reqDict.Set("parametreler", queryDict)

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, "istek gövdesi okunamadı veya çok büyük", http.StatusRequestEntityTooLarge)
			return
		}
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if bodyStr != "" {
			// Gövde yalnızca Content-Type JSON ise JSON olarak çözülür.
			var parsed vm.Value
			if strings.HasPrefix(strings.ToLower(req.Header.Get("Content-Type")), "application/json") {
				parsed, _ = vm.JSONToTR(bodyBytes)
			}
			if parsed != nil {
				reqDict.Set("veri", parsed)
			} else {
				reqDict.Set("veri", &vm.StringVal{Val: bodyStr})
			}
		} else {
			reqDict.Set("veri", vm.NULL)
		}

		// Her istek kendi görev görünümüyle ve yorumlayıcı kilidi altında çalışır:
		// ana programla aynı anda paylaşılan değerlere yazıp süreci çökertemez.
		task := w.vmRef.NewTaskVM(false)
		resVal, err := task.RunTask(func() (vm.Value, error) {
			return task.ApplyFunction(matched.Handler, []vm.Value{reqDict}, nil, 0)
		})

		if err != nil {
			rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(rw, `{"hata": %q}`, err.Error())
			return
		}

		switch rv := resVal.(type) {
		case *vm.DictVal, *vm.ListVal:
			rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			jsonBytes, _ := json.MarshalIndent(vm.TRToGoValue(rv), "", "  ")
			_, _ = rw.Write(jsonBytes)
		case *vm.StringVal:
			trimmed := strings.TrimSpace(rv.Val)
			if isFullHTMLDocument(trimmed) {
				rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			} else if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
				rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			} else {
				rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
			}
			_, _ = rw.Write([]byte(rv.Val))
		default:
			rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = rw.Write([]byte(resVal.Inspect()))
		}
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", w.Port))
	if err != nil {
		return fmt.Errorf("Port %d dinlenemedi: %v", w.Port, err)
	}
	w.server = newLocalServer(w.Port, mux)

	if background {
		go func() {
			_ = w.server.Serve(ln)
		}()
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	fmt.Fprintf(w.vmRef.Out, "🌐 TR Web Sunucusu çalışıyor: http://localhost:%d\n", w.Port)
	// Sunucu çalışırken kilidi bırak; aksi halde hiçbir istek işleyicisi çalışamazdı
	var serveErr error
	w.vmRef.Blocking(func() { serveErr = w.server.Serve(ln) })
	return serveErr
}

// ============================================================================
// 2. GÖRSEL ARAYÜZ MOTORU (panel, kart, tablo, grafik ve form bileşenleri)
// ============================================================================

type guiWidget struct {
	ID       string
	Kind     string // "kpi", "header", "text", "alert", "table", "chart", "form", "button"
	Title    string
	Subtitle string
	Value    string
	Color    string
	Data     vm.Value
	Labels   []string
	Numbers  []float64
	Fields   []string
	Callback vm.Value
}

type GUIAppVal struct {
	Title   string
	Theme   string
	Width   int
	Height  int
	Widgets []guiWidget
	KPIs    []guiWidget
	server  *http.Server
	Port    int
	vmRef   *vm.VM
	mu      sync.Mutex
}

func (g *GUIAppVal) Type() vm.ValueType { return VAL_GUI_APP }
func (g *GUIAppVal) Inspect() string {
	return fmt.Sprintf("<GörselArayüz %q (%d bileşen)>", g.Title, len(g.Widgets)+len(g.KPIs))
}
func (g *GUIAppVal) IsTruthy() bool { return true }

func (g *GUIAppVal) GetProperty(kod sozluk.Kod, prop string) (vm.Value, error) {
	switch kod {
	case uyeKartEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.kart_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("kart_ekle(başlık, değer, [alt_bilgi], [renk]) en az 2 argüman bekler")
				}
				sub := ""
				if len(args) >= 3 {
					sub = args[2].Inspect()
				}
				if s, ok := baglama.AdliAl(named, adAltBilgi); ok {
					sub = s.Inspect()
				}
				color := "#38bdf8"
				if len(args) >= 4 {
					color = resolveColor(args[3].Inspect())
				}
				if c, ok := baglama.AdliAl(named, adRenk); ok {
					color = resolveColor(c.Inspect())
				}
				g.KPIs = append(g.KPIs, guiWidget{
					Kind:     "kpi",
					Title:    args[0].Inspect(),
					Value:    args[1].Inspect(),
					Subtitle: sub,
					Color:    color,
				})
				return g, nil
			},
		}, nil

	case uyeBaslikEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.başlık_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("başlık_ekle(metin, [alt_metin]) argüman bekler")
				}
				sub := ""
				if len(args) >= 2 {
					sub = args[1].Inspect()
				}
				g.Widgets = append(g.Widgets, guiWidget{
					Kind:     "header",
					Title:    args[0].Inspect(),
					Subtitle: sub,
				})
				return g, nil
			},
		}, nil

	case uyeMetinEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.metin_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("metin_ekle(metin) 1 argüman bekler")
				}
				g.Widgets = append(g.Widgets, guiWidget{
					Kind:  "text",
					Value: args[0].Inspect(),
				})
				return g, nil
			},
		}, nil

	case uyeTabloEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.tablo_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("tablo_ekle(başlık, liste_veya_matris) 2 argüman bekler")
				}
				g.Widgets = append(g.Widgets, guiWidget{
					ID:    fmt.Sprintf("tbl_%d", len(g.Widgets)+1),
					Kind:  "table",
					Title: args[0].Inspect(),
					Data:  args[1],
				})
				return g, nil
			},
		}, nil

	case uyeGrafikEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.grafik_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 3 {
					return nil, fmt.Errorf("grafik_ekle(başlık, etiketler_listesi, değerler_listesi) 3 argüman bekler")
				}
				var labels []string
				if lv, ok := args[1].(*vm.ListVal); ok {
					for _, el := range lv.Elements {
						labels = append(labels, el.Inspect())
					}
				}
				var nums []float64
				if lv, ok := args[2].(*vm.ListVal); ok {
					for _, el := range lv.Elements {
						if f, ok := vm.ToFloat(el); ok {
							nums = append(nums, f)
						}
					}
				}
				chartType := "çubuk"
				if len(args) >= 4 {
					chartType = args[3].Inspect()
				}
				if ct, ok := baglama.AdliAl(named, adTip); ok {
					chartType = ct.Inspect()
				}
				g.Widgets = append(g.Widgets, guiWidget{
					ID:       fmt.Sprintf("chart_%d", len(g.Widgets)+1),
					Kind:     "chart",
					Title:    args[0].Inspect(),
					Subtitle: chartType,
					Labels:   labels,
					Numbers:  nums,
				})
				return g, nil
			},
		}, nil

	case uyeFormEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.form_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 3 {
					return nil, fmt.Errorf("form_ekle(başlık, alanlar_listesi, gönder_fonksiyonu) 3 argüman bekler")
				}
				var fields []string
				if lv, ok := args[1].(*vm.ListVal); ok {
					for _, el := range lv.Elements {
						fields = append(fields, el.Inspect())
					}
				}
				g.Widgets = append(g.Widgets, guiWidget{
					ID:       fmt.Sprintf("form_%d", len(g.Widgets)+1),
					Kind:     "form",
					Title:    args[0].Inspect(),
					Fields:   fields,
					Callback: args[2],
				})
				return g, nil
			},
		}, nil

	case uyeButonEkle:
		return &vm.BuiltinVal{
			Name: "arayüz.buton_ekle",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("buton_ekle(etiket, fonksiyon) 2 argüman bekler")
				}
				g.Widgets = append(g.Widgets, guiWidget{
					ID:       fmt.Sprintf("btn_%d", len(g.Widgets)+1),
					Kind:     "button",
					Title:    args[0].Inspect(),
					Callback: args[1],
				})
				return g, nil
			},
		}, nil

	case uyeHTMLKaydet, uyeKaydet:
		return &vm.BuiltinVal{
			Name: "arayüz.html_kaydet",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) != 1 {
					return nil, vm.ArgumanSayisiHatasi("html_kaydet", "1", len(args), "arayüz.html_kaydet(dosya_yolu)")
				}
				yol, ok := args[0].(*vm.StringVal)
				if !ok {
					return nil, vm.ArgumanTipHatasi("html_kaydet", 1, "Metin (dosya yolu)", args[0])
				}
				htmlStr := g.renderHTML()
				// Yazma da okuma gibi TR'nin tek yol kuralını izler (vm.YolCoz)
				cozulen, _ := v.YolCoz(yol.Val)
				if err := os.WriteFile(cozulen, []byte(htmlStr), 0644); err != nil {
					return nil, turkce.DosyaHatasi("html_kaydet", yol.Val, "yazılamadı", err)
				}
				return vm.TRUE, nil
			},
		}, nil

	case uyeHTMLUret:
		return &vm.BuiltinVal{
			Name: "arayüz.html_üret",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				return &vm.StringVal{Val: g.renderHTML()}, nil
			},
		}, nil

	case uyeArkaPlandaBaslat:
		return &vm.BuiltinVal{
			Name: "arayüz.arka_planda_başlat",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				port := 8095
				if len(args) >= 1 {
					if iv, ok := args[0].(*vm.IntVal); ok {
						port = int(iv.Val)
					}
				}
				if p, ok := baglama.AdliAl(named, adPort); ok {
					if iv, ok := p.(*vm.IntVal); ok {
						port = int(iv.Val)
					}
				}
				openWin := false
				if ow, ok := baglama.AdliAl(named, adPencereAc); ok && ow.IsTruthy() {
					openWin = true
				}
				g.vmRef = v
				if err := g.startGUIServer(port, true, openWin); err != nil {
					return nil, err
				}
				return &vm.StringVal{Val: fmt.Sprintf("http://localhost:%d", port)}, nil
			},
		}, nil

	case uyeBaslat:
		return &vm.BuiltinVal{
			Name: "arayüz.başlat",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				port := 8095
				if len(args) >= 1 {
					if iv, ok := args[0].(*vm.IntVal); ok {
						port = int(iv.Val)
					}
				}
				if p, ok := baglama.AdliAl(named, adPort); ok {
					if iv, ok := p.(*vm.IntVal); ok {
						port = int(iv.Val)
					}
				}
				openWin := true
				if ow, ok := baglama.AdliAl(named, adPencereAc); ok {
					openWin = ow.IsTruthy()
				}
				bg := false
				if b, ok := baglama.AdliAl(named, adArkaPlan); ok && b.IsTruthy() {
					bg = true
				}
				g.vmRef = v
				if err := g.startGUIServer(port, bg, openWin); err != nil {
					return nil, err
				}
				return &vm.StringVal{Val: fmt.Sprintf("http://localhost:%d", port)}, nil
			},
		}, nil

	case uyeDurdur:
		return &vm.BuiltinVal{
			Name: "arayüz.durdur",
			Fn: func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if g.server != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					_ = g.server.Shutdown(ctx)
				}
				return vm.TRUE, nil
			},
		}, nil
	}
	return nil, fmt.Errorf("GörselArayüz üzerinde %q özelliği veya metodu bulunamadı", prop)
}

func resolveColor(c string) string {
	switch strings.ToLower(c) {
	case "yeşil", "yesil", "green":
		return "#22c55e"
	case "mavi", "blue":
		return "#38bdf8"
	case "kırmızı", "kirmizi", "red":
		return "#ef4444"
	case "sarı", "sari", "turuncu", "yellow", "orange":
		return "#f59e0b"
	case "mor", "purple":
		return "#a855f7"
	default:
		if safeCSSColor.MatchString(strings.TrimSpace(c)) {
			return strings.TrimSpace(c)
		}
		// Güvenli olmayan / tanınmayan renk değeri style özniteliğine yazılmaz
		return "#38bdf8"
	}
}

// safeCSSColor yalnızca #hex, renk adı (red) veya rgb()/rgba()/hsl()/hsla() biçimine izin verir.
// Böylece renk değeri üzerinden style özniteliğinden çıkıp HTML/JS enjekte edilemez.
var safeCSSColor = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[a-zA-Z]{1,30}|(rgb|rgba|hsl|hsla)\(\s*[0-9.,%\s]{1,60}\))$`)

// jsStringAttr bir metni, HTML özniteliği içindeki JavaScript metin değişmezi olarak güvenle kodlar.
func jsStringAttr(s string) string {
	b, _ := json.Marshal(s)
	return html.EscapeString(string(b))
}

func (g *GUIAppVal) startGUIServer(port int, background bool, openWindow bool) error {
	g.Port = port
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Bileşenlerdeki TR değerleri (Liste/Sözlük) ana programla aynı anda okunmasın diye kilit altında üret
		var page string
		_, _ = g.vmRef.NewTaskVM(false).RunTask(func() (vm.Value, error) {
			page = g.renderHTML()
			return vm.NULL, nil
		})
		_, _ = rw.Write([]byte(page))
	})

	// Form ve Buton etkileşim API uç noktası
	mux.HandleFunc("/api/etkilesim", func(rw http.ResponseWriter, req *http.Request) {
		// CSRF / DNS rebinding koruması: yalnızca kendi sayfamızdan gelen JSON POST istekleri.
		// application/json başka sitelerden ön kontrolsüz (preflight) gönderilemez.
		host := strings.ToLower(req.Host)
		if host != fmt.Sprintf("localhost:%d", port) && host != fmt.Sprintf("127.0.0.1:%d", port) {
			http.Error(rw, "yasak: geçersiz Host", http.StatusForbidden)
			return
		}
		if req.Method != http.MethodPost || !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
			http.Error(rw, "yalnızca application/json POST", http.StatusUnsupportedMediaType)
			return
		}
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		var payload struct {
			WidgetID string            `json:"widget_id"`
			Values   map[string]string `json:"values"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			_, _ = fmt.Fprintf(rw, `{"durum":"hata","mesaj":%q}`, err.Error())
			return
		}

		g.mu.Lock()
		defer g.mu.Unlock()

		for _, w := range g.Widgets {
			if w.ID == payload.WidgetID && w.Callback != nil {
				formDict := vm.NewDictVal()
				for k, v := range payload.Values {
					formDict.Set(k, &vm.StringVal{Val: v})
				}
				task := g.vmRef.NewTaskVM(false)
				cb := w.Callback
				res, err := task.RunTask(func() (vm.Value, error) {
					return task.ApplyFunction(cb, []vm.Value{formDict}, nil, 0)
				})
				if err != nil {
					_, _ = fmt.Fprintf(rw, `{"durum":"hata","mesaj":%q}`, err.Error())
					return
				}
				_, _ = fmt.Fprintf(rw, `{"durum":"tamam","sonuc":%q}`, res.Inspect())
				return
			}
		}
		_, _ = fmt.Fprintf(rw, `{"durum":"hata","mesaj":"Etkileşim bileşeni bulunamadı"}`)
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("Arayüz portu (%d) açılamadı: %v", port, err)
	}
	g.server = newLocalServer(port, mux)

	url := fmt.Sprintf("http://localhost:%d", port)
	if openWindow {
		go launchNativeAppWindow(url, g.Width, g.Height)
	}

	if background {
		go func() {
			_ = g.server.Serve(ln)
		}()
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	fmt.Fprintf(g.vmRef.Out, "🖥️  TR Görsel Arayüz (GUI) çalışıyor: %s\n", url)
	var serveErr error
	g.vmRef.Blocking(func() { serveErr = g.server.Serve(ln) })
	return serveErr
}

func launchNativeAppWindow(url string, width, height int) {
	if runtime.GOOS == "windows" {
		// Edge veya Chrome'u masaüstü uygulama penceresi (--app) modunda aç
		edgeCmd := exec.Command("cmd", "/c", "start", "msedge", fmt.Sprintf("--app=%s", url), fmt.Sprintf("--window-size=%d,%d", width, height))
		if err := edgeCmd.Start(); err != nil {
			_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	}
}

func (g *GUIAppVal) renderHTML() string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="UTF-8">
<title>` + html.EscapeString(g.Title) + ` — TR Görsel Arayüz</title>
<style>
  * { box-sizing: border-box; font-family: 'Segoe UI', system-ui, sans-serif; }
  body { margin: 0; background: #0f172a; color: #e2e8f0; }
  header { background: linear-gradient(90deg, #1e293b, #0f172a); padding: 16px 28px; border-bottom: 1px solid #334155; display: flex; justify-content: space-between; align-items: center; }
  header h1 { margin: 0; font-size: 20px; color: #38bdf8; display: flex; align-items: center; gap: 10px; }
  .badge { background: #1e3a8a; color: #93c5fd; padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 600; }
  .container { max-width: 1260px; margin: 24px auto; padding: 0 24px; }
  .kpi-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; margin-bottom: 24px; }
  .kpi-card { background: #1e293b; border: 1px solid #334155; border-left: 4px solid #38bdf8; border-radius: 10px; padding: 16px 20px; box-shadow: 0 4px 12px rgba(0,0,0,0.2); }
  .kpi-title { font-size: 12px; text-transform: uppercase; letter-spacing: 0.6px; color: #94a3b8; font-weight: 600; }
  .kpi-value { font-size: 28px; font-weight: 700; margin: 6px 0 4px 0; color: #f8fafc; }
  .kpi-sub { font-size: 12px; color: #64748b; }
  .panel { background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 20px; margin-bottom: 20px; }
  .panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
  .panel-title { font-size: 16px; font-weight: 700; color: #f1f5f9; margin: 0; }
  .tablo-arac { display: flex; gap: 10px; align-items: center; }
  .tablo-ara { background: #0f172a; border: 1px solid #475569; color: #f8fafc; padding: 6px 12px; border-radius: 6px; font-size: 13px; }
  table { width: 100%; border-collapse: collapse; font-size: 13.5px; }
  th { background: #0f172a; color: #38bdf8; text-align: left; padding: 10px 12px; border-bottom: 2px solid #334155; cursor: pointer; user-select: none; }
  td { padding: 9px 12px; border-bottom: 1px solid #334155; color: #cbd5e1; }
  tr:hover td { background: #26354d; }
  .chart-bars { display: flex; align-items: flex-end; gap: 18px; height: 200px; padding: 20px 12px 0 12px; border-bottom: 1px solid #334155; }
  .bar-col { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 6px; }
  .bar-fill { width: 100%; max-width: 64px; background: linear-gradient(180deg, #38bdf8, #2563eb); border-radius: 6px 6px 0 0; transition: height 0.3s; }
  .bar-val { font-size: 12px; font-weight: 700; color: #93c5fd; }
  .bar-lbl { font-size: 12px; color: #94a3b8; margin-top: 6px; text-align: center; }
  .form-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 14px; }
  .form-group label { display: block; font-size: 12px; color: #94a3b8; margin-bottom: 5px; font-weight: 600; }
  .form-group input { width: 100%; background: #0f172a; border: 1px solid #475569; color: #fff; padding: 8px 12px; border-radius: 6px; }
  .btn { background: #2563eb; color: #fff; border: none; padding: 9px 18px; border-radius: 6px; font-weight: 600; cursor: pointer; font-size: 13px; }
  .btn:hover { background: #1d4ed8; }
  .result-box { margin-top: 12px; padding: 10px 14px; border-radius: 6px; background: #064e3b; color: #a7f3d0; display: none; font-size: 13.5px; }
</style>
</head>
<body>
<header>
  <h1>🇹🇷 ` + html.EscapeString(g.Title) + `</h1>
  <span class="badge">TR Görsel Arayüz</span>
</header>
<div class="container">`)

	if len(g.KPIs) > 0 {
		sb.WriteString(`<div class="kpi-grid">`)
		for _, k := range g.KPIs {
			sb.WriteString(fmt.Sprintf(`
  <div class="kpi-card" style="border-left-color: %s">
    <div class="kpi-title">%s</div>
    <div class="kpi-value" style="color: %s">%s</div>
    <div class="kpi-sub">%s</div>
  </div>`, html.EscapeString(resolveColor(k.Color)), html.EscapeString(k.Title), html.EscapeString(resolveColor(k.Color)), html.EscapeString(k.Value), html.EscapeString(k.Subtitle)))
		}
		sb.WriteString(`</div>`)
	}

	for _, w := range g.Widgets {
		switch w.Kind {
		case "header":
			sb.WriteString(fmt.Sprintf(`<div class="panel"><h2 class="panel-title">%s</h2><p style="color:#94a3b8;margin:6px 0 0 0;font-size:13.5px">%s</p></div>`,
				html.EscapeString(w.Title), html.EscapeString(w.Subtitle)))

		case "text":
			sb.WriteString(fmt.Sprintf(`<div class="panel"><p style="margin:0;line-height:1.6">%s</p></div>`, html.EscapeString(w.Value)))

		case "table":
			sb.WriteString(g.renderTableHTML(w))

		case "chart":
			sb.WriteString(g.renderChartHTML(w))

		case "form":
			sb.WriteString(g.renderFormHTML(w))

		case "button":
			sb.WriteString(fmt.Sprintf(`<div style="margin-bottom:16px"><button class="btn" onclick="submitWidget(%s, [])">%s</button><div id="res_%s" class="result-box"></div></div>`,
				jsStringAttr(w.ID), html.EscapeString(w.Title), html.EscapeString(w.ID)))
		}
	}

	sb.WriteString(`
</div>
<script>
function tabloFiltrele(inputEl, tableId) {
  const q = inputEl.value.toLowerCase();
  const rows = document.querySelectorAll('#' + tableId + ' tbody tr');
  rows.forEach(r => {
    r.style.display = r.innerText.toLowerCase().includes(q) ? '' : 'none';
  });
}
async function submitWidget(widgetId, fieldNames) {
  const values = {};
  fieldNames.forEach(f => {
    const el = document.getElementById(widgetId + '_' + f);
    if (el) values[f] = el.value;
  });
  const res = await fetch('/api/etkilesim', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({widget_id: widgetId, values: values})
  });
  const data = await res.json();
  const box = document.getElementById('res_' + widgetId);
  if (box) {
    box.style.display = 'block';
    box.innerText = data.durum === 'tamam' ? '✅ ' + data.sonuc : '❌ ' + data.mesaj;
  }
}
</script>
</body>
</html>`)
	return sb.String()
}

func (g *GUIAppVal) renderTableHTML(w guiWidget) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="panel">
  <div class="panel-header">
    <h3 class="panel-title">📋 %s</h3>
    <div class="tablo-arac">
      <input class="tablo-ara" placeholder="🔍 Tabloda ara..." oninput="tabloFiltrele(this, %s)">
    </div>
  </div>
  <table id="%s">`, html.EscapeString(w.Title), jsStringAttr(w.ID), html.EscapeString(w.ID)))

	if lv, ok := w.Data.(*vm.ListVal); ok && len(lv.Elements) > 0 {
		if firstDict, ok := lv.Elements[0].(*vm.DictVal); ok {
			cols := firstDict.Order
			if len(cols) == 0 {
				for k := range firstDict.Pairs {
					cols = append(cols, k)
				}
				sort.Strings(cols)
			}
			sb.WriteString(`<thead><tr>`)
			for _, c := range cols {
				sb.WriteString(fmt.Sprintf(`<th>%s</th>`, html.EscapeString(strings.ToUpper(firstDict.DisplayKey(c)))))
			}
			sb.WriteString(`</tr></thead><tbody>`)
			for _, rowEl := range lv.Elements {
				if d, ok := rowEl.(*vm.DictVal); ok {
					sb.WriteString(`<tr>`)
					for _, c := range cols {
						val := d.Pairs[c]
						valStr := ""
						if val != nil {
							valStr = val.Inspect()
						}
						sb.WriteString(fmt.Sprintf(`<td>%s</td>`, html.EscapeString(valStr)))
					}
					sb.WriteString(`</tr>`)
				}
			}
			sb.WriteString(`</tbody>`)
		} else {
			sb.WriteString(`<thead><tr><th>#</th><th>DEĞER</th></tr></thead><tbody>`)
			for idx, el := range lv.Elements {
				sb.WriteString(fmt.Sprintf(`<tr><td>%d</td><td>%s</td></tr>`, idx, html.EscapeString(el.Inspect())))
			}
			sb.WriteString(`</tbody>`)
		}
	}
	sb.WriteString(`</table></div>`)
	return sb.String()
}

func (g *GUIAppVal) renderChartHTML(w guiWidget) string {
	maxVal := 1.0
	for _, n := range w.Numbers {
		if n > maxVal {
			maxVal = n
		}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="panel"><div class="panel-header"><h3 class="panel-title">📊 %s</h3></div><div class="chart-bars">`, html.EscapeString(w.Title)))
	for i, lbl := range w.Labels {
		val := 0.0
		if i < len(w.Numbers) {
			val = w.Numbers[i]
		}
		hPct := int((val / maxVal) * 150)
		if hPct < 6 {
			hPct = 6
		}
		sb.WriteString(fmt.Sprintf(`
    <div class="bar-col">
      <div class="bar-val">%g</div>
      <div class="bar-fill" style="height:%dpx"></div>
      <div class="bar-lbl">%s</div>
    </div>`, val, hPct, html.EscapeString(lbl)))
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

func (g *GUIAppVal) renderFormHTML(w guiWidget) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="panel"><div class="panel-header"><h3 class="panel-title">📝 %s</h3></div><div class="form-row">`, html.EscapeString(w.Title)))
	var jsFields []string
	for _, f := range w.Fields {
		sb.WriteString(fmt.Sprintf(`
    <div class="form-group">
      <label>%s</label>
      <input id="%s_%s" placeholder="%s girin...">
    </div>`, html.EscapeString(f), html.EscapeString(w.ID), html.EscapeString(f), html.EscapeString(f)))
		jsFields = append(jsFields, jsStringAttr(f))
	}
	sb.WriteString(fmt.Sprintf(`</div><button class="btn" onclick="submitWidget(%s, [%s])">⚡ Çalıştır / Gönder</button><div id="res_%s" class="result-box"></div></div>`,
		jsStringAttr(w.ID), strings.Join(jsFields, ","), html.EscapeString(w.ID)))
	return sb.String()
}

// Kaydet web istemcisi (web_al, web_gönder), web sunucu ve görsel arayüz yerleşiklerini
// VM'ye kaydeder.
func Kaydet(v *vm.VM) {
	// 0. web_al(url), web_gönder(url, veri) — http_istemci.go
	istemciKaydet(v)

	// 1. web_sunucu(port: 8080)
	v.RegisterBuiltinKod(sozluk.W+110, func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		port := 8080
		if len(args) >= 1 {
			if iv, ok := args[0].(*vm.IntVal); ok {
				port = int(iv.Val)
			}
		}
		if p, ok := baglama.AdliAl(named, adPort); ok {
			if iv, ok := p.(*vm.IntVal); ok {
				port = int(iv.Val)
			}
		}
		return &WebServerVal{
			Port:       port,
			Routes:     []routeHandler{},
			StaticDirs: make(map[string]string),
			vmRef:      v,
		}, nil
	})

	// 2. arayüz_oluştur(başlık, genişlik: 1050, yükseklik: 750)
	createGUI := func(v *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		title := "TR Görsel Uygulaması"
		if len(args) >= 1 {
			title = args[0].Inspect()
		}
		width := 1080
		height := 760
		if w, ok := baglama.AdliAl(named, adGenislik); ok {
			if iv, ok := w.(*vm.IntVal); ok {
				width = int(iv.Val)
			}
		}
		if h, ok := baglama.AdliAl(named, adYukseklik); ok {
			if iv, ok := h.(*vm.IntVal); ok {
				height = int(iv.Val)
			}
		}
		return &GUIAppVal{
			Title:   title,
			Theme:   "koyu",
			Width:   width,
			Height:  height,
			Widgets: []guiWidget{},
			KPIs:    []guiWidget{},
			vmRef:   v,
		}, nil
	}
	v.RegisterBuiltinKod(sozluk.W+300, createGUI)
}
