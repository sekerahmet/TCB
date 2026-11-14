package files

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestSha256KnownValues: sha256(b) bilinen değerleri 64 küçük onaltılık karakterle verir (22.5).
func TestSha256KnownValues(t *testing.T) {
	cases := map[string]string{
		`TR_sha256(TR_Bytes([]))`:    `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`,
		`TR_sha256(TR_Bytes("abc"))`: `ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad`,
	}
	for code, want := range cases {
		if got := mustRunTR(t, code); got != want {
			t.Errorf("%s = %s, beklenen %s", code, got, want)
		}
	}
	errContains(t, `TR_sha256("abc")`)
	errContains(t, `TR_sha256()`)
}

// TestSha256FileStreams: dosya_sha256 dosyayı akışla okur (bellek dosyanın boyundan bağımsızdır),
// diskteki baytı karmalar (.gz açılmaz), klasörde H7729, olmayan dosyada okuma hatası verir (22.5).
func TestSha256FileStreams(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "buyuk.bin")
	data := bytes.Repeat([]byte("0123456789abcdef"), 2<<20) // 32 MiB
	if err := os.WriteFile(big, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := mustRunTR(t, `TR_sha256_file("`+filepath.ToSlash(big)+`")`)
	runtime.ReadMemStats(&after)
	if got != want {
		t.Fatalf("dosya_sha256 = %s, beklenen %s", got, want)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 8<<20 {
		t.Errorf("dosya_sha256 32 MiB'lık dosya için %d bayt ayırdı; akışla okumalıydı", grew)
	}

	gz := filepath.Join(dir, "not.txt.gz")
	var packed bytes.Buffer
	w := gzip.NewWriter(&packed)
	w.Write([]byte("merhaba"))
	w.Close()
	if err := os.WriteFile(gz, packed.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	rawSum := sha256.Sum256(packed.Bytes())
	if got := mustRunTR(t, `TR_sha256_file("`+filepath.ToSlash(gz)+`")`); got != hex.EncodeToString(rawSum[:]) {
		t.Errorf(".gz açılmamalı: %s", got)
	}

	errHas(t, `TR_sha256_file("`+filepath.ToSlash(dir)+`")`, messages.H7729)
	errHas(t, `TR_sha256_file("`+filepath.ToSlash(filepath.Join(dir, "yok.bin"))+`")`, messages.H2100)
	errHas(t, `TR_sha256_file("")`, messages.H7124)
}
