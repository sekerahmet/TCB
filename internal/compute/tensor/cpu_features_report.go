package tensor

// CPUFeatures işlemcinin ve işletim sisteminin birlikte desteklediği komut kümelerini sabit
// tablonun sırasıyla verir (19.12, TR_machine().TR_cpu_features). Tablo: x86_64'te "sse4_2",
// "avx", "avx2", "fma", "f16c", "avx512f"; arm64'te "neon". Liste donanımı söyler:
// TR_CPU_AVX2=0 çekirdekleri kapatır ama bu listeyi değiştirmez. Tablo dışındaki mimaride
// liste boştur.
func CPUFeatures() []string { return cpuFeatureReport() }
