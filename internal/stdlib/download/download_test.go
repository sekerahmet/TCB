package download

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// fileServer yolu ne olursa olsun body'yi Content-Length'le verir.
func fileServer(t *testing.T, body []byte) *server {
	return newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f.bin", zeroTime, strings.NewReader(string(body)))
	})
}

// TestDownloadNeedsHashAndSize: TR_sha256: ve TR_size: zorunludur (H7700, H7701); TR_none açık
// değerdir, "verilmiş" sayılır (bind.GetNamed değere bakmaz).
func TestDownloadNeedsHashAndSize(t *testing.T) {
	body := []byte("merhaba")
	s := fileServer(t, body)
	dir := slash(t.TempDir())
	wantCode(t, `TR_download(`+q(s.URL+"/a")+`, `+q(dir+"/a")+`, TR_size: 7)`, messages.H7700)
	wantCode(t, `TR_download(`+q(s.URL+"/a")+`, `+q(dir+"/a")+`, TR_sha256: TR_none)`, messages.H7701)
	out := mustRun(t, "TR_print("+dl(s.URL+"/a", dir+"/b", "TR_none", "TR_none")+")")
	if strings.TrimSpace(out) != sum(body) {
		t.Errorf("sonuç %q, beklenen %s", out, sum(body))
	}
	wantCode(t, dl(s.URL+"/a", dir+"/c", "TR_none", "-1"), messages.H7704)
}

// TestDownloadHashSpelling: büyük harfli karma H7702, 63 ve 65 karakter ve onaltılık dışı H7703.
func TestDownloadHashSpelling(t *testing.T) {
	dir := slash(t.TempDir())
	h := sum([]byte("x"))
	wantCode(t, dl("https://huggingface.co/a", dir+"/a", q(strings.ToUpper(h)), "TR_none"), messages.H7702)
	for _, bad := range []string{h[:63], h + "0", strings.Repeat("g", 64)} {
		wantCode(t, dl("https://huggingface.co/a", dir+"/a", q(bad), "TR_none"), messages.H7703)
	}
}

