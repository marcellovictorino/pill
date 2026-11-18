# Qwen3.8 27B pi-adapted LoRA on UD-IQ3_XXS (10.9 GB), 32k context, thinking off

## TLDR

**Verdict:** Not adopted, but the most consistent Qwen3.8 27B setup tried here. With harness-hub's Pi adapter, Tier 2
scored 22/22 in all three runs using 24–39 tool calls, against 52–97 for the base model's quants; Tier 1 still takes
138–146 s against the 60 s default, and one Tier 2 run hit the 30-minute cap. MiniCPM stays the default.

| Check | Result |
|---|---|
| Tier 1 gate | Fail at the default 60 s: 146, 138 and 146 s, 3 of 3 runs passed only the relaxed 300 s limit |
| Tier 2 (22 tasks; champion bar 22/22) | 22, 22, 22 (relaxed 30-minute cap) |
| Tier 3 (7 tasks) | Not run: Tier 1 rules the model out of the default gate on this machine |
| Quality | No lost check in 3 runs; 24, 39 and 24 tool calls per Tier 2 run |
| Time | Tier 2 12.6, 30.0 (cap) and 13.5 min; 6.6–7.0 tok/s |
| Memory pressure | Minimum free memory 12–17%; swap up 0, 0.7 and 0.7 GB |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes the default gate on this machine: Tier 1 under 60 s, 22/22 in about
5 minutes, about 28 tok/s.

## Details

- **Model:** [harness-hub/Qwen3.8-27B-pi-adapted](https://huggingface.co/harness-hub/Qwen3.8-27B-pi-adapted), a LoRA
  (r 32, alpha 64, all attention and MLP projections) trained with reinforcement learning to drive the Pi coding agent,
  released 16 November 2026. Its card reports Terminal-Bench 2.1 39.5% to 55.8% and 55–62% fewer tool calls under Pi.
  The F16 GGUF adapter (`Qwen3.8-27B-pi-lora-F16.gguf`, 319 MB, from `harness-hub/Qwen3.8-27B-pi-adapted-GGUF`) is
  loaded at runtime on a GGUF of the base model; there is no merged GGUF.
- **Setup:** Pi 1.5.2, thinking off (as the adapter was trained), 32k context, no MTP, llama.cpp build 12359, pill 0.6.0,
  24 GB M4. Unsloth `Qwen3.8-27B-UD-IQ3_XXS.gguf` as the base, with the adapter in the model's preset:

  ```toml
  [model.preset]
    chat-template-kwargs = '{"enable_thinking":false}'
    lora = "~/.pill/adapters/Qwen3.8-27B-pi-lora-F16.gguf"   # full path in models.toml
  ```

  The same greedy request gave a different answer with the adapter's scale set to 0, so the adapter was active.
- **Why IQ3_XXS:** UD-IQ4_XS (14.3 GB) plus the adapter loaded, but left 10–11% free and pushed swap from 0.7 to 6.2 GB
  while other apps were open, so it was not benchmarked. The card notes that lower-bit bases drift further from the
  trained behaviour; the results above are therefore a lower bound for the adapter.
- **Runs:** 18 November 2026, three cold runs at the relaxed gate (300 s, 30 min, memory floor 7%). Run 2 reached 22/22
  but was still working at the 30-minute cap (39 tool calls). No run repeated a tool call in a loop. The Tier 1 answer
  (informational) was right in run 1, a bare `2` in run 2 and unfinished in run 3.
- **Against the base model:** the base UD-IQ3_XXS without the adapter scored 22, 22 and 20 (two at the cap) on 25 October;
  the base UD-IQ4_XS and Dynamic 3.1 quants never scored 22/22 in every run and used 52–97 tool calls. Tier 1 is the
  same, as expected: the adapter does not change the dense model's prompt processing.
- **Caveats:** three runs, one setup; the base comparisons ran on other days and builds. Tier 3 not run.

## What would justify more work

- **Tier 3 on this setup**, to see whether fewer, better tool calls carry over to the larger tasks.
- **A higher-precision base** on a machine with more memory, closer to the BF16 weights the adapter was trained on.
- **A pill change that relaxes Tier 1** for slow, high-quality models; no Qwen3.8 27B quant reaches 60 s here.

## Sources

Raw runs are in `~/.pill/benchmark/` (`20261118T111247Z-qwen3-8-27b-pi-adapted-iq3xxs-32k`, not committed). Row:
[`tried-and-rejected.md`](tried-and-rejected.md). Base model results: [the Qwen3.8 27B report](qwen38-27b.md).
