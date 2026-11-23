# Model belleği, MMS Türkçe (S4/mms-tts-tur), dizüstü: Dost'un kataloğunun bu girdisi (bağlamı 0; tools/assistant/katalog.json
# measured), catalog_memory.tr ile modelin yüklenip bir kez çalıştığı andaki yükleme süresi ve bellek tepesi; bağlam derinliği
# yok (TCB 2026-11-20). Satır: docs/measurements/text-to-speech.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok),
# işlemcide. Model facebook/mms-tts-tur, %USERPROFILE%\models'tan (yeri ve kimliği sources.json'da). 1 ısınma + 2 koşu (sayı koşturucunun sorusunda, questions.py q_catalog); deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/model-memory-mms-tts-tur.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model mms-tts-tur
olcu_makine model-memory-mms-tts-tur
olcu_kos model-memory-mms-tts-tur S4 --key S4/mms-tts-tur/0
