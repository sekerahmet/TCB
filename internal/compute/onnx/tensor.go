package onnx

import "encoding/binary"

// pending okunmakta olan bir tensördür: protobuf alanları her sırada gelebilir, denetim sonda
// yapılır (complete).
type pending struct {
	Tensor
	dims            int    // dosyanın bildirdiği eksen sayısı; Dims en çok MaxDims tutar
	external        bool   // external_data var ya da data_location EXTERNAL
	raw             bool   // raw_data geldi
	floats, doubles []byte // float_data, double_data: küçük uçlu öğeler
	int32s, int64s  []byte // int32_data, int64_data: ardışık, denetlenmiş varint'ler
}

// plain Go öbeğinden yer verir (Constant'ların değerleri küçüktür).
func plain(n int) ([]byte, any) { return make([]byte, n), nil }

// tensor bir TensorProto'yu okur; raw_data'nın yerini alloc verir.
func (d *decoder) tensor(end int64, alloc func(int) ([]byte, any)) (*pending, error) {
	p := &pending{}
	for d.pos < end {
		at := d.pos
		field, wire, err := d.tag("TensorProto", end)
		if err != nil {
			return nil, err
		}
		switch field {
		case 1:
			err = d.varints(wire, at, "TensorProto.dims", end, func(v uint64) {
				if p.dims++; p.dims <= MaxDims {
					p.Dims = append(p.Dims, v)
				}
			})
		case 2:
			var v uint64
			if err = d.want(wire, wireVarint, "TensorProto.data_type", at); err == nil {
				v, err = d.varint("TensorProto.data_type", end)
				p.Type = DataType(int32(v))
			}
		case 8:
			p.Name, err = d.text(wire, at, "TensorProto.name", end)
		case 9:
			err = d.message(wire, at, "TensorProto.raw_data", end, func(stop int64) error {
				buf, owner := alloc(int(stop - d.pos))
				p.Data, p.Owner, p.raw = buf, owner, true
				return d.full(buf, "TensorProto.raw_data")
			})
		case 4:
			p.floats, err = d.fixed(wire, at, 4, p.floats, "TensorProto.float_data", end)
		case 10:
			p.doubles, err = d.fixed(wire, at, 8, p.doubles, "TensorProto.double_data", end)
		case 5:
			err = d.varints(wire, at, "TensorProto.int32_data", end, func(v uint64) { p.int32s = binary.AppendUvarint(p.int32s, v) })
		case 7:
			err = d.varints(wire, at, "TensorProto.int64_data", end, func(v uint64) { p.int64s = binary.AppendUvarint(p.int64s, v) })
		case 13: // external_data
			p.external = true
			err = d.skip(wire, "TensorProto.external_data", end)
		case 14: // data_location: 1 = EXTERNAL
			var v uint64
			if err = d.want(wire, wireVarint, "TensorProto.data_location", at); err == nil {
				v, err = d.varint("TensorProto.data_location", end)
				p.external = p.external || v == 1
			}
		default:
			err = d.skip(wire, "TensorProto", end)
		}
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

// attribute Constant düğümünün bir özniteliğini okur ve değerini tensör olarak verir: `value`
// (tensör), `value_float`, `value_floats`, `value_int`, `value_ints`; öbür öznitelikler nil'dir.
func (d *decoder) attribute(end int64) (*pending, error) {
	var name string
	var t *pending
	var f, floats, i, ints []byte
	count := 0
	for d.pos < end {
		at := d.pos
		field, wire, err := d.tag("AttributeProto", end)
		if err != nil {
			return nil, err
		}
		switch field {
		case 1:
			name, err = d.text(wire, at, "AttributeProto.name", end)
		case 2:
			f, err = d.fixed(wire, at, 4, nil, "AttributeProto.f", end)
		case 3:
			var v uint64
			if err = d.want(wire, wireVarint, "AttributeProto.i", at); err == nil {
				v, err = d.varint("AttributeProto.i", end)
				i = binary.AppendUvarint(nil, v)
			}
		case 5:
			if name != "" && name != "value" {
				err = d.skip(wire, "AttributeProto.t", end)
				break
			}
			err = d.message(wire, at, "AttributeProto.t", end, func(stop int64) (err error) {
				t, err = d.tensor(stop, plain)
				return err
			})
		case 7:
			floats, err = d.fixed(wire, at, 4, floats, "AttributeProto.floats", end)
		case 8:
			err = d.varints(wire, at, "AttributeProto.ints", end, func(v uint64) {
				ints = binary.AppendUvarint(ints, v)
				count++
			})
		default:
			err = d.skip(wire, "AttributeProto", end)
		}
		if err != nil {
			return nil, err
		}
	}
	switch {
	case name == "value":
		return t, nil
	case name == "value_float" && f != nil:
		return &pending{Tensor: Tensor{Type: Float}, floats: f}, nil
	case name == "value_floats":
		return &pending{Tensor: Tensor{Type: Float, Dims: []uint64{uint64(len(floats) / 4)}}, dims: 1, floats: floats}, nil
	case name == "value_int" && i != nil:
		return &pending{Tensor: Tensor{Type: Int64}, int64s: i}, nil
	case name == "value_ints":
		return &pending{Tensor: Tensor{Type: Int64, Dims: []uint64{uint64(count)}}, dims: 1, int64s: ints}, nil
	}
	return nil, nil
}

// varints tek ya da paketli (boylu) varint alanını okur.
func (d *reader) varints(wire int, at int64, what string, end int64, each func(uint64)) error {
	switch wire {
	case wireVarint:
		v, err := d.varint(what, end)
		if err == nil {
			each(v)
		}
		return err
	case wireLen:
		n, err := d.length(what, end)
		if err != nil {
			return err
		}
		for stop := d.pos + n; d.pos < stop; {
			v, err := d.varint(what, stop)
			if err != nil {
				return err
			}
			each(v)
		}
		return nil
	}
	return d.invalid(what, at)
}

// fixed tek ya da paketli sabit boylu (4 ya da 8 bayt) alanı buf'un sonuna ekler.
func (d *reader) fixed(wire int, at int64, size int, buf []byte, what string, end int64) ([]byte, error) {
	var n int64
	switch {
	case wire == wireFixed32 && size == 4, wire == wireFixed64 && size == 8:
		if n = int64(size); n > end-d.pos {
			return nil, d.short(what, end)
		}
	case wire == wireLen:
		var err error
		if n, err = d.length(what, end); err != nil {
			return nil, err
		}
		if n%int64(size) != 0 {
			return nil, d.invalid(what, at)
		}
	default:
		return nil, d.invalid(what, at)
	}
	start := len(buf)
	buf = append(buf, make([]byte, n)...)
	return buf, d.full(buf[start:], what)
}
