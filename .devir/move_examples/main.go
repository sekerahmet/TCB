// move_examples, kodların örneklerini tr.json'dan (Türkçe görünüm) kimlik yazımıyla
// definitions.json'a taşır (Faz 0.5, 17.4). Program örnekleri bugünkü kodlayıcıyla (source.Encode)
// çevrilir; komut satırı örneklerinde (cmd, sub, opt, env, repl, dbg) Türkçe ad kimliğin adı olur.
// Bir kez koşulur, sonra kodlayıcı kalkar: go run ./.devir/move_examples
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

const (
	trPath   = "internal/lang/codes/tr.json"
	enPath   = "internal/lang/codes/en.json"
	defsPath = "internal/lang/codes/definitions.json"
)

func main() {
	examples, trOut := takeExamples(trPath)
	if _, enOut := takeExamples(enPath); enOut != "" {
		must(os.WriteFile(enPath, []byte(enOut), 0644))
	}
	names := commandNames()
	coded := map[string]string{}
	failed := 0
	for key, ex := range examples {
		k, err := codes.Parse(key)
		must(err)
		var out string
		if k.ExpectedKind() == codes.KindCommand {
			out = commandLine(k, ex, names)
		} else {
			c, err := source.Encode(ex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n  %s\n", key, err, ex)
				failed++
				continue
			}
			lines := strings.Split(strings.TrimSuffix(c, "\n"), "\n")
			if len(lines) != 2 || lines[0] != source.Header {
				fmt.Fprintf(os.Stderr, "%s: unexpected form %q\n", key, c)
				failed++
				continue
			}
			out = lines[1]
		}
		coded[key] = out
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d examples could not be converted\n", failed)
		os.Exit(1)
	}
	must(os.WriteFile(trPath, []byte(trOut), 0644))
	must(os.WriteFile(defsPath, []byte(addExamples(defsPath, coded)), 0644))
	fmt.Printf("%d examples moved\n", len(coded))
}

// takeExamples dil dosyasının satırlarından "example" alanını çıkarır; örnekleri ve yeni metni verir.
func takeExamples(path string) (map[string]string, string) {
	data, err := os.ReadFile(path)
	must(err)
	out := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		at := strings.Index(line, `, "example": `)
		if at < 0 {
			continue
		}
		key, entry := entryOf(line)
		var e map[string]any
		must(json.Unmarshal([]byte(entry), &e))
		out[key] = e["example"].(string)
		end := strings.LastIndex(line, " }")
		lines[i] = line[:at] + line[end:]
	}
	if len(out) == 0 {
		return out, ""
	}
	return out, strings.Join(lines, "\n")
}

// entryOf '  "TR_x": { ... },' satırının anahtarını ve değerini verir.
func entryOf(line string) (string, string) {
	s := strings.TrimSpace(line)
	s = strings.TrimSuffix(s, ",")
	i := strings.Index(s, ": ")
	return strings.Trim(s[:i], `"`), s[i+2:]
}

// addExamples definitions.json'un satırlarına "example" alanını son alan olarak ekler.
func addExamples(path string, coded map[string]string) string {
	data, err := os.ReadFile(path)
	must(err)
	lines := strings.Split(string(data), "\n")
	done := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, `  "TR_`) {
			continue
		}
		key, _ := entryOf(line)
		ex, ok := coded[key]
		if !ok {
			continue
		}
		end := strings.LastIndex(line, " }")
		lines[i] = line[:end] + `, "example": ` + jsonString(ex) + line[end:]
		done++
	}
	if done != len(coded) {
		fmt.Fprintf(os.Stderr, "%d examples, %d written to the definitions\n", len(coded), done)
		os.Exit(1)
	}
	return strings.Join(lines, "\n")
}

func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	must(enc.Encode(s))
	return strings.TrimSuffix(b.String(), "\n")
}

// commandNames komut satırı kodlarının Türkçe yazımından kimliğin adına giden tablodur: komut,
// alt komut, kabuk ve hata ayıklayıcı komutu sınıfsız kısa ad ("check"), seçenek "--" ve tireli ad
// ("--go-profile"), ortam değişkeni "TR_" ve ad ("TR_LANG").
func commandNames() map[string]string {
	tr := codes.Source()
	out := map[string]string{}
	for _, k := range codes.Codes() {
		if k.ExpectedKind() != codes.KindCommand {
			continue
		}
		id := k.String()
		short := id[strings.Index(id[3:], "_")+4:]
		name := tr.Name(k)
		switch k.Class() {
		case codes.ClassShell, codes.ClassDebugger:
			out[string(k.Class())+" "+name] = short
		case codes.ClassOption:
			out["--"+strings.ReplaceAll(name, "_", "-")] = "--" + strings.ReplaceAll(short, "_", "-")
		case codes.ClassEnvironment:
			out[name] = "TR_" + short
		default:
			out[name] = short
		}
	}
	return out
}

// commandLine komut satırı örneğinin sözcüklerini kimlik adlarına çevirir; ortam değişkeni atamasının
// adı da çevrilir; kabuk ve hata ayıklayıcı satırında yalnız ilk sözcük komuttur, öbür sözcükler (dosya, kullanıcı adı, değer) kalır.
func commandLine(k codes.Code, ex string, names map[string]string) string {
	words := strings.Split(ex, " ")
	if c := k.Class(); c == codes.ClassShell || c == codes.ClassDebugger {
		// Kabuk ve hata ayıklayıcı komutunun adı yalnız kendi sınıfında aranır (çıkış iki sınıfta da var).
		words[0] = names[string(c)+" "+words[0]]
		return strings.Join(words, " ")
	}
	for i, w := range words {
		if name, value, ok := strings.Cut(w, "="); ok && strings.HasPrefix(name, "TR_") {
			if n, ok := names[name]; ok {
				words[i] = n + "=" + value
			}
			continue
		}
		if n, ok := names[w]; ok && (i == 0 || i == 1 || i == 2 || strings.HasPrefix(w, "--")) {
			words[i] = n
		}
	}
	return strings.Join(words, " ")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
