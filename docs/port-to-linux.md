# Port pill to Linux

Status: not started. This page is the starting point for the work, written so you can pick it up on a Linux machine. Line numbers refer to commit `23e5003`; re-check them with `grep` before editing.

## Is it feasible?

Yes. `GOOS=linux GOARCH=arm64 go build ./...` and `GOOS=linux GOARCH=amd64 go vet ./...` (which also compiles the tests) pass today. The gap is runtime behaviour: macOS tools called by the code, and benchmark thresholds designed for Apple's unified memory. Estimated effort is 2 to 3 days for everything, and under a day for the first slice. The numbers are estimates; nothing here has been run on Linux.

With the current stub in `internal/sysinfo/sysinfo_other.go`, which returns an error for every call:

- `pill catalog` exits 1 (`cmd/catalog.go:23-26`).
- `pill bench run` cannot pass: the sampler takes no samples and the memory check fails (`internal/bench/bench.go:181-186`).
- `pill service install` writes a launchd plist, then the `launchctl` call fails. The plist stays, so `Installed()` is true and every later `pill serve` takes the service path and fails. `pill service uninstall` recovers it. Read from the code, not executed.
- `serve`, `stop`, `pull`, `add`, `ls`, `default`, `rm`, `sync` (without `--bench`), `pi`, `skill` and `doctor` make no `sysinfo` calls. They should work if `ps` and `lsof` exist. Unverified.

## What needs changing

Risk is the risk of the port itself.

| Area | Where | On macOS | On Linux | Risk |
|---|---|---|---|---|
| RAM total | `sysinfo_darwin.go:26` | `sysctl -n hw.memsize` | `MemTotal` in `/proc/meminfo` | low |
| Free memory % | `sysinfo_darwin.go:80-86` | `memory_pressure` | `MemAvailable / MemTotal` | medium, see "Benchmark gate" |
| Swap used | `sysinfo_darwin.go:87-99` | `sysctl vm.swapusage` | `SwapTotal - SwapFree` | low |
| Pressure level | `sysinfo_darwin.go:101` | `kern.memorystatus_vm_pressure_level` | `/proc/pressure/memory` (PSI) mapped to 1, 2, 4, or a fixed 1 | low |
| GPU use and memory | `sysinfo_darwin.go:104-111` | `ioreg IOAccelerator` | `nvidia-smi --query-gpu`, `rocm-smi`, or `-1` (the code already accepts `-1`) | medium |
| Process-tree RSS | `sysinfo_darwin.go:43` | `ps -A -o pid=,ppid=,rss=` | same on procps, or walk `/proc/*/stat` | low |
| OS version | `sysinfo_darwin.go:33`, `config/result.go:10` | `sw_vers`, stored under JSON key `macos` | `/etc/os-release`; add an `os` field | low |
| Process owner check | `router/router.go:249-255` | `ps -p PID -o command=` | `/proc/PID/cmdline` (BusyBox `ps` lacks `-p` and `-o`) | medium |
| Port listener | `router/router.go:257-264` | `lsof -nP -iTCP:PORT -sTCP:LISTEN -t` | `ss -ltnpH` or `/proc/net/tcp`; `lsof` is often not installed | medium |
| Service | `internal/service/service.go:43,55,67`, `cmd/service.go:27` | LaunchAgent plist, `launchctl bootstrap/bootout/kickstart` | `~/.config/systemd/user/pill.service`, `systemctl --user enable --now / disable --now / restart`, `Restart=always`; start at boot needs `loginctl enable-linger` | high |
| Service PATH | `cmd/service.go:132` | adds `/opt/homebrew/bin` | `/usr/local/bin`, `~/.local/bin`, the Linuxbrew prefix | low |
| Stale-service parser | `service.go:117` | reads the plist back | parse `ExecStart=` | medium |
| Install hints | `doctor.go:58,64`, `service.go:61`, `pi.go:59`, `bench.go:108,111` | `brew install ...` | OS-aware hints | low |
| Pi and Node lookup | `pi/launch.go:15-23`, `doctor.go:248` | `exec.LookPath` | works as is if `pi` and `node` are on PATH | low |
| POSIX calls | `config/lock.go:42`, `router.go:356,434-450,458`, `bench/pirun.go:65-66`, `pi/launch.go:52` | `flock`, `Setsid`, `Setpgid`, `Kill`, `Exec` | valid on Linux; they compile | low |
| Config paths | `config/paths.go:27-37` | `~/.config/pill` (XDG honoured), `~/.pill` | works as is | low |
| GPU wording | `cmd/ps.go:58`, `sysinfo.go:17` | "Metal buffers" | wording only | low |

## Risks beyond compiling

### Benchmark gate

- The gate requires free memory of at least 10% across Tier 1 (`bench.go:61,181`) and again in `verdict` (`bench.go:263`).
- `MemAvailable` counts reclaimable page cache as available, and llama.cpp maps weights from the file, so the figure may stay high while the machine is under real stress. Not measured.
- With a discrete GPU the weights sit in VRAM, so system free % says nothing about whether the model fits.
- A usable Linux gate probably needs swap growth, PSI `some avg10` or VRAM free as well.
- Swap growth and tokens per second are recorded (`sampler.go:98-99`) but are not gate inputs today.
- Tier 1 allows 60 seconds for a cold load plus an ~8K-token prompt (`--tier1-max-seconds` overrides it). A CPU-only machine will probably exceed that for a 13.6 GB model. Not measured.

