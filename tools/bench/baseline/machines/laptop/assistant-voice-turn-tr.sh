# Dost'un sesli sohbeti, Türkçe (S8/tr), dizüstü: canlı kip, sahte mikrofonda O-7'nin 10 soruluk sesi (turn_tr.wav, sorular
# arasında ve sonda 48 sn sessizlik, BC6), sessiz çalıcı, en çok 10 tur; OLCU satırlarından ilk ses (ortanca, en kötü), cevap,
# yazı, ilk token. Satır: docs/measurements/assistant.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Modeller Dost'un beşi, %USERPROFILE%\models'tan; ses OLCU_SESLER'den. 1 ısınma + 3 koşu, koşu başına ~500 sn (sesin süresi).
# Ses: dizüstünde run.py --make-sounds ile üretildi; örnekleri Colab'ınkilerle aynı, sha256'yı WAV başlığındaki ffmpeg sürüm
# etiketi ayırır (stt_tr'de örnekler bayt bayt aynı; turn_en A-11'inkine 48 sn sessizlik eklenmişiyle bayt bayt aynı).
# Deneme kipinde yalnız kurulum, modeller, sesin sha256'sı ve makine kaydı: tek koşu 2 dakikaya sığmaz, koşu pencerede.
#   bash tools/bench/baseline/machines/laptop/assistant-voice-turn-tr.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
for m in qwen3-4b-q4km whisper-small silero-vad-16k piper-tr-dfki-medium mms-tts-eng; do olcu_model "$m"; done
olcu_ses "turn_tr.wav=88b23385871f86adcf77db240aa0fb510b56189b6823da86023be088a97c8f34"
olcu_makine assistant-voice-turn-tr
if [ "${OLCU_DENEME:-}" = 1 ]; then echo "deneme: koşu denenmedi (tek koşu ~500 sn, 2 dakikaya sığmaz), pencerede ilk koşu"; exit 0; fi
olcu_kos assistant-voice-turn-tr S8 --key S8/tr
