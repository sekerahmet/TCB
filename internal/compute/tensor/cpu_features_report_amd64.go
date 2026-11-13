package tensor

// cpuFeatureReport CPUID ve XGETBV'yi her çağrıda yeniden okur (Intel SDM, cilt 2A, CPUID):
//   - sse4_2: yaprak 1, ECX bit 20;
//   - avx: yaprak 1, ECX bit 28 ve OSXSAVE (bit 27), XCR0 bit 1 ve 2 (xmm, ymm);
//   - avx2: yaprak 7, EBX bit 5; fma: yaprak 1, ECX bit 12; f16c: yaprak 1, ECX bit 29;
//     üçü de VEX komutudur, ymm durumunu işletim sistemi kaydetmiyorsa (avx yoksa) yoktur;
//   - avx512f: yaprak 7, EBX bit 16 ve XCR0 bit 5–7 (opmask, zmm'nin üst yarıları).
//
// XGETBV yalnız OSXSAVE varken çağrılır; yoksa komut tanımsızdır.
func cpuFeatureReport() []string {
	out := []string{}
	maxLeaf, _, _, _ := cpuid(0, 0)
	if maxLeaf < 1 {
		return out
	}
	_, _, ecx1, _ := cpuid(1, 0)
	var ebx7 uint32
	if maxLeaf >= 7 {
		_, ebx7, _, _ = cpuid(7, 0)
	}
	var xcr0 uint32
	if ecx1&(1<<27) != 0 {
		xcr0 = xgetbv0()
	}
	avx := ecx1&(1<<28) != 0 && xcr0&0x6 == 0x6
	if ecx1&(1<<20) != 0 {
		out = append(out, "sse4_2")
	}
	if avx {
		out = append(out, "avx")
		if ebx7&(1<<5) != 0 {
			out = append(out, "avx2")
		}
		if ecx1&(1<<12) != 0 {
			out = append(out, "fma")
		}
		if ecx1&(1<<29) != 0 {
			out = append(out, "f16c")
		}
		if ebx7&(1<<16) != 0 && xcr0&0xE0 == 0xE0 {
			out = append(out, "avx512f")
		}
	}
	return out
}