### Catalog

- One family (`gemma4-26b`) with two variants, both `min_ram_gb = 24` (`catalog.toml`).
- `Recommended` (`catalog.go:156`) returns the 12.9 GB variant for any machine under 24 GB, so a 16 GB CPU laptop is told to download something that barely fits.
- RAM is the wrong number for a discrete GPU.

### llama.cpp

- Router mode (`--models-preset`, `--models-max`, `--sleep-idle-seconds`) needs a recent llama.cpp. Distro packages may be older. `llamaVersion` parses the `version:` line (`doctor.go:226`).
- `ngl = 99`, `flash-attn = on` and the q8_0 KV cache (`preset.go:19-27`) were chosen for Metal. Check each on CPU, CUDA, ROCm and Vulkan builds.

### Containers

`/proc/meminfo` inside a container usually shows host or VM memory, not the cgroup limit, so free % there is misleading.

## Suggested order

| # | Slice | Effort | Risk |
|---|---|---|---|
| 1 | `sysinfo_linux.go` built from pure parsing functions, build tag `!darwin && !linux` on the stub, an ubuntu CI job, OS-aware hints, README, AGENTS.md and CHANGELOG | small | low |
| 2 | `service install` fails before writing anything on non-macOS, with a clear message | small | low |
| 3 | Replace or soften `ps` and `lsof` (`/proc` and `ss`), keeping the same `Find` behaviour | small to medium | medium |
| 4 | Catalog recommendation by usable memory (VRAM when present), plus a smaller catalog entry | medium | medium |
| 5 | Benchmark gate redesign for Linux (VRAM, PSI, thresholds) | medium | high |
| 6 | systemd user service behind an interface, with a fake `systemctl` in `internal/testutil` | medium to large | medium |

Slices 1 and 2 give a first Linux release: run `pill serve` and use the models from any agent, with `bench` and `service` reporting "not supported yet".

## How to test

### Without a Linux machine

| Layer | What | Notes |
|---|---|---|
| Cross-build | `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...`, same for `amd64` | needs no container |
| Unit tests | `go test ./...` in a `golang` container matching `go.mod` | uses the fake `llama-server`, `pi` and `launchctl`, so no model is needed. Install `procps` and `lsof` first; slim images lack `ps`. |
| Real `llama-server` on CPU | build llama.cpp or copy it from the official server image, then `pill pull hf.co/ggml-org/tinygemma3-GGUF:Q8_0`, `pill add ... --unverified --ctx N`, `pill serve`, `pill ps`, a `curl` to `/v1/chat/completions`, `pill stop --all` | call it from inside the container; `127.0.0.1` is not reachable from the host |
| Bench Tier 1 | exercises the plumbing, not model quality | the 47 MB model's context may be too small for the ~8K prompt, and it will not tool-call, so the Pi turn fails |
| `amd64` on an arm64 Mac | runs under emulation; slow, AVX support unverified | optional |

Not testable in a container: any GPU, systemd user sessions, real model speed and memory behaviour.

### CI

`.github/workflows/ci.yml` runs only on `macos-latest`. Add an `ubuntu-latest` job (or a matrix) running `go build`, `go vet` and `go test`. Once `sysinfo_linux.go` exists, macOS CI never compiles it, so the ubuntu job, or `GOOS=linux go vet` on macOS, is required.

Write the Linux `sysinfo` parsers as functions over text, with golden files for `/proc/meminfo`, PSI and `nvidia-smi` output, so they can be tested anywhere.

### Checklist on the Linux machine

1. Record: distro and version, architecture, GPU (none, NVIDIA, AMD, Intel), RAM and VRAM, whether a systemd user session exists, whether `ps`, `lsof` and `ss` are installed.
2. Install `llama-server` (note the route and `llama-server --version`) and Pi. Check `node --version`.
3. Build pill. Run `pill doctor`, `pill catalog`, `pill pull`, then `pill add <tiny model> --unverified --ctx 8192`.
4. Run `pill serve`, `pill ps`, a `curl` chat call, `pill stop`, `pill stop --all`. Check `pill ps` reports the right pid and `rss`.
5. Run `pill bench run <model>` with relaxed thresholds. Compare the sampler's free % and swap with `free -m` and `nvidia-smi` while it runs.
6. Run `pill` and confirm Pi opens on the model. After slice 6, test `service install`, a reboot and `uninstall`.

## Open questions

- Which Linux targets matter first: CPU-only, NVIDIA, AMD or Vulkan? This decides the gate design and the catalog.
- Is a first release that supports only `serve` and other agents acceptable?
- Should the benchmark gate be one rule everywhere or vary by platform?
- Should results rename the `macos` JSON key to `os`, given older `results.json` files carry the old key?
- Is a Docker-based real-model smoke test in CI worth the setup, or is manual checking enough?
