// record bir ölçüm ya da denetim koşusunu kendiliğinden kaydeder: komutu çalıştırır, çıktısını hem
// ekrana hem kayıt klasörüne yazar, sonunda koşunun kaydını (ne, ne zaman, hangi commit, hangi
// makine, komut, süre, çıkış kodu, ölçümler, dosyalar) JSON olarak yazar ve dizine (index.jsonl) bir
// satır ekler. Elle özet yazılmaz; zaman içindeki karşılaştırma dizinden yapılır.
//
// Kullanım (depo kökünden):
//
//	go run ./tools/record --name zorlama --out /content/drive/MyDrive/TR_dili/olcumler -- bash tools/full_check.sh --kip=zorlama
//
// Düzen: her işin bir klasörü vardır, koşuları içinde zamanla adlanır (Türkiye saati, YYYYAAGGSSDDSS;
// adlar tarih sırasıyla dizilir):
//
//	<out>/index.jsonl                    bütün işlerin koşuları, koşu başına bir satır
//	<out>/<name>/20261023143005.json     kayıt
//	<out>/<name>/20261023143005.log      komutun çıktısı
//	<out>/<name>/20261023143005_<dosya>  komutun bıraktığı dosyalar
//
// Komut, koşu sürerken dosyalarını TR_KAYIT_KLASORU ortam değişkeninin gösterdiği geçici klasöre
// yazar; koşu bitince araç onları zaman önekiyle iş klasörüne taşır. Komut metrics.json bırakırsa
// içeriği kayda "metrics" olarak da gömülür (tools/full_check.sh adım sürelerini ve sonucu böyle
// yazar). Çıkış kodu komutunkidir. Kayıt biçimi pytest-benchmark'ın commit bilgisini (id, branch,
// dirty) ve MLflow'un koşu kaydını (zaman, ölçümler, dosyalar, commit) izler; ayrıntı README'de.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// envDir komuta geçici dosya klasörünü bildiren ortam değişkeni.
const envDir = "TR_KAYIT_KLASORU"

// schemaVersion kayıt biçiminin sürümü; alan anlamı değişince artar.
const schemaVersion = 1

// stamp koşu dosyalarının adının başı: YYYYAAGGSSDDSS. Adlar böylece tarih sırasıyla dizilir.
const stamp = "20060102150405"

// turkey kayıtların saat dilimi: Türkiye saati (UTC+3; Türkiye 2016'dan beri yaz saati uygulamaz).
// Kayıtları okuyan geliştiricinin saatidir; her makine (Colab UTC'dir) zamanı buna çevirdiği için
// farklı makinelerden gelen kayıtlar karışmaz. JSON'daki zamanlar "+03:00" ekiyle yazılır.
var turkey = time.FixedZone("+03", 3*60*60)

// Record bir koşunun kaydıdır (<name>/<zaman>.json ve index.jsonl'ın bir satırı).
type Record struct {
	Schema   int             `json:"schema"`
	Name     string          `json:"name"`
	Started  time.Time       `json:"started"`
	Ended    time.Time       `json:"ended"`
	Seconds  float64         `json:"seconds"`
	Command  []string        `json:"command"`
	ExitCode int             `json:"exit_code"`
	Commit   Commit          `json:"commit"`
	Machine  Machine         `json:"machine"`
	Metrics  json.RawMessage `json:"metrics,omitempty"`
	// Files koşunun dosyaları, kayıt köküne (--out) göre yollar: kayıt, çıktı ve ekler.
	Files []string `json:"files"`
}

// options komut satırı seçenekleri.
type options struct {
	name, out string
}

func main() {
	var o options
	flag.StringVar(&o.name, "name", "", "job name: the folder of its runs (letters, digits, - and _)")
	flag.StringVar(&o.out, "out", defaultOut(), "record root: job folders and index.jsonl")
	flag.Parse()
	code, err := run(o, flag.Args(), os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "record:", err)
		if code == 0 {
			code = 2
		}
	}
	os.Exit(code)
}

// defaultOut varsayılan kayıt yeri depo dışındadır: tam denetim depoda iz bırakan komutu kalır sayar.
func defaultOut() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, "tr-olcumler")
	}
	return filepath.Join(os.TempDir(), "tr-olcumler")
}

var validName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// run komutu çalıştırır ve kaydı yazar; komutun çıkış kodunu verir.
func run(o options, command []string, stdout io.Writer) (int, error) {
	if len(command) == 0 {
		return 2, fmt.Errorf("no command (usage: record --name N [--out DIR] -- command args...)")
	}
	if !validName.MatchString(o.name) {
		return 2, fmt.Errorf("--name %q: use letters, digits, - and _", o.name)
	}
	start := time.Now().In(turkey)
	id := start.Format(stamp)
	job := filepath.Join(o.out, o.name)
	work := filepath.Join(job, "."+id) // komutun geçici dosyaları; koşudan sonra taşınır
	if err := os.MkdirAll(work, 0o755); err != nil {
		return 2, err
	}
	code, err := execute(filepath.Join(job, id+".log"), work, command, stdout)
	if err != nil {
		return 2, err
	}
	end := time.Now().In(turkey)
	r := Record{Schema: schemaVersion, Name: o.name, Started: start, Ended: end,
		Seconds: end.Sub(start).Round(time.Millisecond).Seconds(), Command: command, ExitCode: code,
		Commit: commitInfo(), Machine: machineInfo()}
	if m, err := os.ReadFile(filepath.Join(work, "metrics.json")); err == nil && json.Valid(m) {
		r.Metrics = json.RawMessage(m)
	}
	attached, err := attach(work, job, id)
	if err != nil {
		return code, err
	}
	r.Files = []string{o.name + "/" + id + ".json", o.name + "/" + id + ".log"}
	for _, f := range attached {
		r.Files = append(r.Files, o.name+"/"+f)
	}
	return code, write(r, filepath.Join(job, id+".json"), filepath.Join(o.out, "index.jsonl"))
}

// execute komutu geçici klasörü ortamda bildirerek çalıştırır; çıktısını hem stdout'a hem
// logPath'e yazar. Komutun kendi hatası (sıfırdan farklı çıkış) hata değil, çıkış kodudur.
func execute(logPath, work string, command []string, stdout io.Writer) (int, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(os.Environ(), envDir+"="+work)
	w := io.MultiWriter(stdout, logFile)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = w, w, os.Stdin
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// attach geçici klasördeki dosyaları iş klasörüne <id>_<göreli yol, / yerine _> adıyla taşır, klasörü
// siler ve yeni adları (sıralı) verir.
func attach(work, job, id string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(work, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(work, p)
		name := id + "_" + strings.ReplaceAll(filepath.ToSlash(rel), "/", "_")
		names = append(names, name)
		return os.Rename(p, filepath.Join(job, name))
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, os.RemoveAll(work)
}

// write kaydı kendi dosyasına (girintili) ve dizine (tek satır) yazar.
func write(r Record, path, index string) error {
	if err := os.WriteFile(path, encode(r, "  "), 0o644); err != nil {
		return err
	}
	f, err := os.OpenFile(index, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(encode(r, ""))
	return err
}

// encode kaydı JSON'a çevirir (indent boşsa tek satır); HTML kaçışı yoktur, komuttaki '&&' ve
// '>' okunduğu gibi kalır.
func encode(r Record, indent string) []byte {
	var b strings.Builder
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", indent)
	e.Encode(r) // Record'un alanları her zaman kodlanabilir
	return []byte(b.String())
}

// gitOut git komutunun kırpılmış çıktısı ("" hatada).
func gitOut(args ...string) string {
	b, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
