# Metin üretme, Qwen3-4B Q8_0 (Qwen/Qwen3-4B-GGUF) (S1/qwen3-4b-q8), dizüstü: boş önbellekte 512 token okuma, 512 derinlikten 128 adım üretim;
# TR (language_model.tr, nicemli etkinleştirme Dost'un kuralıyla) ile llama.cpp b12124 (llama-bench -ngl 0), ABBA 10 koşu.
# Uyuşma (B14): llama-completion'ın 32 token'ı TR'de zorlanmış okumayla; ε 2,3770726, koşudan önce burada yazılı (BC17,
# TCB 2026-11-20 02:29). Neden: TR'nin Q8_0 için nicemli yolu yok (15.53a: q4, q4_k, q6_k), öbür modellerin ε yöntemi
# (nicemli ile nicemsiz yolun farkı) burada uygulanamaz; ama llama.cpp işlemcide Q8_0 çarpımında etkinliği 8 bite nicemler,
# ε 0 değildir. Bu yüzden aynı makinenin Qwen3-4B Q4_K_M ε'u: dizüstünde iki kez 2,3770726 ölçüldü (S1 denemesi
# 2026-11-19 23:36, S3 denemesi 2026-11-20 01:01; "measured here, quantized and unquantized path"). HIGHRAM'in
# S1/qwen3-4b-q8 satırı "ölçülmedi: BC17" kalır; 2026-11-20'deki deneme (ε 0'la 2. konum 0,0121 ayrı) bu ε'la yargılanmaz.
# Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: ikisi işlemcide (llama.cpp'nin Windows paketi
# işlemci paketi, komutta -ngl 0). llama.cpp'nin yeri OLCU_LLAMA (öntanımlı $USERPROFILE/tools/llama.cpp/b12124).
#   bash tools/bench/baseline/machines/laptop/generate-qwen3-4b-q8.sh
. "$(dirname "$0")/lib.sh"
LLAMA=${OLCU_LLAMA:-$USERPROFILE/tools/llama.cpp/b12124}
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model qwen3-4b-q8
olcu_makine generate-qwen3-4b-q8
"$LLAMA/llama-completion.exe" --version 2>&1 | tr -d '\r' | tee -a "$KAYIT/surumler.txt"
olcu_kos generate-qwen3-4b-q8 S1 --key S1/qwen3-4b-q8 --epsilon S1/qwen3-4b-q8=2.3770726
