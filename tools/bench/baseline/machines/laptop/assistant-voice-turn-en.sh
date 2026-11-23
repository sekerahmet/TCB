# Dost'un sesli sohbeti, İngilizce (S8/en), dizüstü: canlı kip, sahte mikrofonda O-7'nin 10 soruluk sesi (turn_en.wav, sorular
# arasında ve sonda 48 sn sessizlik, BC6), sessiz çalıcı, en çok 10 tur; OLCU satırlarından ilk ses (ortanca, en kötü), cevap,
# yazı, ilk token. Satır: docs/measurements/assistant.md, "Dizüstü i7-13700H". Taraflar: yalnız TR (emsal yok), işlemcide.
# Modeller Dost'un bu dildeki dördü (dilin sesi mms-tts-eng; BC28), %USERPROFILE%\models'tan; ses OLCU_SESLER'den. 1 ısınma + 3 koşu, koşu başına ~500 sn (sesin süresi).
# Ses: dizüstünde run.py --make-sounds ile üretildi; örnekleri Colab'ınkilerle aynı, sha256'yı WAV başlığındaki ffmpeg sürüm
# etiketi ayırır (stt_tr'de örnekler bayt bayt aynı; turn_en A-11'inkine 48 sn sessizlik eklenmişiyle bayt bayt aynı).
# Deneme kipinde yalnız kurulum, modeller, sesin sha256'sı ve makine kaydı: tek koşu 2 dakikaya sığmaz, koşu pencerede.
#   bash tools/bench/baseline/machines/laptop/assistant-voice-turn-en.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
for m in qwen3-4b-q4km whisper-small silero-vad-16k mms-tts-eng; do olcu_model "$m"; done
olcu_ses "turn_en.wav=2ffbe20118cbee7f8ae95d310ce49a1ffef87da0c9511b33cc4c056206d87e4b"
olcu_makine assistant-voice-turn-en
if [ "${OLCU_DENEME:-}" = 1 ]; then echo "deneme: koşu denenmedi (tek koşu ~500 sn, 2 dakikaya sığmaz), pencerede ilk koşu"; exit 0; fi
olcu_kos assistant-voice-turn-en S8 --key S8/en
