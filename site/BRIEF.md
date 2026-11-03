# Brief: pill landing page

Build a landing page for **pill**, a free, open-source command-line tool. Write exactly three files in the current directory: `index.html`, `style.css`, `script.js`. Plain HTML, CSS and JavaScript. No build step, no npm, no external libraries, no web fonts or images from the network (system font stacks only; draw every graphic with inline SVG or CSS). The page must work when `index.html` is opened from disk.

## What pill is (use only these facts)

pill runs the Pi coding agent against local language models served by llama.cpp's `llama-server`. Apple Silicon Macs only for now. Free and open source (MIT licence). Source: github.com/marcellovictorino/pill

## Look

- Brand motif: a pill. Use a pill/capsule shape as the logo, in dividers, buttons, badges and a large animated hero capsule (two halves, one cyan and one off-white, that can split, rotate or float).
- Colours: background is a gradient from dark navy `#05070d` to `#161c30`; accent is electric cyan `#7fdcff` with a soft glow (`box-shadow` / `text-shadow` / SVG blur); text and borders are off-white `#e9eef8` (use lower opacity for secondary text and thin borders).
- Dark theme only. Large confident type, generous spacing, thin 1px off-white borders, glowing cyan highlights.
- Responsive from 390 px wide phones to 1440 px desktops, no horizontal scrolling.
- Scroll animation: sections and cards fade and slide in as they enter the viewport (IntersectionObserver), the hero capsule reacts to scroll (parallax, rotation or splitting), plus at least one more scroll-driven effect (for example a progress bar, counting numbers, or a terminal that types its commands when it comes into view). Respect `prefers-reduced-motion` (turn motion off).

## Sections, in this order

1. **Hero.** Headline: "No subscription, no problem!" Below it, the joke, shown as three short lines that appear one after another: "You get tokens, you get tokens, you get tokens." (a nod to Oprah). One line saying pill runs coding-agent models on your own Mac. Buttons: "Get started" (scrolls to the install section) and a link to the GitHub repository.
2. **Runs on your Mac.** pill runs a local model that fits your machine, with Apple Silicon (M-series) in focus. `pill catalog` lists the models pill can download and shows the variant recommended for this machine's RAM. Everything stays on your computer: no account, no API key, no per-token bill.
3. **Pi, the lightest harness.** pill opens the Pi coding agent (github.com/badlogic/pi-mono). Pi is a deliberately minimal coding harness, so its prompt and tool overhead leave more of a local model's limited context window for your code. Say this plainly; do not invent token counts.
4. **Built on giants.** pill is a thin wrapper around llama.cpp (github.com/ggml-org/llama.cpp), adding the convenience and ergonomics people know from Ollama. Show these real commands with one-line descriptions, in a terminal-style block:
   - `pill catalog`: what pill can download, with the variant recommended for your RAM
   - `pill pull <name>`: download a model (resumes, verifies sha256)
   - `pill add <name> --unverified`: register it with the router and Pi
   - `pill ls`: list local models and their state (pulled, unverified, passed, failed, missing)
   - `pill default <name>`: choose the default model
   - `pill ps`: router state, port, loaded model and its memory
   - `pill serve`: start the router for any OpenAI-compatible client (http://127.0.0.1:11435/v1)
   - `pill stop`: unload the model and free memory
   - `pill`: there is no `run` command. Bare `pill` starts the router if needed and opens Pi on your default model. Say this explicitly.
5. **Benchmarked in tiers, hunting a champion.** `pill bench run <name>` gates every model: Tier 1 is a cold-start reply check, Tier 2 is a coding task scored by a hidden verifier (22/22 required), and the separate Tier 3 evaluations (Go, Python and web tasks including landing pages) show where each model is strong. The benchmark is re-run as new models are released or llama.cpp gains new flags, always looking for a new Champion. Current champion: MiniCPM-V 4.7 IQ2, which passed Tier 2 in 5 of 5 runs at about 25 tokens per second. Show the three tiers as a staircase or ladder with a crown/champion marker at the top. Command: `pill bench summary` ranks benchmarked models.
6. **Install.** Requirements: `brew install llama.cpp`, `brew install pi-coding-agent`. Install: `go install github.com/marcellovictorino/pill@latest`. Then: `pill pull gemma4-26b` (just an example from the catalog), `pill add gemma4-26b-iq4xs-64k --unverified`, `pill`. Each code block has a copy button (use `navigator.clipboard` with a fallback).
7. **Footer.** GitHub link, MIT licence, "Issues welcome. Pull requests are not accepted; please open an issue instead.", and the music credit (below).

## Music button

A small, fixed "Play the theme" button (pill-shaped, with an equaliser icon) toggles background music. The tune is "Pill" by Heuse & Zeus x Crona (feat. Emma Sameth), from NoCopyrightSounds. Use the official YouTube embed, created only after the click: an `<iframe>` with `src="https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1"`, `allow="autoplay; encrypted-media"`, visually tiny or hidden but present in the DOM. The clip runs from 0:24 to 1:02. Clicking again removes the iframe (stop). Never download, copy or bundle audio. Show a credit line in the footer: "Music: Pill by Heuse & Zeus x Crona (feat. Emma Sameth) [NCS Release]. Music provided by NoCopyrightSounds." linking to https://www.youtube.com/watch?v=CLiXUT3MS34. Add a short comment in script.js noting the YouTube player may not load when the page is opened from file://.

## Rules

- Real facts only; do not invent features, numbers, testimonials or commands.
- Semantic HTML, accessible labels, visible focus states, good contrast.
- Finish all three files; the page must be complete and polished.
