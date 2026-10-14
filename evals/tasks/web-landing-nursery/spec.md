Build a single-page marketing website for a fictional company (Montessori nursery) in this directory. It is a Vite + React + TypeScript project that is already scaffolded: `src/main.tsx` renders `src/App.tsx`, and `node_modules` is already installed. Do NOT run `npm install` and do not add dependencies: use only react and react-dom, plain CSS and TypeScript. Keep `tsconfig.json` strict. `npm run build` (which runs `tsc --noEmit && vite build`) must pass. Write your code under `src/`; you may add files and CSS there. A browser is not available to you, so reason carefully about layout and keep the code simple and robust.

## The company and its audience

A Montessori nursery for children aged 0 to 5. The audience is anxious, time-poor parents (and grandparents paying for care) who want to feel that their child will be safe, seen and gently challenged. Tone: warm, calm, reassuring and trustworthy, never salesy. Visual direction: natural, soft and unhurried; generous whitespace, friendly rounded forms, earthy or pastel palette.

## Requirements

- Invent the company name, all copy, prices, names and quotes. Write real, specific copy for the audience above (at least 500 words of copy). No lorem ipsum, no TODOs, no "coming soon".
- No external assets: no remote fonts, images, icon CDNs or scripts. Use system fonts, CSS and inline SVG for visuals. Decorative SVG should be `aria-hidden="true"`; meaningful images and `role="img"` SVG need a text alternative.
- Responsive from 390px to 1440px wide with no horizontal scrolling at either width.
- Accessible: one `h1`, headings in order without skipping levels, every section has a heading, sufficient colour contrast, labelled form fields, visible keyboard focus, a `lang` on the document.
- Design quality matters: a human will rate the look and feel of the page (typography, spacing, colour, hierarchy, tone) against the audience, in a blind comparison. Make deliberate design choices that fit this audience rather than a generic template.
- A single lead form with at least three labelled fields (name, email and one field specific to the business). Submitting it empty must show validation errors; submitting it filled in correctly must show a confirmation message on the page (no network calls, no page reload).

## Test contract (use exactly these `data-testid` values)

- `section-hero`: the opening section containing the `h1`, a one-sentence promise and a call to action to book a visit.
- `section-philosophy`: what the Montessori approach means in practice for a young child.
- `section-day`: a day in the life at the nursery, hour by hour.
- `section-teachers`: the teachers, with at least 3 `teacher-card` elements (name, role, one line about them).
- `section-enrol`: how to enrol or book a visit; contains the lead form (the business-specific field is the child's age).
- `section-faq`: frequently asked questions (fees, ratios, settling in, food and allergies).
- `lead-form`: the form element; `form-submit`: its submit button; `form-error`: a visible error message element shown when validation fails (and mark invalid fields with `aria-invalid="true"`); `form-success`: the confirmation element, only present after a valid submission.
- `faq-item`: each FAQ entry (at least 4). Use `<details>`/`<summary>` or a button with `aria-expanded` so it toggles.

When you are done, stop and reply with a one-line summary.
