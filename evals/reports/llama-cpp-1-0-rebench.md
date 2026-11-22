# llama.cpp 1.0.0 (build 12359) against the build 12110 records, 24 GB M4

## TLDR

**Verdict:** Nothing improves on 1.0.0 for these models. The default MiniCPM and both MoE models fail Tier 1 on the GPU path, where the build 12110 records passed or failed on memory alone. The MoE expert cache does not change the memory gate: CPU-expert runs pass the 10% floor with or without it, and both miss the 60 s Tier 1 limit.

| Check | MiniCPM-V 4.7 IQ2 (default), 64k, GPU | Gemma 4 26B-A4B UD-Q4_K_S, 32k, GPU | Qwen3.6 35B-A3B UD-Q3_K_S, 32k, GPU |
|---|---|---|---|
| Tier 1 gate, 1.0.0 (3 runs, 60 s) | fail, 0 of 3 | fail, 0 of 3 (HTTP 500 compute error) | fail, 0 of 3 (HTTP 500 compute error) |
| Tier 1 gate, 12110 record | pass, 6 of 6 (control, 10 Nov) | fail, 0 of 3 (memory only, 6 to 7% free) | pass, 3 of 3 (20 to 21 s) |
| Tier 2 on 1.0.0 | not run (Tier 1 failed) | not run | not run |
| Tier 2 on 12110 record | 22, 17, 22 and 17, 22, 22 | not run | 22, 22, 20 (cap) |
| Memory, 1.0.0 | lowest free 6%; Metal out of memory in the smoke run | lowest free 12%; swap growth up to 42 MB | lowest free 12%; no swap growth |

| Check | Gemma 4 UD-Q4_K_S, CPU experts, no cache | Gemma 4 UD-Q4_K_S, CPU experts, 4 GB cache |
|---|---|---|
| Tier 1, 1.0.0 (3 runs, 300 s) | 206 s pass, 281 s pass, 313 s fail | 233 s pass, 261 s pass, 307 s fail |
| Tier 2, 1.0.0 | 22/22, 22/22 | 22/22, 18/22 |
| Memory, 1.0.0 | lowest free 24%; swap growth 3038 MB | lowest free 18%; swap growth 3111 MB |
| Decode | about 4 tok/s | about 4 tok/s |

**Against the champion:** MiniCPM-V 4.7 IQ2 passed Tier 1 in all six 12110 control runs (Tier 2 22, 17 and 22 in two batches of three), but passed none of three runs on 1.0.0 at the same 64k catalog setting. Whether the build or the machine's state caused this is not yet separated.

## Details

- **Setup:** pill 0.6.0 built from b0414d9; llama.cpp 1.0.0 build 12359 (commit b1b770d90) from Homebrew, linked since 16 Nov; Pi 1.5.2, thinking off; macOS 27.2.1; 24 GB M4. Three runs per arm, through `pill bench run`.
- **Gate:** the default-GPU arms ran at the 12110 Tier 1 limit of 60 s, 10% free memory and the 15-minute Tier 2 cap. The CPU-expert arms ran at a 300 s Tier 1 limit, which is this pill build's default and the one that let them pass Tier 1 at all.
- **Failures on GPU:** most GPU-resident runs stopped with HTTP 500 "Compute error", and the llama-server log shows Metal "Insufficient Memory" (kIOGPUCommandBufferCallbackErrorOutOfMemory). The 12110 Gemma record failed on free memory without any compute error. The cause in 1.0.0 is not yet known; a new default context, batch size or flash-attention setting is one candidate.
- **Expert cache:** the cache only applies to expert weights kept on the CPU (`--cpu-moe`, `--n-cpu-moe`). It is set here through `LLAMA_ARG_MOE_CACHE_MIB`. On this sample it lowered the lowest free memory by about 6 points and raised swap growth by about 70 MB, neither a clear effect over three runs.
- **Memory gate:** the 12110 Gemma record failed the 10% memory floor on the GPU path, at 6 to 7% free. The CPU-expert arms passed the floor at 18 to 24% free, so keeping experts on the CPU is what changes the memory result. The price is a Tier 1 time of 200 to 310 s, well past the 60 s limit.
- **Caveats:** the 12110 controls were run at other times and on a different machine state, when swap was 5.6 to 6.3 GB in use; this run started with about 2.9 to 3 GB. Pi moved from 1.5.0 to 1.5.2. The MiniCPM control's context is not recorded, so the 64k comparison is assumed. The GLM family named in PILL-107 was not run. Nothing was tested with a changed llama.cpp flag beyond the expert-cache variable.

## Sources

Raw runs are in the local `~/.pill/benchmark` folder, dated 21 and 22 Nov 2026. The 12110 records are in `bigger-models-overnight.md`, `tried-and-rejected.md` and the MiniCPM catalog entry.
