# Dost'un yazılı sohbeti, İngilizce (S9/en), dizüstü: ekranlı kipte send_text ile S8'in 10 sorusu; ilk token süresi ve token/sn.
# Satır: docs/measurements/assistant.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Modeller Dost'un bu dildeki dördü (questions.py dost_args: qwen3-4b-q4km, whisper-small, silero-vad-16k, dilin sesi
# mms-tts-eng; BC28: öbür dilin sesi bağlanmaz), %USERPROFILE%\models'tan. Koşturucu Dost'u boş bir kapıda ve sessiz çalıcıyla açar (questions.py
# screen_cmd, BC23); Ahmet'in açık bir Dost'u işlemciyi paylaşır, ölçüden önce kapanmalı. 1 ısınma + 3 koşu; deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/assistant-text-turn-en.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
for m in qwen3-4b-q4km whisper-small silero-vad-16k mms-tts-eng; do olcu_model "$m"; done
olcu_makine assistant-text-turn-en
olcu_kos assistant-text-turn-en S9 --key S9/en
