import { useState } from "react";
import type { FormEvent } from "react";
import "./App.css";

type Item = { title: string; text: string; price?: string; sub?: string };
type Section = { id: string; title: string; intro?: string; items?: Item[]; itemTestid?: string; cols?: number; faq?: boolean; form?: boolean };
const content = {
 "brand": "Ares Habitats",
 "tagline": "Own the first home on another planet",
 "promise": "Ares Habitats designs, ships and assembles pressurised glass-and-regolith domes on Mars. Reserve yours today and be among the first hundred owners when the first cargo fleet lands.",
 "cta": "Reserve your dome",
 "extraLabel": "Model of interest",
 "extraOptions": [
  "Outpost (1 to 2 people)",
  "Homestead (up to 6 people)",
  "Estate (up to 12 people)"
 ],
 "sections": [
  {
   "id": "models",
   "title": "The dome models",
   "faq": false,
   "form": false,
   "intro": "Every dome is built from a titanium frame, triple-layer transparent shielding and a regolith outer shell, and arrives with life support, power and a greenhouse module.",
   "items": [
    {
     "title": "Outpost",
     "price": "$1.2M",
     "text": "One bedroom, a lab bench and a small greenhouse. The sensible first step for a founder or a researcher."
    },
    {
     "title": "Homestead",
     "price": "$3.8M",
     "text": "Three bedrooms, a shared kitchen garden and a seven-day emergency reserve. Built for a family or a small crew."
    },
    {
     "title": "Estate",
     "price": "$9.5M",
     "text": "A 600 square metre habitat with a private observatory, workshop and a dedicated airlock to the surface."
    }
   ],
   "itemTestid": "dome-model"
  },
  {
   "id": "life",
   "title": "Life on Mars",
   "faq": false,
   "form": false,
   "intro": "Days last 24 hours and 39 minutes, so you will gain forty minutes of sleep every night for the first month. Mornings begin with a sunrise that is smaller, paler and bluer than the one you remember. You will tend crops under tuned light, take surface walks in a pressurised rover, and video-call Earth with a delay of between four and twenty-four minutes. Owners form a community of engineers, growers and artists. Gravity is thirty-eight percent of Earth's, so every jump becomes a small celebration."
  },
  {
   "id": "safety",
   "title": "Safety and logistics",
   "faq": false,
   "form": false,
   "intro": "We design for the failures that matter most: loss of pressure, radiation storms and power cuts. Each dome has three independent life-support loops, two metres of regolith shielding overhead and storm shelters sized for every resident. Cargo travels aboard our fleet of reusable landers on the next available window, roughly every twenty-six months. Construction crews of autonomous robots prepare the site before you arrive, so the dome is pressurised and tested before you ever step inside. Every owner trains for six months on Earth in our habitat simulator."
  },
  {
   "id": "reserve",
   "title": "Reserve your dome",
   "faq": false,
   "form": true,
   "intro": "A refundable deposit of one percent secures your place in the delivery queue. Tell us which model interests you and a member of our ownership team will contact you within one working day to explain the legal framework, the payment schedule and the training programme."
  },
  {
   "id": "faq",
   "title": "Frequently asked questions",
   "faq": true,
   "form": false
  }
 ],
 "faq": [
  {
   "q": "Who owns the land?",
   "a": "Under the Outer Space Treaty no nation can own Mars. You own the structure and a long lease on the site, recognised by our charter and by participating registries."
  },
  {
   "q": "When will my dome be delivered?",
   "a": "The first fleet launches in the next available window. Reservations are served in order, and we publish the schedule openly to every reservation holder."
  },
  {
   "q": "Is the deposit refundable?",
   "a": "Yes. You may cancel at any time before the first payment milestone and receive the full deposit back within thirty days."
  },
  {
   "q": "What if something goes wrong?",
   "a": "Every system is triple redundant and monitored from our operations centre on Earth. We also keep a rescue lander in orbit for the first ten years."
  },
  {
   "q": "Do I need to be an astronaut?",
   "a": "No. You need a medical clearance and six months of training, which covers life support, emergency drills and surface operations."
  }
 ],
 "footer": "Ares Habitats Inc. Dome prices are indicative and subject to confirmation at contract. Investing in space habitats involves significant risk."
} as {
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
