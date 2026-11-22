# Konuşmayı yazıya çevirme, Whisper small Türkçe (S5/tr/whisper.cpp), dizüstü: O-7'nin sabit sesi (stt_tr.wav, 16 kHz);
# TR (whisper.tr) ile whisper.cpp f16 (whisper-cli -ng, işlemcide), ABBA 10 koşu. Uyuşma B15: TR'nin float32 başvurulara
# (faster-whisper float32, transformers) sözcük uzaklığı onların birbirine uzaklığını aşmaz; whisper.cpp sınır kurmaz,
# uzaklığı kayda yazılır; transformers yalnız uyuşmada koşar, hız emsali değil (TCB 2026-11-20).
# Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: hepsi işlemcide.
# whisper.cpp: ggml-org/whisper.cpp'nin b5454 Windows x64 işlemci derlemesi (v1.9.5 ile aynı commit d1be6fde; Colab v1.9.5'i
# kaynaktan derler), yeri OLCU_WHISPER_CLI. Modeller ggerganov/whisper.cpp ggml-small, openai/whisper-small ve
# Systran/faster-whisper-small, %USERPROFILE%\models'tan; ses OLCU_SESLER'den.
# Deneme kipinde uyuşma geçişi başlar ama 2 dakikaya sığmaz (kurulum ~50 sn, uyuşma dört program): deneme kurulum ve iki
# tarafın (TR, whisper.cpp) çıkış 0'ıyla geçer; uyuşma pencerede ilk koşu, uyuşmazsa durur, sıradaki kaleme geçilir (TCB 2026-11-20).
#   bash tools/bench/baseline/machines/laptop/speech-to-text-whisper-small-tr-whisper-cpp.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "whisper_cli=${OLCU_WHISPER_CLI:-$USERPROFILE/tools/whisper.cpp/b5454/Release/whisper-cli.exe}"
olcu_model ggml-small
olcu_model whisper-small
olcu_model faster-whisper-small
olcu_ses "stt_tr.wav=61eecd21db79818e022e32145301a92ebf4e82e3b4ce3b0c8c97035c6cdd176e"
olcu_makine speech-to-text-whisper-small-tr-whisper-cpp
olcu_surum faster_whisper ctranslate2 torch transformers
olcu_kos speech-to-text-whisper-small-tr-whisper-cpp S5 --key S5/tr/whisper.cpp
