# Dost'un yazılı sohbeti, Türkçe (S9/tr), dizüstü: ekranlı kipte send_text ile S8'in 10 sorusu; ilk token süresi ve token/sn.
# Satır: docs/measurements/assistant.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Modeller Dost'un beşi (questions.py MODELS: qwen3-4b-q4km, whisper-small, silero-vad-16k, piper-tr-dfki-medium,
# mms-tts-eng), %USERPROFILE%\models'tan. Koşturucu Dost'u boş bir kapıda ve sessiz çalıcıyla açar (questions.py
# screen_cmd, BC23); Ahmet'in açık bir Dost'u işlemciyi paylaşır, ölçüden önce kapanmalı. 1 ısınma + 3 koşu; deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/assistant-text-turn-tr.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
for m in qwen3-4b-q4km whisper-small silero-vad-16k piper-tr-dfki-medium mms-tts-eng; do olcu_model "$m"; done
olcu_makine assistant-text-turn-tr
olcu_kos assistant-text-turn-tr S9 --key S9/tr
