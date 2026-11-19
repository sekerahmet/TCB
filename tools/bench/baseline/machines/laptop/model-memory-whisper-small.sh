# Model belleği, Whisper small (S4/whisper-small), dizüstü: Dost'un kataloğunun bu girdisi (bağlamı 0; tools/assistant/katalog.json
# measured), catalog_memory.tr ile modelin yüklenip bir kez çalıştığı andaki yükleme süresi ve bellek tepesi; bağlam derinliği
# yok (TCB 2026-11-20). Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok),
# işlemcide. Model openai/whisper-small, %USERPROFILE%\models'tan (sha256 Dost'un kataloğundan). 1 ısınma + 5 koşu; deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/model-memory-whisper-small.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model whisper-small
olcu_makine model-memory-whisper-small
olcu_kos model-memory-whisper-small S4 --key S4/whisper-small/0
