package onnx

import (
	"bufio"
	"io"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Protobuf'un tel biçimi (https://protobuf.dev/programming-guides/encoding/): her alan bir
// etiketle (alan numarası << 3 | tel türü, varint) başlar. Tel türleri: 0 varint, 1 sabit 64 bit,
// 2 boylu (boy varint, ardından o kadar bayt), 5 sabit 32 bit; 3 ve 4 (grup) eski ve ONNX'te
// yoktur, geçersiz sayılır.
const (
	wireVarint  = 0
	wireFixed64 = 1
	wireLen     = 2
	wireFixed32 = 5

	// maxField protobuf'un en büyük alan numarasıdır (2^29 − 1).
	maxField = 1<<29 - 1
	// maxVarint bir varint'in en çok bayt sayısıdır (64 bit / 7).
	maxVarint = 10
)

// reader dosyayı baştan sona bir kez okur ve konumu tutar. Her okuma bir sınıra göredir: içinde
// bulunulan iletinin bittiği bayt (end); dosyanın sonu size'dır. Sınırı aşan okuma, sınır dosyanın
// sonuysa "dosya bitti", değilse "geçersiz kodlama" hatasıdır (H6742).
type reader struct {
	r    *bufio.Reader
	pos  int64
	size int64
}

// short end sınırında biten bir okumanın hatasıdır.
func (d *reader) short(what string, end int64) error {
	if end < d.size {
		return messages.New(messages.H6742, what, "invalid", d.pos)
	}
	return messages.New(messages.H6742, what, "end", d.pos)
}

// invalid at baytındaki geçersiz kodlamanın hatasıdır.
func (d *reader) invalid(what string, at int64) error {
	return messages.New(messages.H6742, what, "invalid", at)
}

func (d *reader) byte(what string, end int64) (byte, error) {
	if d.pos >= end {
		return 0, d.short(what, end)
	}
	b, err := d.r.ReadByte()
	if err != nil {
		return 0, messages.New(messages.H6742, what, "end", d.pos)
	}
	d.pos++
	return b, nil
}

// varint bir varint okur; 10 bayttan uzun ya da 64 biti aşan yazım geçersizdir.
func (d *reader) varint(what string, end int64) (uint64, error) {
	at := d.pos
	var v uint64
	for i := range maxVarint {
		b, err := d.byte(what, end)
		if err != nil {
			return 0, err
		}
		if i == maxVarint-1 && b > 1 {
			return 0, d.invalid(what, at)
		}
		v |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return v, nil
		}
	}
	return 0, d.invalid(what, at)
}

// tag bir alan etiketini okur.
func (d *reader) tag(what string, end int64) (field uint64, wire int, err error) {
	at := d.pos
	t, err := d.varint(what, end)
	if err != nil {
		return 0, 0, err
	}
	field, wire = t>>3, int(t&7)
	if field == 0 || field > maxField || (wire != wireVarint && wire != wireFixed64 && wire != wireLen && wire != wireFixed32) {
		return 0, 0, d.invalid(what, at)
	}
	return field, wire, nil
}

// length boylu bir alanın boyunu okur; boy içinde bulunulan iletinin kalanını aşamaz. Sınır
// dosyanın sonuysa bu H6743'tür ve ileti dosyanın kalanını söyler; iç içe bir iletinin sınırını
// aşan boy, reader'ın kuralıyla geçersiz kodlamadır (H6742): dosyada daha çok bayt olabilir.
func (d *reader) length(what string, end int64) (int64, error) {
	at := d.pos
	n, err := d.varint(what, end)
	if err != nil {
		return 0, err
	}
	if left := end - d.pos; n > uint64(left) {
		if end < d.size {
			return 0, d.invalid(what, at)
		}
		return 0, messages.New(messages.H6743, at, n, left)
	}
	return int64(n), nil
}

// discard n baytı atlar; n sınır içindedir (length denetledi ya da sabit boydur).
func (d *reader) discard(n int64, what string, end int64) error {
	if n > end-d.pos {
		return d.short(what, end)
	}
	for n > 0 {
		k := int(min(n, 1<<30))
		got, err := d.r.Discard(k)
		d.pos += int64(got)
		if err != nil {
			return messages.New(messages.H6742, what, "end", d.pos)
		}
		n -= int64(k)
	}
	return nil
}

// full buf'u doldurur; boy sınır içindedir.
func (d *reader) full(buf []byte, what string) error {
	got, err := io.ReadFull(d.r, buf)
	d.pos += int64(got)
	if err != nil {
		return messages.New(messages.H6742, what, "end", d.pos)
	}
	return nil
}

// bytes boylu bir alanın baytlarını okur.
func (d *reader) bytes(what string, end int64) ([]byte, error) {
	n, err := d.length(what, end)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	return buf, d.full(buf, what)
}

// skip tel türüne göre bir alanı atlar.
func (d *reader) skip(wire int, what string, end int64) error {
	switch wire {
	case wireVarint:
		_, err := d.varint(what, end)
		return err
	case wireFixed64:
		return d.discard(8, what, end)
	case wireFixed32:
		return d.discard(4, what, end)
	}
	n, err := d.length(what, end)
	if err != nil {
		return err
	}
	return d.discard(n, what, end)
}

// want alanın beklenen tel türüyle geldiğini denetler.
func (d *reader) want(wire, expected int, what string, at int64) error {
	if wire != expected {
		return d.invalid(what, at)
	}
	return nil
}
