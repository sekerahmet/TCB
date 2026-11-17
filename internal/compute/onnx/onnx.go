// Package onnx bir ONNX modelinin (protobuf, onnx.proto3) tensörlerini okur (dil tanımı 15.47):
// ağırlıklar (GraphProto.initializer) adlarıyla, Constant düğümlerinin değerleri çıkış adlarıyla
// gelir. Adı `onnx::` ile başlayan (dışa vericinin adsız bıraktığı) tensör, onu girdi olarak kullanan
// ilk düğümün adıyla ve girdi sırasıyla adlanır: `/flow/flows.0/enc/in_layers.0/Conv:1`. Düğümler ve
// hesap okunmaz; alt çizgeler (If, Loop), seyrek ağırlıklar ve işlevler de okunmaz.
//
// Dosya güvenilmeyen girdidir: protobuf'un tel biçimi elle, bağımlılıksız çözülür; her boy
// içinde bulunduğu iletinin kalanıyla denetlenir (H6743), dosya bir kez, baştan sona okunur ve
// ayırma dosyadaki baytla orantılıdır: ağırlık verisi bildirilen boyda (dosyada vardır), varint'li
// tam sayı alanları en çok 8 katı, tensör kaydı en çok MaxTensors tanedir.
//
// Layer: compute core (internal/compute); yalnızca ileti kataloğunu içe aktarır.
package onnx

