# Model belleği, MMS İngilizce (S4/mms-tts-eng), dizüstü: Dost'un kataloğunun bu girdisi (bağlamı 0; tools/assistant/katalog.json
# measured), catalog_memory.tr ile modelin yüklenip bir kez çalıştığı andaki yükleme süresi ve bellek tepesi; bağlam derinliği
# yok (TCB 2026-11-20). Satır: docs/measurements/text-to-speech.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok),
# işlemcide. Model facebook/mms-tts-eng, %USERPROFILE%\models'tan (yeri ve kimliği sources.json'da). 1 ısınma + 5 koşu; deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/model-memory-mms-tts-eng.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model mms-tts-eng
olcu_makine model-memory-mms-tts-eng
olcu_kos model-memory-mms-tts-eng S4 --key S4/mms-tts-eng/0
