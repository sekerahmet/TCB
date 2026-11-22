# Dost'un sayfasının açılışı (S10), dizüstü: ekranlı kipte açılıştan state idle'a kadar geçen süre; dizüstünde yalnız
# sıcak (O-3). Satır: docs/measurements/assistant.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Modeller Dost'un beşi (questions.py MODELS: qwen3-4b-q4km, whisper-small, silero-vad-16k, piper-tr-dfki-medium,
# mms-tts-eng), %USERPROFILE%\models'tan. Koşturucu Dost'u boş bir kapıda ve sessiz çalıcıyla açar (questions.py
# screen_cmd, BC23); Ahmet'in açık bir Dost'u işlemciyi paylaşır, ölçüden önce kapanmalı. 3 koşu; deneme kipinde bir kez.
#   bash tools/bench/baseline/machines/laptop/assistant-page-ready.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
for m in qwen3-4b-q4km whisper-small silero-vad-16k piper-tr-dfki-medium mms-tts-eng; do olcu_model "$m"; done
olcu_makine assistant-page-ready
olcu_kos assistant-page-ready S10