import (
	"bufio"
	"encoding/binary"
	"math/big"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// DataType TensorProto.DataType'ın değeridir (onnx.proto3).
type DataType int32

// Okunan veri türleri.
const (
	Float    DataType = 1
	Uint8    DataType = 2
	Int8     DataType = 3
	Int16    DataType = 5
	Int32    DataType = 6
	Int64    DataType = 7
	Bool     DataType = 9
	Float16  DataType = 10
	Double   DataType = 11
	BFloat16 DataType = 16
)

// elemSize okunan türlerin öğe boyudur (bayt); listede olmayan tür H6746'dır.
var elemSize = map[DataType]int{Float: 4, Uint8: 1, Int8: 1, Int16: 2, Int32: 4, Int64: 8, Bool: 1,
	Float16: 2, Double: 8, BFloat16: 2}

// typeNames onnx.proto3'ün tür adlarıdır: H6746 dosyadaki türü adıyla söyler.
var typeNames = []string{"UNDEFINED", "FLOAT", "UINT8", "INT8", "UINT16", "INT16", "INT32", "INT64", "STRING",
	"BOOL", "FLOAT16", "DOUBLE", "UINT32", "UINT64", "COMPLEX64", "COMPLEX128", "BFLOAT16", "FLOAT8E4M3FN",
	"FLOAT8E4M3FNUZ", "FLOAT8E5M2", "FLOAT8E5M2FNUZ", "UINT4", "INT4", "FLOAT4E2M1"}

func (t DataType) String() string {
	if t >= 0 && int(t) < len(typeNames) {
		return typeNames[t]
	}
	return strconv.Itoa(int(t))
}

const (
	// MaxTensors bir dosyanın en çok tensör sayısıdır (ağırlık ve Constant birlikte). Gerçek
	// seslerde 401 ağırlık var; sınır boş kayıtlarla dev ayırmayı önler.
	MaxTensors = 1 << 16
	// MaxDims bir tensörün en çok eksen sayısıdır (TR'nin sınırı, 15.46).
	MaxDims = 32

	anonPrefix = "onnx::"
)

// Tensor dosyadaki bir tensördür.
type Tensor struct {
	Name string
	// Dims dosyadaki sırayla eksen boylarıdır; eksi eksen dosyada 2^64'e sarılmış görünür.
	Dims []uint64
	Type DataType
	// Data öğelerin küçük uçlu baytlarıdır; boyu eksenlerin çarpımı × öğe boyudur.
	Data []byte
	// Owner Data'yı Options.Alloc verdiyse onun sahibidir.
	Owner any
}

// Options çözümün çağırana bağlı yanlarıdır.
type Options struct {
	// Alloc bir ağırlığın raw_data'sı için n baytlık yer verir; nil ise Go öbeği kullanılır.
	Alloc func(n int) ([]byte, any)
	// Check tamamlanan tensörü, şekli ve türü bilinince, veri boyu denetlenmeden önce denetler.
	Check func(t *Tensor) error
}

// decoder bir dosyanın çözüm durumudur.
type decoder struct {
	reader
	opt     Options
	tensors []Tensor
	count   int               // görülen tensör sayısı; MaxTensors'u aşınca tensörler atlanır, sayılır
	users   map[string]string // adsız tensör → ilk kullananın adı ve girdi sırası ("": düğümün adı yok)
}

// Decode boyutu size bayt olan ONNX dosyasını r'den çözer; tensörler dosyadaki sırayla döner.
func Decode(r *bufio.Reader, size int64, opt Options) ([]Tensor, error) {
	if opt.Alloc == nil {
		opt.Alloc = func(n int) ([]byte, any) { return make([]byte, n), nil }
	}
	d := &decoder{reader: reader{r: r, size: size}, opt: opt, users: map[string]string{}}
	for d.pos < size {
		at := d.pos
		field, wire, err := d.tag("ModelProto", size)
		if err == nil {
			if field == 7 { // graph
				err = d.message(wire, at, "ModelProto.graph", size, d.graph)
			} else {
				err = d.skip(wire, "ModelProto", size)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if d.count > MaxTensors {
		return nil, messages.New(messages.H6744, d.count, MaxTensors)
	}
	return d.finish()
}

// message boylu bir alt iletiyi each ile okur.
func (d *reader) message(wire int, at int64, what string, end int64, each func(end int64) error) error {
	if err := d.want(wire, wireLen, what, at); err != nil {
		return err
	}
	n, err := d.length(what, end)
	if err != nil {
		return err
	}
	return each(d.pos + n)
}

// text boylu bir metin alanını okur.
func (d *reader) text(wire int, at int64, what string, end int64) (string, error) {
	if err := d.want(wire, wireLen, what, at); err != nil {
		return "", err
	}
	b, err := d.bytes(what, end)
	return string(b), err
}

// graph GraphProto'yu okur: düğümler ve ağırlıklar; öbür alanlar atlanır.
func (d *decoder) graph(end int64) error {
	for d.pos < end {
		at := d.pos
		field, wire, err := d.tag("GraphProto", end)
		if err == nil {
			switch field {
			case 1:
				err = d.message(wire, at, "GraphProto.node", end, d.node)
			case 5:
				err = d.message(wire, at, "GraphProto.initializer", end, d.initializer)
			default:
				err = d.skip(wire, "GraphProto", end)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// initializer bir ağırlığı okur.
func (d *decoder) initializer(end int64) error {
	d.count++
	if d.count > MaxTensors {
		return d.discard(end-d.pos, "GraphProto.initializer", end)
	}
	p, err := d.tensor(end, d.opt.Alloc)
	if err != nil {
		return err
	}
	return d.add(p)
}

// anonUse adsız bir tensörün bir düğümdeki girdi sırasıdır.
type anonUse struct {
	name  string
	index int
}

// node bir düğümü okur: adsız girdilerin ilk kullananı kaydedilir; Constant'ın değeri tensördür.
func (d *decoder) node(end int64) error {
	var name, op, domain, output string
	var anon []anonUse
	var value *pending
	inputs, outputs := 0, 0
	// Sınır aşıldıysa (over) değer okunmaz, Constant yalnız sayılır.
	over, attributes := d.count >= MaxTensors, false
	for d.pos < end {
		at := d.pos
		field, wire, err := d.tag("NodeProto", end)
		if err != nil {
			return err
		}
		var s string
		switch field {
		case 1:
			if s, err = d.text(wire, at, "NodeProto.input", end); strings.HasPrefix(s, anonPrefix) {
				anon = append(anon, anonUse{s, inputs})
			}
			inputs++
		case 2:
			if s, err = d.text(wire, at, "NodeProto.output", end); outputs == 0 {
				output = s
			}
			outputs++
		case 3:
			name, err = d.text(wire, at, "NodeProto.name", end)
		case 4:
			op, err = d.text(wire, at, "NodeProto.op_type", end)
		case 7:
			domain, err = d.text(wire, at, "NodeProto.domain", end)
		case 5:
			// op_type çoğunlukla öznitelikten önce gelir: Constant olmayanın özniteliği okunmaz.
			attributes = true
			if (op != "" && op != "Constant") || value != nil || over {
				err = d.skip(wire, "NodeProto.attribute", end)
				break
			}
			err = d.message(wire, at, "NodeProto.attribute", end, func(end int64) (err error) {
				value, err = d.attribute(end)
				return err
			})
		default:
			err = d.skip(wire, "NodeProto", end)
		}
		if err != nil {
			return err
		}
	}
	for _, u := range anon {
		if _, seen := d.users[u.name]; !seen {
			d.users[u.name] = ""
			if name != "" {
				d.users[u.name] = name + ":" + strconv.Itoa(u.index)
			}
		}
	}
	if op != "Constant" || (domain != "" && domain != "ai.onnx") || outputs == 0 || (value == nil && !(over && attributes)) {
		return nil
	}
	if d.count++; over {
		return nil
	}
	value.Name = output
	return d.add(value)
}

// add tamamlanan tensörü denetler ve listeye ekler.
func (d *decoder) add(p *pending) error {
	if err := d.complete(p); err != nil {
		return err
	}
	d.tensors = append(d.tensors, p.Tensor)
	return nil
}

// finish adsız tensörleri adlar ve yinelenen adı reddeder.
func (d *decoder) finish() ([]Tensor, error) {
	seen := make(map[string]bool, len(d.tensors))
	for i := range d.tensors {
		t := &d.tensors[i]
		if strings.HasPrefix(t.Name, anonPrefix) && d.users[t.Name] != "" {
			t.Name = d.users[t.Name]
		}
		if seen[t.Name] {
			return nil, messages.New(messages.H6748, t.Name)
		}
		seen[t.Name] = true
	}
	return d.tensors, nil
}

// complete okunan tensörü sırayla denetler (eksen sayısı, tür, çağıranın sınırı, dış veri, veri
// boyu) ve verisini tek bir küçük uçlu dizide toplar.
func (d *decoder) complete(p *pending) error {
	if p.dims > MaxDims {
		return messages.New(messages.H6745, p.Name, p.dims, MaxDims)
	}
	size, ok := elemSize[p.Type]
	if !ok {
		return messages.New(messages.H6746, p.Name, p.Type.String())
	}
	if d.opt.Check != nil {
		if err := d.opt.Check(&p.Tensor); err != nil {
			return err
		}
	}
	p.collect()
	want := big.NewInt(int64(size))
	for _, n := range p.Dims {
		want.Mul(want, new(big.Int).SetUint64(n))
	}
	if p.external {
		return messages.New(messages.H6747, p.Name, "external", shapeText(p.Dims), want.String(), len(p.Data))
	}
	if !want.IsUint64() || want.Uint64() != uint64(len(p.Data)) {
		return messages.New(messages.H6747, p.Name, "other", shapeText(p.Dims), want.String(), len(p.Data))
	}
	return nil
}

// shapeText eksenleri "[2, 3]" biçiminde yazar (tensor.ShapeText gibi).
func shapeText(dims []uint64) string {
	p := make([]string, len(dims))
	for i, n := range dims {
		p[i] = strconv.FormatUint(n, 10)
	}
	return "[" + strings.Join(p, ", ") + "]"
}

// collect raw_data yoksa verisini türün alanından küçük uçlu dizi olarak kurar. Seçim onnx'in
// başvurusuyla (numpy_helper.to_array) aynıdır: raw_data varsa o, yoksa türün alanı; öbür veri
// alanları okunmaz.
func (p *pending) collect() {
	if p.raw {
		return
	}
	switch p.Type {
	case Float:
		p.Data = p.floats
	case Double:
		p.Data = p.doubles
	case Int64:
		p.Data = narrow(p.int64s, 8, p.Type)
	default:
		p.Data = narrow(p.int32s, elemSize[p.Type], p.Type)
	}
}

// narrow ardışık varint'leri (denetlenmiş) öğe boyuna indirir: int32_data'daki değer türün bitlerine
// kesilir (onnx'in numpy_helper'ı gibi: `astype`), mantıkta sıfırdan farklı değer 1'dir.
func narrow(varints []byte, size int, t DataType) []byte {
	var out []byte
	for len(varints) > 0 {
		v, n := binary.Uvarint(varints)
		varints = varints[n:]
		switch {
		case t == Bool:
			out = append(out, boolByte(v))
		case size == 1:
			out = append(out, byte(v))
		case size == 2:
			out = binary.LittleEndian.AppendUint16(out, uint16(v))
		case size == 4:
			out = binary.LittleEndian.AppendUint32(out, uint32(v))
		default:
			out = binary.LittleEndian.AppendUint64(out, v)
		}
	}
	return out
}

func boolByte(v uint64) byte {
	if v != 0 {
		return 1
	}
	return 0
}
