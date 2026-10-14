#!/usr/bin/env python3
"""Dev-time generator for the web-landing-* reference overlays (compliant, not beautiful).
Run: python3 evals/tasks/_web/make_reference.py   -> writes web-landing-*/reference/src/{App.tsx,App.css}"""
import json
from pathlib import Path

TASKS = Path(__file__).resolve().parent.parent

APP = r'''import { useState } from "react";
import type { FormEvent } from "react";
import "./App.css";

type Item = { title: string; text: string; price?: string; sub?: string };
type Section = { id: string; title: string; intro?: string; items?: Item[]; itemTestid?: string; cols?: number; faq?: boolean; form?: boolean };
const content = __CONTENT__ as {
  brand: string; tagline: string; promise: string; cta: string; extraLabel: string; extraOptions: string[];
  sections: Section[]; faq: { q: string; a: string }[]; footer: string;
};

function LeadForm() {
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [done, setDone] = useState(false);
  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const next: Record<string, string> = {};
    if (!String(f.get("name") ?? "").trim()) next.name = "Please enter your name.";
    if (!/^\S+@\S+\.\S+$/.test(String(f.get("email") ?? ""))) next.email = "Please enter a valid email address.";
    if (!String(f.get("extra") ?? "")) next.extra = "Please choose an option.";
    setErrors(next);
    setDone(Object.keys(next).length === 0);
  };
  return (
    <form data-testid="lead-form" onSubmit={submit} noValidate>
      <div className="field">
        <label htmlFor="f-name">Your name</label>
        <input id="f-name" name="name" autoComplete="name" aria-invalid={errors.name ? "true" : undefined} />
      </div>
      <div className="field">
        <label htmlFor="f-email">Email address</label>
        <input id="f-email" name="email" type="email" autoComplete="email" aria-invalid={errors.email ? "true" : undefined} />
      </div>
      <div className="field">
        <label htmlFor="f-extra">{content.extraLabel}</label>
        <select id="f-extra" name="extra" defaultValue="" aria-invalid={errors.extra ? "true" : undefined}>
          <option value="">Choose...</option>
          {content.extraOptions.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      </div>
      <div className="field">
        <label htmlFor="f-note">Anything we should know? (optional)</label>
        <textarea id="f-note" name="note" rows={3} />
      </div>
      {Object.keys(errors).length > 0 && (
        <p role="alert" className="error" data-testid="form-error">{Object.values(errors).join(" ")}</p>
      )}
      <button type="submit" className="btn" data-testid="form-submit">{content.cta}</button>
      {done && <p role="status" className="success" data-testid="form-success">Thank you. We have your details and will be in touch within one working day.</p>}
    </form>
  );
}

export default function App() {
  return (
    <>
      <header className="top">
        <strong className="brand">{content.brand}</strong>
        <nav aria-label="Main">
          {content.sections.map((s) => <a key={s.id} href={`#${s.id}`}>{s.title}</a>)}
        </nav>
      </header>
      <main>
        <section id="hero" data-testid="section-hero" className="hero">
          <h1>{content.tagline}</h1>
          <p className="lead">{content.promise}</p>
          <a className="btn" href={`#${content.sections.find((s) => s.form)!.id}`}>{content.cta}</a>
        </section>
        {content.sections.map((s) => (
          <section key={s.id} id={s.id} data-testid={`section-${s.id}`} className="block">
            <h2>{s.title}</h2>
            {s.intro && <p className="intro">{s.intro}</p>}
            {s.items && (
              <div className="grid">
                {s.items.map((it) => (
                  <article key={it.title} className="card" data-testid={s.itemTestid}>
                    <h3>{it.title}</h3>
                    {it.sub && <p className="sub">{it.sub}</p>}
                    {it.price && <p className="price">{it.price}</p>}
                    <p>{it.text}</p>
                  </article>
                ))}
              </div>
            )}
            {s.faq && (
              <div className="faq">
                {content.faq.map((f) => (
                  <details key={f.q} data-testid="faq-item"><summary>{f.q}</summary><p>{f.a}</p></details>
                ))}
              </div>
            )}
            {s.form && <LeadForm />}
          </section>
        ))}
      </main>
      <footer><p>{content.footer}</p></footer>
    </>
  );
}
'''

