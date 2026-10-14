Build a single-page marketing website for a fictional company (Mars domes) in this directory. It is a Vite + React + TypeScript project that is already scaffolded: `src/main.tsx` renders `src/App.tsx`, and `node_modules` is already installed. Do NOT run `npm install` and do not add dependencies: use only react and react-dom, plain CSS and TypeScript. Keep `tsconfig.json` strict. `npm run build` (which runs `tsc --noEmit && vite build`) must pass. Write your code under `src/`; you may add files and CSS there. A browser is not available to you, so reason carefully about layout and keep the code simple and robust.

## The company and its audience

A company selling homes and habitat domes on Mars, with first deliveries in a few years. The audience is ambitious, high-net-worth early adopters and founders who want to be among the first people to own property on another planet. Tone: bold, confident, futuristic and aspirational, but credible about risk. Visual direction: dramatic and cinematic; dark backgrounds, warm Martian oranges and reds, strong large type and big contrasts.

## Requirements

- Invent the company name, all copy, prices, names and quotes. Write real, specific copy for the audience above (at least 500 words of copy). No lorem ipsum, no TODOs, no "coming soon".
- No external assets: no remote fonts, images, icon CDNs or scripts. Use system fonts, CSS and inline SVG for visuals. Decorative SVG should be `aria-hidden="true"`; meaningful images and `role="img"` SVG need a text alternative.
- Responsive from 390px to 1440px wide with no horizontal scrolling at either width.
- Accessible: one `h1`, headings in order without skipping levels, every section has a heading, sufficient colour contrast, labelled form fields, visible keyboard focus, a `lang` on the document.
- Design quality matters: a human will rate the look and feel of the page (typography, spacing, colour, hierarchy, tone) against the audience, in a blind comparison. Make deliberate design choices that fit this audience rather than a generic template.
- A single lead form with at least three labelled fields (name, email and one field specific to the business). Submitting it empty must show validation errors; submitting it filled in correctly must show a confirmation message on the page (no network calls, no page reload).

## Test contract (use exactly these `data-testid` values)

- `section-hero`: the opening section containing the `h1`, a one-sentence promise and a call to action to reserve a dome.
- `section-models`: the dome models for sale, with at least 3 `dome-model` elements, each showing a name, a short description and a price (for example `$1.2M`).
- `section-life`: what daily life on Mars is like for an owner.
- `section-safety`: safety, life support, radiation protection and logistics (how a dome gets to Mars).
- `section-reserve`: reserving a dome; contains the lead form (the business-specific field is the model of interest or budget).
- `section-faq`: frequently asked questions (ownership, legal status, delivery date, refunds).
- `lead-form`: the form element; `form-submit`: its submit button; `form-error`: a visible error message element shown when validation fails (and mark invalid fields with `aria-invalid="true"`); `form-success`: the confirmation element, only present after a valid submission.
- `faq-item`: each FAQ entry (at least 4). Use `<details>`/`<summary>` or a button with `aria-expanded` so it toggles.

When you are done, stop and reply with a one-line summary.
