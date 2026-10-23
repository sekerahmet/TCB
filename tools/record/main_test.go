package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain alt süreç kipini taşır: sınama ikilisi TR_RECORD_HELPER ile çağrılınca kaydedilen
// komut gibi davranır (geçici klasöre metrics.json ve iki dosya yazar, çıktı verir, 3 ile çıkar).
func TestMain(m *testing.M) {
	if os.Getenv("TR_RECORD_HELPER") == "1" {
		dir := os.Getenv(envDir)
		os.WriteFile(filepath.Join(dir, "metrics.json"), []byte(`{"steps":[{"name":"a","seconds":2}],"passed":false}`), 0o644)
		os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{}`), 0o644)
		os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
		os.WriteFile(filepath.Join(dir, "sub", "x.txt"), []byte("x"), 0o644)
		os.Stdout.WriteString("helper output\n")
		os.Exit(3)
	}
	os.Exit(m.Run())
}

// Kural: kayıt aracı komutu çalıştırır ve çıkış kodunu geçirir. Her koşu iş klasöründe zaman
// adlıdır: <zaman>.json (kayıt, metrics.json gömülü), <zaman>.log (çıktı), <zaman>_<dosya> (komutun
// bıraktıkları; geçici klasör silinir). Her koşu index.jsonl'a bir satır ekler.
func TestRunWritesRecordAndIndex(t *testing.T) {
	out := t.TempDir()
	t.Setenv("TR_RECORD_HELPER", "1")
	self, _ := os.Executable()
	var stdout bytes.Buffer
	for i := 0; i < 2; i++ {
		code, err := run(options{name: "deneme", out: out}, []string{self}, &stdout)
		if err != nil || code != 3 {
			t.Fatalf("run: code %d, err %v", code, err)
		}
	}
	if !strings.Contains(stdout.String(), "helper output") {
		t.Errorf("command output not passed through: %q", stdout.String())
	}
	lines := readIndex(t, filepath.Join(out, "index.jsonl"))
	if len(lines) != 2 {
		t.Fatalf("index has %d lines, want 2", len(lines))
	}
	r := lines[0]
	if r.Schema != schemaVersion || r.Name != "deneme" || r.ExitCode != 3 || r.Machine.Cores == 0 || r.Machine.Go == "" {
		t.Errorf("record fields: %+v", r)
	}
	if !strings.Contains(string(r.Metrics), `"passed":false`) {
		t.Errorf("metrics not embedded: %s", r.Metrics)
	}
	id := r.Started.Format(stamp)
	want := []string{"deneme/" + id + ".json", "deneme/" + id + ".log", "deneme/" + id + "_metrics.json",
		"deneme/" + id + "_profile.json", "deneme/" + id + "_sub_x.txt"}
	if strings.Join(r.Files, ",") != strings.Join(want, ",") {
		t.Errorf("files %v, want %v", r.Files, want)
	}
	for _, f := range r.Files {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("listed file missing: %v", err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(out, "deneme", id+".log")); !strings.Contains(string(b), "helper output") {
		t.Errorf("log: %q", b)
	}
	entries, _ := os.ReadDir(filepath.Join(out, "deneme"))
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("temporary folder left: %s", e.Name())
		}
	}
}

func readIndex(t *testing.T, path string) []Record {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []Record
	s := bufio.NewScanner(f)
	for s.Scan() {
		var r Record
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			t.Fatalf("index line is not a record: %v", err)
		}
		lines = append(lines, r)
	}
	return lines
}

// Kural: adı geçersiz ya da komutu olmayan koşu klasör açmadan reddedilir.
func TestRunRejectsBadInput(t *testing.T) {
	out := t.TempDir()
	if _, err := run(options{name: "a b", out: out}, []string{"x"}, &bytes.Buffer{}); err == nil {
		t.Error("name with a space accepted")
	}
	if _, err := run(options{name: "a", out: out}, nil, &bytes.Buffer{}); err == nil {
		t.Error("missing command accepted")
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("rejected runs left %d entries", len(entries))
	}
}

// Kural: nvidia-smi'nin CSV satırları kartlara ayrıştırılır; bozuk satır atlanır.
func TestParseGPUs(t *testing.T) {
	got := parseGPUs("NVIDIA A100-SXM4-40GB, 40960, 550.54.15\nbozuk satır\nNVIDIA L4, 23034, 550.54.15\n")
	if len(got) != 2 || got[0].Name != "NVIDIA A100-SXM4-40GB" || got[0].MemoryMiB != 40960 || got[1].Driver != "550.54.15" {
		t.Errorf("parseGPUs: %+v", got)
	}
}
