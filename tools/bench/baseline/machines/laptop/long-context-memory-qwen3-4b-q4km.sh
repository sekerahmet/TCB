# Uzun bağlamda bellek, Qwen3-4B Q4_K_M (S4/qwen3-4b-q4km), dizüstü: Dost'un kataloğunun bu girdisi, kataloğun ölçülmüş iki
# bağlamında (45 ve 7981 token; tools/assistant/katalog.json measured) catalog_memory.tr ile yükleme süresi ve bellek tepesi.
# Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Model %USERPROFILE%\models'tan (sha256 Dost'un kataloğundan). 1 ısınma + 5 koşu.
# Deneme kipinde yalnız 45 token'lık kalem bir kez koşar: 7981 token'ı işlemcide okumak dizüstünde 2 dakikaya sığmaz
# (2026-11-20 denemesinde 79 sn'de kesildi; Dost'un okuma hızıyla ~5 dk). İki kalem aynı komutu --context'le ayırır.
#   bash tools/bench/baseline/machines/laptop/long-context-memory-qwen3-4b-q4km.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model qwen3-4b-q4km
olcu_makine long-context-memory-qwen3-4b-q4km
if [ "${OLCU_DENEME:-}" = 1 ]; then anahtar=S4/qwen3-4b-q4km/45; else anahtar=S4/qwen3-4b-q4km/; fi
olcu_kos long-context-memory-qwen3-4b-q4km S4 --key "$anahtar"
