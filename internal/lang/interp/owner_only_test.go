package interp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// TestWriteFileOwnerOnly: TR_owner_only: dosyayı yalnız kullanıcıya açık yazar; var olanın izinleri
// daralır, geçici dosya kalmaz (18.13).
func TestWriteFileOwnerOnly(t *testing.T) {
	folder := t.TempDir()
	for _, existing := range []bool{false, true} {
		path := filepath.Join(folder, map[bool]string{false: "yeni.txt", true: "eski.txt"}[existing])
		if existing {
			if err := os.WriteFile(path, []byte("eski"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := WriteFileContent(path, []byte("gizli metin"), true); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(path); string(b) != "gizli metin" {
			t.Fatalf("içerik: %q", b)
		}
		checkOwnerOnly(t, path)
	}
	if g, _ := reporoot.ReadDir(folder); len(g) != 2 {
		t.Fatalf("geçici dosya kaldı: %v", g)
	}
}

// TestWriteFileSecretNeedsOwnerOnly: gizli değeri TR_owner_only: olmadan yazmak çağrı hatasıdır
// (H7600) ve dosya yazılmaz; argümanla yazılır. Yanlış tip ve bilinmeyen ad hatadır.
func TestWriteFileSecretNeedsOwnerOnly(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "belirtec.txt"))
	cases := []struct {
		call string
		code messages.Code
	}{
		{`TR_write_file("` + path + `", "anahtar: çokgizlideğer")`, messages.H7600},
		{`TR_write_file("` + path + `", TR_Bytes("çokgizlideğer"), TR_owner_only: TR_false)`, messages.H7600},
		{`TR_write_file("` + path + `", "a", TR_owner_only: 1)`, messages.H7905},
		{`TR_write_file("` + path + `", "a", TR_overwrite: TR_true)`, messages.H7901},
		{`TR_write_file("` + path + `", "çokgizlideğer", TR_owner_only: TR_true)`, 0},
	}
	for _, c := range cases {
		v := New()
		v.AddSecret("çokgizlideğer")
		_, err := v.Run(mustParseProgram(t, c.call))
		if c.code == 0 {
			if err != nil {
				t.Fatalf("%s: %v", c.call, err)
			}
			checkOwnerOnly(t, filepath.FromSlash(path))
			continue
		}
		if !messages.Has(err, c.code) {
			t.Fatalf("%s: %s bekleniyordu, alınan %v", c.call, c.code, err)
		}
		if _, e := os.Stat(path); e == nil {
			t.Fatalf("%s: hatalı çağrı dosya yazmamalı", c.call)
		}
	}
}
