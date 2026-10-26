<!-- Summary template for model reports and the PR descriptions that carry them. Fill every <...>.
     The TLDR comes first and must let a reader decide in two minutes. Details follow, trimmed hard. -->

# <Model name, quantisation, context>

## TLDR

**Verdict:** <Good model, Not a good model, or Not decided>. <One sentence: the reason, with the number behind it.>

| Check | Result |
|---|---|
| Tier 1 gate | <pass or fail, and runs passed, e.g. 3 of 3> |
| Tier 2 (22 tasks; champion bar 22/22) | <scores by run, e.g. 22, 17, 19> |
| Tier 3 (7 tasks) | <run, with its scores; or not run, and why> |
| Quality | <the number that decides the verdict, with its run count> |
| Time | <Tier 2 minutes per run; tok/s> |
| Memory pressure | <minimum free memory; swap growth> |

**Against the champion:** <one line with the champion's numbers, MiniCPM-V 4.7 IQ2 today>.

## Details

Only what a decision needs. Each point is one or two short bullets, or one table.

- **Setup:** agent, thinking on or off, context, quantisation, file size, machine.
- **Runs:** how many per task, when, and any run that hit a time cap or looped.
- **Caveats:** what was not tested, and what would change the verdict.

## Sources

Link the full report, `summary.md` and the raw run folders. Do not paste them here.