CSS = r'''* { box-sizing: border-box; }
html { scroll-behavior: smooth; }
body { margin: 0; font-family: __FONT__; background: var(--bg); color: var(--fg); line-height: 1.6; }
:root { --bg: __BG__; --fg: __FG__; --accent: __ACCENT__; --accent-fg: __ACCENT_FG__; --card: __CARD__; --muted: __MUTED__; }
a { color: var(--accent); }
:focus-visible { outline: 3px solid var(--accent); outline-offset: 2px; }
.top { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; justify-content: space-between; padding: 12px 20px; background: var(--card); }
.top nav { display: flex; flex-wrap: wrap; gap: 14px; }
.top nav a { color: var(--fg); font-size: 0.95rem; }
.brand { font-size: 1.2rem; }
.hero { padding: 72px 20px; max-width: 900px; margin: 0 auto; }
.hero h1 { font-size: clamp(2rem, 6vw, 3.4rem); line-height: 1.1; margin: 0 0 16px; }
.lead { font-size: 1.2rem; max-width: 640px; }
.block { padding: 48px 20px; max-width: 1000px; margin: 0 auto; }
.block h2 { font-size: 1.8rem; margin: 0 0 12px; }
.intro { max-width: 720px; }
.grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); margin-top: 16px; }
.card { background: var(--card); border-radius: 12px; padding: 18px; }
.card h3 { margin: 0 0 6px; }
.sub { color: var(--muted); margin: 0 0 6px; }
.price { font-size: 1.5rem; font-weight: 700; margin: 4px 0; }
.btn { display: inline-block; background: var(--accent); color: var(--accent-fg); border: 0; border-radius: 8px; padding: 12px 22px; font-size: 1rem; font-weight: 600; text-decoration: none; cursor: pointer; }
.faq details { background: var(--card); border-radius: 8px; padding: 12px 16px; margin-bottom: 8px; }
.faq summary { cursor: pointer; font-weight: 600; }
form { max-width: 520px; margin-top: 16px; }
.field { display: grid; gap: 4px; margin-bottom: 12px; }
input, select, textarea { font: inherit; padding: 10px; border: 2px solid var(--muted); border-radius: 8px; background: #fff; color: #111; width: 100%; }
[aria-invalid="true"] { border-color: #b00020; }
.error { color: var(--err, #b00020); font-weight: 600; }
.success { font-weight: 600; }
footer { padding: 24px 20px; text-align: center; background: var(--card); }
'''

def sec(id, title, intro=None, items=None, itemTestid=None, faq=False, form=False):
    d = {"id": id, "title": title, "faq": faq, "form": form}
    if intro: d["intro"] = intro
    if items: d["items"] = items; d["itemTestid"] = itemTestid
    return d

SITES = {}

SITES["nursery"] = dict(
    theme=dict(FONT='Georgia, "Times New Roman", serif', BG="#fbf6ee", FG="#3b2f26", ACCENT="#2f6b4f", ACCENT_FG="#ffffff", CARD="#f0e6d4", MUTED="#6b5a4c"),
    c=dict(
        brand="Little Acorn Montessori",
        tagline="A gentle first school where your child is seen, safe and curious",
        promise="Little Acorn is a small Montessori nursery for children from three months to five years, with warm teachers who know every child by name and a calm, unhurried day built around what your child is ready to learn.",
        cta="Book a visit",
        extraLabel="Your child's age",
        extraOptions=["Under 1 year", "1 to 2 years", "2 to 3 years", "3 to 5 years"],
        sections=[
            sec("philosophy", "Our philosophy", "Maria Montessori believed that children learn best when they are free to choose purposeful work in a prepared environment. At Little Acorn that is not a slogan; it shapes every room, shelf and routine. Your child will pour water, fold napkins, sort shells and trace sandpaper letters, because real tasks build real confidence. We follow the child, not the clock, and we never rush a milestone. Mixed-age rooms let the youngest watch and the oldest lead, so kindness and patience grow alongside early reading and number sense."),
            sec("day", "A day in the life", "We keep a steady rhythm so little ones always know what comes next. Morning arrivals are slow and personal, with a hug at the door and time to settle.", [
                dict(title="8:00 Settling in", text="Parents are welcome to stay a few minutes. Children greet their teacher and choose a quiet activity."),
                dict(title="9:00 Work cycle", text="A long, uninterrupted stretch of self-chosen activity: practical life, sensorial materials, early language and number."),
                dict(title="11:30 Lunch together", text="Children set the table and serve themselves a freshly cooked vegetarian meal. Allergies are handled with care."),
                dict(title="13:00 Rest and stories", text="Babies and toddlers nap in the quiet room while older children rest, listen to stories or look at books."),
                dict(title="15:00 Garden time", text="Growing vegetables, mud kitchens and climbing, whatever the weather. Pick-up is unhurried, with a short note on your child's day."),
            ], "day-slot"),
            sec("teachers", "Meet our teachers", "Every lead teacher holds a recognised Montessori diploma, a paediatric first aid certificate and an enhanced background check. Our ratio is never worse than one adult to four children.", [
                dict(title="Hannah Okafor", sub="Head of nursery", text="Twenty years in early years. Hannah loves watching a shy child discover that they can do it themselves."),
                dict(title="Luis Marchetti", sub="Toddler room lead", text="Trained in Bologna, Luis brings songs, patience and a very large collection of wooden animals."),
                dict(title="Priya Raman", sub="Pre-school guide", text="Priya prepares children for primary school through maps, music and the joy of writing their own name."),
            ], "teacher-card"),
            sec("enrol", "Enrol or visit us", "The best way to know if we are right for your family is to come and see a normal morning. Visits last forty minutes, there is always tea, and there is no obligation. Tell us a little about your child and we will call you to choose a time.", form=True),
            sec("faq", "Questions parents ask us", faq=True),
        ],
        faq=[
            dict(q="What are your fees?", a="Full days are 68 pounds and half days 42 pounds. We accept government funding for children aged three and over and will explain exactly how it applies to you."),
            dict(q="How do you help children settle in?", a="We plan a gradual start over two weeks, with short visits first and a key person who stays with your child. Parents can phone us at any time."),
            dict(q="What about food and allergies?", a="Meals are cooked on site, vegetarian and nut-free. We keep a written plan for every child with an allergy and all staff are trained to follow it."),
            dict(q="What is your adult to child ratio?", a="We are never below one adult to four children for under twos and one to six for older children, which is better than the legal minimum."),
            dict(q="Do you take babies?", a="Yes, from three months. Our baby room is quiet and softly lit, with floor beds and low mirrors, so little ones can move freely."),
        ],
        footer="Little Acorn Montessori, 14 Orchard Lane. Open Monday to Friday, 8:00 to 18:00. Registered with the local early years authority.",
    ))

