# Yazıyı sese çevirme, MMS Türkçe (S6/tr), dizüstü: texts.json'un üç cümlesi, gürültü 0, cümle başına tepe normlama;
# TR (mms.tr), 1 ısınma + 5 koşu. Emsal yok (en hızlısı aranıyor: sherpa-onnx'in MMS ONNX modelleri; TCB 2026-11-20:
# transformers hız emsali değil); transformers VitsModel yalnız uyuşmada (aynı dalga boyu, iki rastgele tensör).
# Satır: docs/measurements/text-to-speech.md, "Dizüstü i7-13700H". İşlemcide; model facebook/mms-tts-tur (yeri ve kimliği
# sources.json'da). Deneme kipinde uyuşma ve TR bir kez.
#   bash tools/bench/baseline/machines/laptop/text-to-speech-mms-tr.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model mms-tts-tur
olcu_makine text-to-speech-mms-tr
olcu_surum torch transformers
olcu_kos text-to-speech-mms-tr S6 --key S6/tr
