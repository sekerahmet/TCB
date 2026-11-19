# Konuşmayı yazıya çevirme, Whisper small İngilizce (S5/en/faster-whisper-int8), dizüstü: O-7'nin sabit sesi
# (stt_en.wav, 16 kHz), açgözlü, tek ışın; TR (whisper.tr) ile faster-whisper (ctranslate2) int8,
# ABBA 10 koşu. Uyuşma B15: TR'nin float32 başvurulara (faster-whisper float32, transformers) sözcük uzaklığı onların
# birbirine uzaklığını aşmaz; transformers yalnız uyuşmada koşar, hız emsali değil (TCB 2026-11-20). whisper.cpp f16 kalemi
# bu betikte yok (modeli dizüstünde yok; tools.json'daki whisper_cli yalnız kalemlerin kurulması için).
# Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: hepsi işlemcide.
# Modeller openai/whisper-small ve Systran/faster-whisper-small, %USERPROFILE%\models'tan; ses OLCU_SESLER'den.
# Deneme kipinde yalnız uyuşma geçişi (her taraf bir koşu).
#   bash tools/bench/baseline/machines/laptop/speech-to-text-whisper-small-en-int8.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "whisper_cli=${OLCU_WHISPER_CLI:-$USERPROFILE/tools/whisper.cpp/whisper-cli.exe}"
olcu_model whisper-small
olcu_model faster-whisper-small
olcu_ses "stt_en.wav=ef0b864750557c7d7d47e5e78416239401efad4cecc1597af93c4f6d75fa44d8"
olcu_makine speech-to-text-whisper-small-en-int8
olcu_surum faster_whisper ctranslate2 torch transformers
olcu_kos speech-to-text-whisper-small-en-int8 S5 --key S5/en/faster-whisper-int8
