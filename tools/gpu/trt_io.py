"""TR tensor file (.trt, version 2) reader and writer for Python.

Lets PyTorch weights (for example the target model's initial weights) be loaded in TR with
`tensör_yükle`, and TR weights be read back in Python. The format is defined in
internal/stdlib/ml/save.go:

    magic "TRTENSOR" (8 bytes), version uint16 = 2, entry count uint32, then per entry:
    name length uint32, name UTF-8, dtype uint8 (1 float32, 2 bf16, 3 fp16), ndim uint8,
    shape uint64 x ndim, data (row-major; bf16 and fp16 as 2-byte bit patterns);
    finally a CRC-32 (IEEE) of every preceding byte. All integers are little-endian.

Only numpy is needed. bf16 arrays are passed as uint16 bit patterns with dtype="bf16"
(numpy has no bfloat16), or as float32 with dtype="bf16" (rounded to nearest even).
"""
import struct
import zlib

import numpy as np

MAGIC = b"TRTENSOR"
VERSION = 2
CODES = {"float32": 1, "bf16": 2, "fp16": 3}
NAMES = {v: k for k, v in CODES.items()}


def float32_to_bf16_bits(a):
    """Rounds float32 values to bf16 (nearest even) and returns the uint16 bit patterns."""
    u = np.ascontiguousarray(a, dtype=np.float32).view(np.uint32)
    rounding = ((u >> 16) & 1) + 0x7FFF
    return ((u + rounding) >> 16).astype(np.uint16)


def bf16_bits_to_float32(bits):
    return (np.asarray(bits, dtype=np.uint16).astype(np.uint32) << 16).view(np.float32)


def _payload(array, dtype):
    if dtype == "float32":
        return np.ascontiguousarray(array, dtype="<f4").tobytes()
    if dtype == "fp16":
        return np.ascontiguousarray(array, dtype="<f2").tobytes()
    if dtype == "bf16":
        a = np.asarray(array)
        bits = a if a.dtype == np.uint16 else float32_to_bf16_bits(a)
        return np.ascontiguousarray(bits, dtype="<u2").tobytes()
    raise ValueError("unknown dtype %r (float32, bf16, fp16)" % dtype)


def write_trt(path, tensors, dtypes=None):
    """Writes {name: numpy array} to path. dtypes optionally maps a name to "bf16"/"fp16"
    (default float32). Order is the dict's order, as TR keeps it."""
    dtypes = dtypes or {}
    body = bytearray(MAGIC)
    body += struct.pack("<HI", VERSION, len(tensors))
    for name, array in tensors.items():
        dtype = dtypes.get(name, "float32")
        shape = np.shape(array)
        raw = name.encode("utf-8")
        if len(raw) > 65535 or len(shape) > 32:
            raise ValueError("%s: name or rank too large" % name)
        body += struct.pack("<I", len(raw)) + raw
        body += struct.pack("<BB", CODES[dtype], len(shape))
        body += struct.pack("<%dQ" % len(shape), *shape)
        body += _payload(array, dtype)
    body += struct.pack("<I", zlib.crc32(bytes(body)) & 0xFFFFFFFF)
    with open(path, "wb") as f:
        f.write(body)


def read_trt(path):
    """Reads a .trt file into {name: (numpy float32 array, dtype name)}."""
    data = open(path, "rb").read()
    if data[:8] != MAGIC:
        raise ValueError("%s: not a TR tensor file" % path)
    if struct.unpack("<I", data[-4:])[0] != zlib.crc32(data[:-4]) & 0xFFFFFFFF:
        raise ValueError("%s: checksum mismatch" % path)
    version, count = struct.unpack_from("<HI", data, 8)
    if version != VERSION:
        raise ValueError("%s: version %d, expected %d" % (path, version, VERSION))
    pos, out = 14, {}
    for _ in range(count):
        (n,) = struct.unpack_from("<I", data, pos)
        pos += 4
        name = data[pos:pos + n].decode("utf-8")
        pos += n
        code, ndim = struct.unpack_from("<BB", data, pos)
        pos += 2
        shape = struct.unpack_from("<%dQ" % ndim, data, pos)
        pos += 8 * ndim
        count_el = int(np.prod(shape)) if ndim else 1
        dtype = NAMES[code]
        size = 4 if dtype == "float32" else 2
        raw = data[pos:pos + count_el * size]
        pos += count_el * size
        if dtype == "float32":
            a = np.frombuffer(raw, dtype="<f4")
        elif dtype == "fp16":
            a = np.frombuffer(raw, dtype="<f2").astype(np.float32)
        else:
            a = bf16_bits_to_float32(np.frombuffer(raw, dtype="<u2"))
        out[name] = (a.reshape(shape), dtype)
    return out


if __name__ == "__main__":
    import argparse

    ap = argparse.ArgumentParser(description="Lists the entries of a .trt file")
    ap.add_argument("path")
    for name, (a, dt) in read_trt(ap.parse_args().path).items():
        print("%s %s %s" % (name, dt, list(a.shape)))
