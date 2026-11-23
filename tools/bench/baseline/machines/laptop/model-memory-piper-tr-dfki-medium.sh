# Model belleği, Piper Türkçe dfki medium (S4/piper-tr-dfki-medium), dizüstü: Dost'un kataloğunun bu girdisi (bağlamı 0; tools/assistant/katalog.json
# measured), catalog_memory.tr ile modelin yüklenip bir kez çalıştığı andaki yükleme süresi ve bellek tepesi; bağlam derinliği
# yok (TCB 2026-11-20). Satır: docs/measurements/text-to-speech.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok),
# işlemcide. Model rhasspy/piper-voices tr_TR-dfki-medium, %USERPROFILE%\models'tan (yeri ve kimliği sources.json'da). 1 ısınma + 2 koşu (sayı koşturucunun sorusunda, questions.py q_catalog); deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/model-memory-piper-tr-dfki-medium.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model piper-tr-dfki-medium
olcu_makine model-memory-piper-tr-dfki-medium
olcu_kos model-memory-piper-tr-dfki-medium S4 --key S4/piper-tr-dfki-medium/0
