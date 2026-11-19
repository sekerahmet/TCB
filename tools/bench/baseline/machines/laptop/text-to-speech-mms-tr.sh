# Yazıyı sese çevirme, MMS Türkçe (S6/tr), dizüstü: texts.json'un üç cümlesi, gürültü 0, cümle başına tepe normlama; TR
# (mms.tr) ile transformers VitsModel, ABBA 10 koşu, uyuşma aynı dalga boyu. Satır: docs/measurements/text-to-speech.md,
# "Dizüstü i7-13700H". Taraflar: ikisi işlemcide; model facebook/mms-tts-tur (sha256 Dost'un kataloğundan).
# Not: emsal süreç içi ısınmasız ilk çağrılarla ölçülür (Colab'da şüpheli işaretli, TCB 22:45); satır bunu yazar.
#   bash tools/bench/baseline/machines/laptop/text-to-speech-mms-tr.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model mms-tts-tur
olcu_makine text-to-speech-mms-tr
olcu_surum torch transformers
olcu_kos text-to-speech-mms-tr S6 --key S6/tr
