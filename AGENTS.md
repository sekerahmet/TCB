# TR for coding agents

TR is a statically typed language for AI work: preparing data, training models, running published models. Every word of the language is an identity that starts with `TR_`, such as `TR_print` or `TR_for`. Programs are written with identities only, in any human language's comments and names.

The rest of this repository is mostly in Turkish. The code is not: `TR_` identities mean the same everywhere.

This page is also at https://raw.githubusercontent.com/sekerahmet/TCB/main/AGENTS.md

## If you cannot clone or run anything

If you can only read web pages, read the raw links under "Where to look" and write the program from them. Say plainly that you did not run it: no output is real until `trlang` has printed it.

## Install

You need Go 1.25 or newer. No cgo, no CUDA toolkit.

```bash
git clone https://github.com/sekerahmet/TCB
cd TCB
go build -o trlang ./cmd/trlang
```

On Windows, build `trlang.exe`.

The command is `trlang`. Never type `tr`: on Linux that is another program and it does nothing useful, silently.

## Commands

```bash
TR_LANG=en ./trlang check --json program.tr   # check without running; one JSON record per message
TR_LANG=en ./trlang run program.tr            # check and run
```

`TR_LANG=en` gives messages in English. Exit code 0 means no errors. Messages say what is wrong and usually give the correct spelling: read them and follow them. A clean `check` does not mean every rule held: the tool does not check all of them yet, and the reference below is the rule.

## Writing a program

- The first line of a file is exactly `# TR 4`.
- There is no `main`: the file runs from top to bottom.
- Put functions and types before the line that runs them.
- Use `TR_var x = 1`, `TR_func f(a: TR_Int) -> TR_Int { TR_return a }`, `TR_if c { } TR_else { }`, `TR_for x TR_in l { }`.
- Named arguments use `:`, as in `f(k: 3)`. Strings use double quotes, and `"\(x)"` inserts a value.
- Your own names must not start with `TR_`.

## Train, save, load, generate

This program trains a small character model, saves it, loads it into a new model and generates text. It runs on a CPU in about a second.

```tr
# TR 4
TR_import "tr/layers"

TR_const TEXT = "the cat sat on the mat. the dog sat on the log. the cat saw the dog. " * 8
TR_const CONTEXT = 16

# Character vocabulary: each new character gets the next number.
TR_var chars: TR_List[TR_String] = []
TR_var ids: TR_Dict[TR_String, TR_Int] = {}
TR_var data: TR_List[TR_Int] = []
TR_for c TR_in TEXT {
    TR_if TR_not ids.TR_contains(c) {
        ids[c] = chars.TR_length
        chars.TR_append(c)
    }
    data.TR_append(ids[c])
}

# Inputs are token lists; targets are a tensor of the next tokens.
TR_var inputs: TR_List[TR_List[TR_Int]] = []
TR_var targets: TR_List[TR_List[TR_Int]] = []
TR_for b TR_in 0..8 {
    inputs.TR_append(data[b * 20..b * 20 + CONTEXT])
    targets.TR_append(data[b * 20 + 1..b * 20 + CONTEXT + 1])
}

TR_var model = TR_GPT(chars.TR_length, TR_context: CONTEXT, TR_embed_dim: 32, TR_num_layers: 2, TR_num_heads: 4, TR_seed: 1)
TR_var optimizer = TR_AdamW(TR_parameters(model), TR_lr: 0.01, TR_weight_decay: 0.0)
TR_for step TR_in 0..60 {
    optimizer.TR_zero_grad()
    TR_var loss = model.TR_loss(inputs, TR_Tensor(targets))
    TR_backward(loss)
    optimizer.TR_step()
    TR_if step % 20 == 0 {
        TR_print(step, TR_round(loss.TR_tolist, 3))
    }
}

# Save by name, then load into a model built with another seed.
TR_save_tensors("model.safetensors", TR_parameters(model, TR_named: TR_true))
TR_var restored = TR_GPT(chars.TR_length, TR_context: CONTEXT, TR_embed_dim: 32, TR_num_layers: 2, TR_num_heads: 4, TR_seed: 2)
TR_print("loaded", TR_load_parameters(restored, TR_load_tensors("model.safetensors")))

TR_var generated = restored.TR_generate(data[0..8], 30, TR_temperature: 0.0, TR_seed: 1)
TR_print(generated.TR_map((i) => chars[i]).TR_join(""))
```

## Where to look

Comments in the examples are in Turkish. The code in them runs as it is from the repository root. The links are raw files, so they work without a clone; in a clone, the same path is on disk.

| You want | Read |
| :-- | :-- |
| The language in one page | [`docs/agent_reference.md`](https://raw.githubusercontent.com/sekerahmet/TCB/main/docs/agent_reference.md) |
| The full rules | [`docs/spec/README.md`](https://raw.githubusercontent.com/sekerahmet/TCB/main/docs/spec/README.md) |
| The library: layers, optimizers, training, audio | [`lib/README.md`](https://raw.githubusercontent.com/sekerahmet/TCB/main/lib/README.md) |
| A guide from the first step | [`docs/guide/README.md`](https://raw.githubusercontent.com/sekerahmet/TCB/main/docs/guide/README.md) |
| A model written by hand | [`examples/tr/36_karakter_modeli.tr`](https://raw.githubusercontent.com/sekerahmet/TCB/main/examples/tr/36_karakter_modeli.tr) |
| A model from the library, with checkpoints | [`examples/tr/41_gpt.tr`](https://raw.githubusercontent.com/sekerahmet/TCB/main/examples/tr/41_gpt.tr) |
| Your own data and a tokenizer | [`examples/tr/39_veri_ile_egitim.tr`](https://raw.githubusercontent.com/sekerahmet/TCB/main/examples/tr/39_veri_ile_egitim.tr) |
| Stopping and resuming a run | [`examples/tr/40_kontrol_noktasi.tr`](https://raw.githubusercontent.com/sekerahmet/TCB/main/examples/tr/40_kontrol_noktasi.tr) |
| Running a published model | [`examples/tr/46_hazir_model_ve_uretim.tr`](https://raw.githubusercontent.com/sekerahmet/TCB/main/examples/tr/46_hazir_model_ve_uretim.tr) |

All examples: https://github.com/sekerahmet/TCB/tree/main/examples/tr. Search them for a name before you guess it: there is one spelling for each thing.
