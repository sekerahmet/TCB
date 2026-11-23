# Uzun bağlamda bellek, Gemma 3 4B Q4_K_M (S4/gemma3-4b-q4km), dizüstü: Dost'un kataloğunun bu girdisi, kataloğun ölçülmüş iki
# bağlamında (40 ve 7981 token; tools/assistant/katalog.json measured) catalog_memory.tr ile yükleme süresi ve bellek tepesi.
# Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Model %USERPROFILE%\models'tan (yeri ve kimliği sources.json'da). 1 ısınma + 2 koşu (sayı koşturucunun sorusunda, questions.py q_catalog).
# Deneme kipinde yalnız 40 token'lık kalem bir kez koşar: 7981 token'ı işlemcide okumak dizüstünde 2 dakikaya sığmaz
# (Qwen3-4B'nin 2026-11-20 denemesinde 79 sn'de kesildi). İki kalem aynı komutu --context'le ayırır.
#   bash tools/bench/baseline/machines/laptop/long-context-memory-gemma3-4b-q4km.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model gemma3-4b-q4km
olcu_makine long-context-memory-gemma3-4b-q4km
baglam=""; [ "${OLCU_DENEME:-}" = 1 ] && baglam=40  # denemede yalnız 40 token'lık kalem (lib.sh gibi yalnız OLCU_DENEME=1)
olcu_kos long-context-memory-gemma3-4b-q4km S4 --key "S4/gemma3-4b-q4km/$baglam"
