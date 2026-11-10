package assistant_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Ekranın olay kimliği kuralı (contract.md, "Kopma ve yeniden bağlanma"): kimlik `<ön ek>-<sıra>` metnidir. Kuralı
// index.html'deki iki saf fonksiyon yazar (splitId, eventStep); test onları işaretlerin arasından alıp node ile koşar.
// Node yoksa atlanır (otomatik sınamada node vardır; emsal cmd/trlang/e2e'nin TestExtensionJSTests'i).

const (
	idBlockStart = "// ---------- Olay kimliği ----------"
	idBlockEnd   = "// ---------- Olay kimliği sonu ----------"
	prefixP      = "3f9a0c2e7b1d4e65"
	prefixQ      = "c41b77e0a9d25f13"
)

type idStep struct {
	Apply     bool    `json:"apply"`
	LastID    *string `json:"lastId"`
	Reconnect bool    `json:"reconnect"`
}

type idEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// runEventSteps olay dizisini ekranın kuralıyla sırayla işler: her adımın son kimliği sonrakinin girdisidir.
func runEventSteps(t *testing.T, lastID *string, events []idEvent) []idStep {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node bulunamadı; ekranın kimlik kuralı koşulmadı")
	}
	screen := readFile(t, screenPath)
	_, rest, ok := strings.Cut(screen, idBlockStart)
	block, _, found := strings.Cut(rest, idBlockEnd)
	if !ok || !found {
		t.Fatalf("%s: %q ... %q block not found", screenPath, idBlockStart, idBlockEnd)
	}
	input, _ := json.Marshal(map[string]any{"last": lastID, "events": events})
	driver := block + `
const input = JSON.parse(process.argv[2]);
let last = input.last;
const out = [];
for (const e of input.events) {
  const step = eventStep(last, e.id, e.type);
  last = step.lastId;
  out.push(step);
}
process.stdout.write(JSON.stringify(out));
`
	file := filepath.Join(t.TempDir(), "event_id.js")
	if err := os.WriteFile(file, []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, file, string(input)).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, output)
	}
	var steps []idStep
	if err := json.Unmarshal(output, &steps); err != nil {
		t.Fatalf("node output %q: %v", output, err)
	}
	return steps
}

func str(s string) *string { return &s }

func describe(s idStep) string {
	last := "null"
	if s.LastID != nil {
		last = *s.LastID
	}
	b, _ := json.Marshal(map[string]any{"apply": s.Apply, "lastId": last, "reconnect": s.Reconnect})
	return string(b)
}

func checkSteps(t *testing.T, name string, got []idStep, want []idStep) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d steps, want %d", name, len(got), len(want))
	}
	for i := range want {
		if describe(got[i]) != describe(want[i]) {
			t.Errorf("%s, step %d: got %s, want %s", name, i+1, describe(got[i]), describe(want[i]))
		}
	}
}

func TestEventIdOrder(t *testing.T) {
	p := func(n string) string { return prefixP + "-" + n }
	q := func(n string) string { return prefixQ + "-" + n }
	apply := func(id string) idStep { return idStep{Apply: true, LastID: str(id)} }
	skip := func(id string) idStep { return idStep{LastID: str(id)} }
	reset := idStep{Reconnect: true}

	// İlk bağlantı: ilk olay session'dır; ondan önce gelen başka olay uygulanmaz, ekran Last-Event-ID'siz yeniden bağlanır.
	checkSteps(t, "first contact", runEventSteps(t, nil, []idEvent{
		{p("4"), "turn_text"}, {p("0"), "session"}, {p("1"), "state"}, {p("2"), "turn_text"},
	}), []idStep{reset, apply(p("0")), apply(p("1")), apply(p("2"))})

	// Aynı sunucu: sıra sayı olarak karşılaştırılır (metin olarak "10" < "9"); büyük olmayan sıra atılır.
	checkSteps(t, "same broadcaster", runEventSteps(t, str(p("9")), []idEvent{
		{p("10"), "state"}, {p("10"), "state"}, {p("3"), "turn_text"}, {p("9007199254740993"), "state"},
	}), []idStep{apply(p("10")), skip(p("10")), skip(p("10")), apply(p("9007199254740993"))})

	// Sunucu yeniden başladı: eski kimlikle bağlanan ekran görüntüyü alır; session'ın ve ondan sonraki yeni ön ekli,
	// sırası eskiden küçük olayların hepsi uygulanır.
	checkSteps(t, "restarted server", runEventSteps(t, str(p("57")), []idEvent{
		{q("2"), "session"}, {q("3"), "turn_start"}, {q("4"), "turn_text"},
	}), []idStep{apply(q("2")), apply(q("3")), apply(q("4"))})

	// Sunucu session vermeden ön eki farklı bir olay gönderir: uygulanmaz, son kimlik silinir (sonraki bağlantı
	// Last-Event-ID'siz), yeniden bağlanılır; gelen session yeni ön eki kurar.
	checkSteps(t, "foreign prefix without a session", runEventSteps(t, str(p("57")), []idEvent{
		{q("1"), "turn_text"}, {q("1"), "session"}, {q("2"), "turn_end"},
	}), []idStep{reset, apply(q("1")), apply(q("2"))})

	// Bozuk kimlik ön eki tutmayan kimlik gibidir.
	checkSteps(t, "malformed ids", runEventSteps(t, str(p("5")), []idEvent{
		{"", "state"}, {p("6"), "session"}, {prefixP, "state"}, {p("7"), "session"}, {p("x"), "state"},
	}), []idStep{reset, apply(p("6")), reset, apply(p("7")), reset})
}