// TestDownloadIgnoresServerName: hedef yalnız programdan gelir; sunucunun Content-Disposition'ı ve
// yönlendirmenin yolu kullanılmaz.
func TestDownloadIgnoresServerName(t *testing.T) {
	body := []byte("veri")
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilk" {
			http.Redirect(w, r, "/baska/ad.bin", http.StatusFound)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="kotu.txt"`)
		w.Write(body)
	})
	dir := t.TempDir()
	mustRun(t, dl(s.URL+"/ilk", slash(dir)+"/hedef.bin", q(sum(body)), "4"))
	if !exists(filepath.Join(dir, "hedef.bin")) {
		t.Error("hedef.bin olmalı")
	}
	for _, name := range []string{"kotu.txt", "ad.bin", "ilk"} {
		if exists(filepath.Join(dir, name)) {
			t.Errorf("sunucunun adı kullanılmamalı: %s var", name)
		}
	}
}

// TestDownloadErrorsAreCoded: bütün hatalar kodlu çalışma hatasıdır: durum H7709, hedefin klasörü
// yok H7724, hedef klasör H7725, var olan hedef H7726 (TR_overwrite: ile geçer).
func TestDownloadErrorsAreCoded(t *testing.T) {
	body := []byte("veri")
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/yok" {
			http.Error(w, "yok", http.StatusNotFound)
			return
		}
		w.Write(body)
	})
	dir := slash(t.TempDir())
	err := wantCode(t, dl(s.URL+"/yok", dir+"/a", "TR_none", "TR_none"), messages.H7709)
	if !strings.Contains(err.Error(), strconv.Itoa(http.StatusNotFound)) {
		t.Errorf("ileti durum kodunu yazmalı: %v", err)
	}
	if exists(dir + "/a") {
		t.Error("hata sonrası hedef olmamalı")
	}
	wantCode(t, dl(s.URL+"/a", dir+"/olmayan/a", "TR_none", "TR_none"), messages.H7724)
	wantCode(t, dl(s.URL+"/a", dir, "TR_none", "TR_none"), messages.H7725)
	os.WriteFile(dir+"/var.bin", []byte("eski"), 0o644)
	wantCode(t, dl(s.URL+"/a", dir+"/var.bin", "TR_none", "TR_none"), messages.H7726)
	mustRun(t, dl(s.URL+"/a", dir+"/var.bin", "TR_none", "TR_none", "TR_overwrite: TR_true"))
	if b, _ := os.ReadFile(dir + "/var.bin"); string(b) != "veri" {
		t.Errorf("hedef ezilmeli: %q", b)
	}
}

// suggestionCodes indirmenin önerili iletileridir; hepsi aşağıdaki tabloda sınanır (DENETİM 645 N1:
// H7706'nın parametresi local olduğu için öneri tablosunun kapsam denetimi onu görmez).
var suggestionCodes = []messages.Code{messages.H7700, messages.H7701, messages.H7702, messages.H7706, messages.H7717, messages.H7720, messages.H7724, messages.H7726, messages.H7739}

// TestDownloadSuggestionsWork: her önerili iletinin önerisi kopyalanınca indirme biter.
func TestDownloadSuggestionsWork(t *testing.T) {
	body := []byte("merhaba")
	h := q(sum(body))
	s := fileServer(t, body)
	closeBody := rawServer(t, "HTTP/1.1 200 OK\r\nConnection: close\r\n\r\nmerhaba")
	dir := slash(t.TempDir())
	os.WriteFile(dir+"/var.bin", []byte("eski"), 0o644)
	cases := map[messages.Code][2]string{
		messages.H7700: {`TR_download(` + q(s.URL+"/a") + `, ` + q(dir+"/1") + `, TR_size: 7)`, dl(s.URL+"/a", dir+"/1", "TR_none", "7")},
		messages.H7701: {`TR_download(` + q(s.URL+"/a") + `, ` + q(dir+"/2") + `, TR_sha256: ` + h + `)`, dl(s.URL+"/a", dir+"/2", h, "TR_none")},
		messages.H7702: {dl(s.URL+"/a", dir+"/3", q(strings.ToUpper(sum(body))), "7"), dl(s.URL+"/a", dir+"/3", q(strings.ToUpper(sum(body)))+".TR_lower()", "7")},
		messages.H7706: {dl(strings.Replace(s.URL, "127.0.0.1", "localhost", 1)+"/a", dir+"/4", h, "7"), dl(s.URL+"/a", dir+"/4", h, "7")},
		messages.H7724: {dl(s.URL+"/a", dir+"/yeni/5", h, "7"), "TR_make_dir(" + q(dir+"/yeni") + "); " + dl(s.URL+"/a", dir+"/yeni/5", h, "7")},
		messages.H7726: {dl(s.URL+"/a", dir+"/var.bin", h, "7"), dl(s.URL+"/a", dir+"/var.bin", h, "7", "TR_overwrite: TR_true")},
		messages.H7739: {dl(closeBody, dir+"/6", "TR_none", "TR_none"), dl(closeBody, dir+"/6", "TR_none", "7")},
	}
	// H7717 ve H7720'nin karmasız dalı: aynı çağrı karmayla yinelenince Range gider ve indirme sürer.
	half := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			http.ServeContent(w, r, "f.bin", zeroTime, strings.NewReader(string(body)))
			return
		}
		w.Header().Set("Content-Length", "7")
		w.Write(body[:3])
	})
	cases[messages.H7720] = [2]string{dl(half.URL+"/a", dir+"/7", "TR_none", "TR_none"), dl(half.URL+"/a", dir+"/7", h, "7")}
	stall := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			http.ServeContent(w, r, "f.bin", zeroTime, strings.NewReader(string(body)))
			return
		}
		w.Header().Set("Content-Length", "7")
		w.Write(body[:3])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	cases[messages.H7717] = [2]string{dl(stall.URL+"/a", dir+"/8", "TR_none", "TR_none"), dl(stall.URL+"/a", dir+"/8", h, "7")}
	shortIdle(t)
	for _, c := range suggestionCodes {
		pair, ok := cases[c]
		if !ok {
			t.Errorf("%s önerili ama tabloda yok", c)
			continue
		}
		_, err := run(t, pair[0])
		if !messages.Has(err, c) {
			t.Errorf("%s bekleniyordu, gelen %v\n%s", c, err, pair[0])
			continue
		}
		if _, err := run(t, pair[1]); err != nil {
			t.Errorf("%s'ün önerisi çalışmadı: %v\n%s", c, err, pair[1])
		}
	}
	if len(cases) != len(suggestionCodes) {
		t.Errorf("tablo %d kod sınıyor, önerili kod %d", len(cases), len(suggestionCodes))
	}
	if r := half.ranges(); len(r) != 2 || r[1] != "bytes=3-" {
		t.Errorf("karmalı yineleme Range ile sürmeli: %v", r)
	}
}
