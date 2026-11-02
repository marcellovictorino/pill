# Qwen3.8 Flash Next on a 24 GB M4: Coder GSQ-RCO IQ1_M (256 of 512 experts), 32k context

## TLDR

**Verdict:** Not a good model for this machine. The only Flash Next file that loads here runs at about 4 tok/s,
takes 632 s on the Tier 1 prompt (limit 60 s) and scored 20/22 on Tier 2 in 33 minutes, against MiniCPM's 22/22 in
about 4 minutes.

| Check | Result |
|---|---|
| Tier 1 gate | Fail, 1 of 1 at the default limit: no answer within the limit (pill cut the request at 180 s). Relaxed limit (900 s): 632 s, answer correct |
| Tier 2 (22 tasks; champion bar 22/22) | 20 (one relaxed run, 33.1 minutes; failed `bestMove blocks` and `bestMove lowest index tie-break`) |
| Tier 3 (7 tasks) | Not run: at about 4 tok/s every task would hit its 30-minute cap |
| Quality | 20/22, single run; no evidence it beats MiniCPM on anything |
| Time | Tier 2 33.1 minutes; decode 4.1 tok/s in the bench, 2.3–2.6 tok/s on code in a probe; prompt processing about 10 tok/s |
| Memory pressure | Minimum free memory 17% (38% in Tier 1); swap growth 0 |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes the default gate with 22/22 in every run (5 runs) at about 25 tok/s.

## Details

- **Why the Coder file:** every full Qwen3.8 Flash Next GGUF is three times this machine's memory or more (smallest
  found: Unsloth UD-IQ1_S, about 74 GB; Bartowski IQ1_S, about 70 GB). ISTA-DASLab's Coder pack keeps 256 of the 512
  experts per layer: 58 GB on disk, of which the 28.8 GB second shard is an n-gram table that `--lazy-mode on` leaves
  on disk. Strata, the engine the model was announced with, was dropped: it is not supported on Mac.
- **Setup:** llama.cpp build 11960 through pill 0.4.1, Pi, thinking off, 32k context, q8_0 KV cache, 24 GB M4. The
  `models.toml` preset that loads in 19 s with no swap:

  ```toml
  [model.preset]
    cpu-moe = true      # expert weights stay in CPU memory, attention and shared layers on the GPU
    no-repack = true    # without it the CPU backend copies the experts and swap grows
    fit = "off"
    load-mode = "mmap"
    lazy-mode = "on"    # keep the n-gram shard on disk
  ```

- **Layouts that failed:** `--fit on` at 64k moved experts to the CPU with repacking: swap grew to 14.6 GB and prompt
  processing fell to 6.3 tok/s, so the run was stopped. All layers on the GPU (`-ngl 99`, mmap, 32k) exceeded Metal's
  recommended working set and free memory fell to 6%, so the run was stopped.
- **MTP (multi-token prediction):** llama.cpp 11960 supports it for this model (`--spec-type draft-mtp` with the
  separate 2.2 GB MTP head). In two 128-token counting probes it raised decoding from 4.8–7.1 to 6.4–8.4 tok/s with
  98% of drafts accepted. On a 300-token coding answer it gave nothing: 2.3–2.6 tok/s with MTP against 2.6 without,
  76–78% accepted. Reading experts from memory pages dominates, so the tiers ran without MTP.
- **Runs:** one default run (Tier 1 failed, Tier 2 skipped) and one relaxed run (`--tier1-max-seconds 900
  --tier2-max-minutes 90`) on 1 November 2026, with no other model loaded. Tier 2 used 5 agent turns.
- **Files:** deleted on 2 November 2026, with the MTP head and the partial Strata install. Do not re-test on this
  machine.
- **Caveats:** single runs. A machine with 64 GB or more could hold a full Flash Next file; nothing here says how
  that would score. Strata on Mac streams experts from the SSD and was not measured.

## Sources

Seen issues table and re-test conditions: [`tried-and-rejected.md`](tried-and-rejected.md). Raw runs (not
committed): `~/.pill/benchmark/20261101T201610Z-qwen38-flash-coder-iq1m-32k` (default) and
`20261101T202329Z-qwen38-flash-coder-iq1m-32k` (relaxed).