SITES["mars"] = dict(
    theme=dict(FONT='"Helvetica Neue", Arial, sans-serif', BG="#120a08", FG="#f6ece6", ACCENT="#ff7a3d", ACCENT_FG="#1a0b05", CARD="#251512", MUTED="#d8b8a8"),
    c=dict(
        brand="Ares Habitats",
        tagline="Own the first home on another planet",
        promise="Ares Habitats designs, ships and assembles pressurised glass-and-regolith domes on Mars. Reserve yours today and be among the first hundred owners when the first cargo fleet lands.",
        cta="Reserve your dome",
        extraLabel="Model of interest",
        extraOptions=["Outpost (1 to 2 people)", "Homestead (up to 6 people)", "Estate (up to 12 people)"],
        sections=[
            sec("models", "The dome models", "Every dome is built from a titanium frame, triple-layer transparent shielding and a regolith outer shell, and arrives with life support, power and a greenhouse module.", [
                dict(title="Outpost", price="$1.2M", text="One bedroom, a lab bench and a small greenhouse. The sensible first step for a founder or a researcher."),
                dict(title="Homestead", price="$3.8M", text="Three bedrooms, a shared kitchen garden and a seven-day emergency reserve. Built for a family or a small crew."),
                dict(title="Estate", price="$9.5M", text="A 600 square metre habitat with a private observatory, workshop and a dedicated airlock to the surface."),
            ], "dome-model"),
            sec("life", "Life on Mars", "Days last 24 hours and 39 minutes, so you will gain forty minutes of sleep every night for the first month. Mornings begin with a sunrise that is smaller, paler and bluer than the one you remember. You will tend crops under tuned light, take surface walks in a pressurised rover, and video-call Earth with a delay of between four and twenty-four minutes. Owners form a community of engineers, growers and artists. Gravity is thirty-eight percent of Earth's, so every jump becomes a small celebration."),
            sec("safety", "Safety and logistics", "We design for the failures that matter most: loss of pressure, radiation storms and power cuts. Each dome has three independent life-support loops, two metres of regolith shielding overhead and storm shelters sized for every resident. Cargo travels aboard our fleet of reusable landers on the next available window, roughly every twenty-six months. Construction crews of autonomous robots prepare the site before you arrive, so the dome is pressurised and tested before you ever step inside. Every owner trains for six months on Earth in our habitat simulator."),
            sec("reserve", "Reserve your dome", "A refundable deposit of one percent secures your place in the delivery queue. Tell us which model interests you and a member of our ownership team will contact you within one working day to explain the legal framework, the payment schedule and the training programme.", form=True),
            sec("faq", "Frequently asked questions", faq=True),
        ],
        faq=[
            dict(q="Who owns the land?", a="Under the Outer Space Treaty no nation can own Mars. You own the structure and a long lease on the site, recognised by our charter and by participating registries."),
            dict(q="When will my dome be delivered?", a="The first fleet launches in the next available window. Reservations are served in order, and we publish the schedule openly to every reservation holder."),
            dict(q="Is the deposit refundable?", a="Yes. You may cancel at any time before the first payment milestone and receive the full deposit back within thirty days."),
            dict(q="What if something goes wrong?", a="Every system is triple redundant and monitored from our operations centre on Earth. We also keep a rescue lander in orbit for the first ten years."),
            dict(q="Do I need to be an astronaut?", a="No. You need a medical clearance and six months of training, which covers life support, emergency drills and surface operations."),
        ],
        footer="Ares Habitats Inc. Dome prices are indicative and subject to confirmation at contract. Investing in space habitats involves significant risk.",
    ))

