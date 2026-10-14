Build a single-page marketing website for a fictional company (automated accounting and tax filing) in this directory. It is a Vite + React + TypeScript project that is already scaffolded: `src/main.tsx` renders `src/App.tsx`, and `node_modules` is already installed. Do NOT run `npm install` and do not add dependencies: use only react and react-dom, plain CSS and TypeScript. Keep `tsconfig.json` strict. `npm run build` (which runs `tsc --noEmit && vite build`) must pass. Write your code under `src/`; you may add files and CSS there. A browser is not available to you, so reason carefully about layout and keep the code simple and robust.

## The company and its audience

A software service that automates bookkeeping and files taxes for freelancers and small-business owners. The audience is busy, sceptical self-employed people who dislike admin and fear mistakes with the tax authority. Tone: clean, credible, plain-spoken and efficient; concrete numbers, no hype. Visual direction: crisp and professional; restrained palette, strong typographic hierarchy, clear data-like details.

## Requirements

- Invent the company name, all copy, prices, names and quotes. Write real, specific copy for the audience above (at least 500 words of copy). No lorem ipsum, no TODOs, no "coming soon".
- No external assets: no remote fonts, images, icon CDNs or scripts. Use system fonts, CSS and inline SVG for visuals. Decorative SVG should be `aria-hidden="true"`; meaningful images and `role="img"` SVG need a text alternative.
- Responsive from 390px to 1440px wide with no horizontal scrolling at either width.
- Accessible: one `h1`, headings in order without skipping levels, every section has a heading, sufficient colour contrast, labelled form fields, visible keyboard focus, a `lang` on the document.
- Design quality matters: a human will rate the look and feel of the page (typography, spacing, colour, hierarchy, tone) against the audience, in a blind comparison. Make deliberate design choices that fit this audience rather than a generic template.
- A single lead form with at least three labelled fields (name, email and one field specific to the business). Submitting it empty must show validation errors; submitting it filled in correctly must show a confirmation message on the page (no network calls, no page reload).

## Test contract (use exactly these `data-testid` values)

- `section-hero`: the opening section containing the `h1`, a one-sentence promise and a call to action to start free.
- `section-how`: how it works, in three or four steps.
- `section-pricing`: pricing, with at least 3 `pricing-tier` elements, each showing a plan name, a price (for example `$29/mo`) and what is included.
- `section-security`: security and compliance (data protection, encryption, audits, who can see your data).
- `section-testimonials`: at least 2 `testimonial` elements (quote and the person's name and role).
- `section-faq`: frequently asked questions (supported countries, what if the filing is wrong, cancelling).
- `section-signup`: the sign-up section; contains the lead form (the business-specific field is the business type).
- `lead-form`: the form element; `form-submit`: its submit button; `form-error`: a visible error message element shown when validation fails (and mark invalid fields with `aria-invalid="true"`); `form-success`: the confirmation element, only present after a valid submission.
- `faq-item`: each FAQ entry (at least 4). Use `<details>`/`<summary>` or a button with `aria-expanded` so it toggles.

When you are done, stop and reply with a one-line summary.
