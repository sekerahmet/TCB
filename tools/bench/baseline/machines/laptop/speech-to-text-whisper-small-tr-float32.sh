# Konuşmayı yazıya çevirme, Whisper small Türkçe (S5/tr/faster-whisper-float32), dizüstü: O-7'nin sabit sesi
# (stt_tr.wav, 16 kHz), açgözlü, tek ışın; TR (whisper.tr) ile faster-whisper (ctranslate2) float32,
# ABBA 10 koşu. Uyuşma B15: TR'nin float32 başvurulara (faster-whisper float32, transformers) sözcük uzaklığı onların
# birbirine uzaklığını aşmaz; transformers yalnız uyuşmada koşar, hız emsali değil (TCB 2026-11-20). whisper.cpp f16 kalemi
# bu betikte yok (modeli dizüstünde yok; tools.json'daki whisper_cli yalnız kalemlerin kurulması için).
# Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: hepsi işlemcide.
# Modeller openai/whisper-small ve Systran/faster-whisper-small, %USERPROFILE%\models'tan; ses OLCU_SESLER'den.
# Deneme kipinde yalnız uyuşma geçişi (her taraf bir koşu).
#   bash tools/bench/baseline/machines/laptop/speech-to-text-whisper-small-tr-float32.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "whisper_cli=${OLCU_WHISPER_CLI:-$USERPROFILE/tools/whisper.cpp/whisper-cli.exe}"
olcu_model whisper-small
olcu_model faster-whisper-small
olcu_ses "stt_tr.wav=61eecd21db79818e022e32145301a92ebf4e82e3b4ce3b0c8c97035c6cdd176e"
olcu_makine speech-to-text-whisper-small-tr-float32
olcu_surum faster_whisper ctranslate2 torch transformers
olcu_kos speech-to-text-whisper-small-tr-float32 S5 --key S5/tr/faster-whisper-float32
