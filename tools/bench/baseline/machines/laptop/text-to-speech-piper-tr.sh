# Yazıyı sese çevirme, Piper Türkçe (S7/tr), dizüstü: texts.json'un üç cümlesi, TR'nin çeviricisinin kimlikleriyle
# (piper.tr --write_ids), gürültü 0; TR (piper.tr) ile onnxruntime, ABBA 10 koşu. Satır: docs/measurements/text-to-speech.md,
# "Dizüstü i7-13700H". Taraflar: ikisi işlemcide (onnxruntime CPUExecutionProvider; sağlayıcı emsalin çıktısında).
# Model rhasspy/piper-voices tr_TR-dfki-medium (sha256 Dost'un kataloğundan), %USERPROFILE%\models\piper-tr-dfki-medium\.
# Not: emsal süreç içi ısınmasız ilk çağrılarla ölçülür (Colab'da şüpheli işaretli, TCB 22:45); satır bunu yazar.
#   bash tools/bench/baseline/machines/laptop/text-to-speech-piper-tr.sh
. "$(dirname "$0")/lib.sh"
olcu_kur
olcu_model piper-tr-dfki-medium
olcu_makine text-to-speech-piper-tr
olcu_surum onnxruntime numpy
olcu_kos text-to-speech-piper-tr S7 --key S7/tr
