# Model belleği, Silero VAD 16 kHz (S4/silero-vad-16k), dizüstü: Dost'un kataloğunun bu girdisi (bağlamı 0; tools/assistant/katalog.json
# measured), catalog_memory.tr ile modelin yüklenip bir kez çalıştığı andaki yükleme süresi ve bellek tepesi; bağlam derinliği
# yok (TCB 2026-11-20). Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok),
# işlemcide. Model snakers4/silero-vad, %USERPROFILE%\models'tan (yeri ve kimliği sources.json'da). 1 ısınma + 2 koşu (sayı koşturucunun sorusunda, questions.py q_catalog); deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/model-memory-silero-vad-16k.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model silero-vad-16k
olcu_makine model-memory-silero-vad-16k
olcu_kos model-memory-silero-vad-16k S4 --key S4/silero-vad-16k/0
