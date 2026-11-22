# Konuşmayı yazıya çevirme, Whisper small İngilizce (S5/en/whisper.cpp), dizüstü: O-7'nin sabit sesi (stt_en.wav, 16 kHz);
# TR (whisper.tr) ile whisper.cpp f16 (whisper-cli -ng, işlemcide), ABBA 10 koşu. Uyuşma B15: TR'nin float32 başvurulara
# (faster-whisper float32, transformers) sözcük uzaklığı onların birbirine uzaklığını aşmaz; whisper.cpp sınır kurmaz,
# uzaklığı kayda yazılır; transformers yalnız uyuşmada koşar, hız emsali değil (TCB 2026-11-20).
# Satır: docs/measurements/speech-to-text.md, "Dizüstü i7-13700H". Taraflar: hepsi işlemcide.
# whisper.cpp: ggml-org/whisper.cpp'nin b5454 Windows x64 işlemci derlemesi (v1.9.5 ile aynı commit d1be6fde; Colab v1.9.5'i
# kaynaktan derler), yeri OLCU_WHISPER; ikilinin sha256'sı ve sürümü sources.json'da. Modeller ggerganov/whisper.cpp ggml-small, openai/whisper-small ve
# Systran/faster-whisper-small, %USERPROFILE%\models'tan; ses OLCU_SESLER'den.
# Deneme kipinde uyuşma geçişi; 2 dakikaya sığmayabilir (kurulum 25–50 sn, uyuşma dört program; 2026-11-20'de sığmadı,
# 2026-11-22'de İngilizcesi 76 sn'de uyuştu): sığmazsa deneme kurulum ve iki tarafın (TR, whisper.cpp) çıkış 0'ıyla geçer;
# uyuşma pencerede ilk koşu, uyuşmazsa durur, sıradaki kaleme geçilir (TCB 2026-11-20).
#   bash tools/bench/baseline/machines/laptop/speech-to-text-whisper-small-en-whisper-cpp.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "whisper_cli=$WHISPER/whisper-cli.exe"
olcu_model ggml-small
olcu_model whisper-small
olcu_model faster-whisper-small
olcu_ses "stt_en.wav=ef0b864750557c7d7d47e5e78416239401efad4cecc1597af93c4f6d75fa44d8"
olcu_makine speech-to-text-whisper-small-en-whisper-cpp
olcu_arac whisper.cpp
olcu_surum faster-whisper ctranslate2 torch transformers
olcu_kos speech-to-text-whisper-small-en-whisper-cpp S5 --key S5/en/whisper.cpp
