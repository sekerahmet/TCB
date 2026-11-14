package dataload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

type errReaderAt struct {
	prefix []byte
	err    error
}

func (r errReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < int64(len(r.prefix)) {
		n := copy(p, r.prefix[off:])
		if n == len(p) {
			return n, nil
		}
		return n, r.err
	}
	return 0, r.err
}

func assertExactCodedText(t *testing.T, err error, want *messages.Error, root messages.Code) {
	t.Helper()
	if !messages.Has(err, want.Code) {
		t.Fatalf("expected outer code %v, got: %v", want.Code, err)
	}
	gotRoot, ok := messages.Root(err)
	if !ok || gotRoot.Code != root {
		t.Fatalf("expected root %v, got %v (ok=%v): %v", root, gotRoot, ok, err)
	}
	for _, lang := range []string{"tr", "en"} {
		got := messages.ErrorIn(err, lang)
		exp := messages.ErrorIn(want, lang)
		if got != exp {
			t.Fatalf("lang %s: got %q, want exact catalog text %q", lang, got, exp)
		}
	}
}

func TestDataloadSystemErrorsCarryNoSystemText(t *testing.T) {
	testCacheAndDatasetSystemErrors(t)
	testHeaderAndShardSystemErrors(t)
}

func testCacheAndDatasetSystemErrors(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenCache(CacheOptions{Dir: filepath.Join(blocker, "sub"), Quota: 4096}); err == nil {
		t.Fatal("OpenCache under a regular file succeeded")
	} else if !messages.Has(err, messages.H5006) {
		t.Fatalf("expected H5006, got %v", err)
	}

	var raw bytes.Buffer
	if err := WriteNpyI32(&raw, []int32{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "p0.npy")
	if err := os.WriteFile(src, raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache")
	c, err := OpenCache(CacheOptions{Dir: cacheDir, Quota: 1 << 20, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, _, err = c.Fetch(context.Background(), filepath.Join(dir, "missing.npy"))
	assertExactCodedText(t, err, messages.New(messages.H5010, messages.New(messages.H2132)), messages.H2132)

	targetDir := filepath.Join(cacheDir, localName(src))
	if err := os.MkdirAll(filepath.Join(targetDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Fetch(context.Background(), src)
	assertExactCodedText(t, err, messages.New(messages.H5013, messages.New(messages.H2130)), messages.H2130)
	var le *os.LinkError
	if !errors.As(err, &le) {
		t.Fatalf("expected original *os.LinkError in Unwrap chain, got %T: %v", err, err)
	}

	_, err = ReadMeta(targetDir)
	assertExactCodedText(t, err, messages.New(messages.H5017, messages.New(messages.H2130)), messages.H2130)
	_, err = OpenDataset(filepath.Join(dir, "missing_ds"), DatasetOptions{})
	assertExactCodedText(t, err, messages.New(messages.H5019, messages.New(messages.H2132)), messages.H2132)
}

func testHeaderAndShardSystemErrors(t *testing.T) {
	t.Helper()
	permErr := &fs.PathError{Op: "read", Path: "shard.npy", Err: fs.ErrPermission}
	_, err := ReadHeader(errReaderAt{err: permErr})
	assertExactCodedText(t, err, messages.New(messages.H5030, messages.New(messages.H2133)), messages.H2133)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected fs.ErrPermission in Unwrap chain: %v", err)
	}

	var hdr bytes.Buffer
	if err := WriteHeader(&hdr, DTypeI32, 8, 1); err != nil {
		t.Fatal(err)
	}
	rawDiskErr := errors.New("raw controller glitch")
	_, err = ReadHeader(errReaderAt{prefix: hdr.Bytes()[:headerPrefixV2], err: rawDiskErr})
	assertExactCodedText(t, err, messages.New(messages.H5030, messages.New(messages.H2629)), messages.H2629)
	if !errors.Is(err, rawDiskErr) {
		t.Fatalf("expected rawDiskErr in Unwrap chain: %v", err)
	}
	if _, err = ReadHeader(bytes.NewReader(hdr.Bytes()[:headerPrefixV2])); !messages.Has(err, messages.H5056) {
		t.Fatalf("short EOF header must stay H5056, got %v", err)
	}

	dir := t.TempDir()
	_, err = OpenShard(filepath.Join(dir, "missing.npy"), ShardOptions{NoMmap: true})
	assertExactCodedText(t, err, messages.New(messages.H5024, messages.New(messages.H2132)), messages.H2132)
	_, err = OpenShard(dir, ShardOptions{NoMmap: true})
	assertExactCodedText(t, err, messages.New(messages.H5030, messages.New(messages.H2130)), messages.H2130)

	var full bytes.Buffer
	if err := WriteNpyI32(&full, []int32{10, 20, 30, 40, 50, 60}); err != nil {
		t.Fatal(err)
	}
	shardPath := filepath.Join(dir, "cut.npy")
	if err := os.WriteFile(shardPath, full.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sh, err := OpenShard(shardPath, ShardOptions{NoMmap: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sh.Close()
	if err := os.Truncate(shardPath, sh.Header().DataOffset+4); err != nil {
		t.Fatal(err)
	}
	err = sh.Read(make([]int32, 4), 0)
	assertExactCodedText(t, err, messages.New(messages.H5089, shardPath, messages.New(messages.H2153)), messages.H2153)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF in Unwrap chain: %v", err)
	}
}