SITES["tax"] = dict(
    theme=dict(FONT='system-ui, -apple-system, "Segoe UI", sans-serif', BG="#ffffff", FG="#14213d", ACCENT="#0b57d0", ACCENT_FG="#ffffff", CARD="#eef2f9", MUTED="#4a5a78"),
    c=dict(
        brand="LedgerLeaf",
        tagline="Your books and your taxes, done automatically",
        promise="LedgerLeaf connects to your bank, sorts every transaction, and files your tax return on time. Freelancers save an average of nine hours a month and never miss a deadline.",
        cta="Start free",
        extraLabel="Type of business",
        extraOptions=["Freelancer", "Sole trader", "Limited company", "Partnership"],
        sections=[
            sec("how", "How it works", "Set up takes about ten minutes, and after that LedgerLeaf runs in the background.", [
                dict(title="1. Connect your accounts", text="Link your bank and card accounts with read-only access. We pull in twelve months of history automatically."),
                dict(title="2. We categorise everything", text="Our engine sorts income and expenses with 97 percent accuracy and asks you only about the unclear ones."),
                dict(title="3. We file and you approve", text="Before every deadline you review a one-page summary, tap approve, and we submit your return."),
            ], "step"),
            sec("pricing", "Simple pricing", "Pay monthly, cancel any time. Every plan includes bank sync, receipt capture and deadline reminders.", [
                dict(title="Solo", price="$19/mo", text="One bank connection, automatic bookkeeping and one annual tax return for sole traders."),
                dict(title="Studio", price="$39/mo", text="Unlimited connections, quarterly estimates, VAT returns and an invoice tool for growing businesses."),
                dict(title="Company", price="$89/mo", text="Everything in Studio plus payroll, accountant access and year-end company accounts."),
            ], "pricing-tier"),
            sec("security", "Security and compliance", "Money is personal, so we keep this part boring and strict. Your financial data is encrypted in transit and at rest with AES-256. We have read-only access to your accounts and can never move money. Our systems are audited annually under SOC 2 Type II, and our filings are reviewed by chartered accountants before submission. Access is limited to you and anyone you invite, and you can export or delete all your data whenever you like. We never sell data and we never show you ads."),
            sec("testimonials", "What customers say", "More than twelve thousand freelancers, consultants and small studios use LedgerLeaf for their books and their returns. Here is what a few of them told us after their first filing season, in their own words and with their permission.", [
                dict(title="Maya Lindqvist", sub="Freelance designer", text="I used to dread January. This year I approved my return on the sofa in four minutes."),
                dict(title="Tomás Ferreira", sub="Owner, Ferreira Joinery", text="It caught eleven hundred dollars of expenses I had forgotten. It paid for itself in the first quarter."),
                dict(title="Aisha Rahman", sub="Consultant", text="My accountant now gets clean books automatically and charges me half as much."),
            ], "testimonial"),
            sec("signup", "Start your free month", "Create your account in minutes. No card is needed for the first month, and you can switch plans or cancel at any time. Tell us about your business so we can set up the right defaults.", form=True),
            sec("faq", "Frequently asked questions", faq=True),
        ],
        faq=[
            dict(q="Which countries do you support?", a="We file in the United States, the United Kingdom, Ireland, Canada and Australia, with more on the way."),
            dict(q="What if a filing turns out to be wrong?", a="We correct it for free and cover any penalties caused by our error, up to ten thousand dollars per year."),
            dict(q="Can I cancel any time?", a="Yes. There are no contracts. Cancel from your account page and we export all your records to a spreadsheet."),
            dict(q="Do you replace my accountant?", a="For most sole traders, yes. If you prefer to keep one, you can invite them to your account at no extra cost."),
            dict(q="How do you protect my bank login?", a="We use read-only connections through regulated open-banking providers. We never see or store your bank password."),
        ],
        footer="LedgerLeaf Ltd. Not a law firm. Tax figures shown are illustrative; your own results will depend on your circumstances.",
    ))

for kind, site in SITES.items():
    out = TASKS / f"web-landing-{kind}" / "reference" / "src"
    out.mkdir(parents=True, exist_ok=True)
    (out / "App.tsx").write_text(APP.replace("__CONTENT__", json.dumps(site["c"], indent=1)))
    css = CSS
    for k, v in site["theme"].items():
        css = css.replace(f"__{k}__", v)
    (out / "App.css").write_text(css)
    # the hero section is built into the template, so the section list holds the rest
    print("wrote", out)
