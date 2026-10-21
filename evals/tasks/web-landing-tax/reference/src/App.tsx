import { useState } from "react";
import type { FormEvent } from "react";
import "./App.css";

type Item = { title: string; text: string; price?: string; sub?: string };
type Section = { id: string; title: string; intro?: string; items?: Item[]; itemTestid?: string; cols?: number; faq?: boolean; form?: boolean };
const content = {
 "brand": "LedgerLeaf",
 "tagline": "Your books and your taxes, done automatically",
 "promise": "LedgerLeaf connects to your bank, sorts every transaction, and files your tax return on time. Freelancers save an average of nine hours a month and never miss a deadline.",
 "cta": "Start free",
 "extraLabel": "Type of business",
 "extraOptions": [
  "Freelancer",
  "Sole trader",
  "Limited company",
  "Partnership"
 ],
 "sections": [
  {
   "id": "how",
   "title": "How it works",
   "faq": false,
   "form": false,
   "intro": "Set up takes about ten minutes, and after that LedgerLeaf runs in the background.",
   "items": [
    {
     "title": "1. Connect your accounts",
     "text": "Link your bank and card accounts with read-only access. We pull in twelve months of history automatically."
    },
    {
     "title": "2. We categorise everything",
     "text": "Our engine sorts income and expenses with 97 percent accuracy and asks you only about the unclear ones."
    },
    {
     "title": "3. We file and you approve",
     "text": "Before every deadline you review a one-page summary, tap approve, and we submit your return."
    }
   ],
   "itemTestid": "step"
  },
  {
   "id": "pricing",
   "title": "Simple pricing",
   "faq": false,
   "form": false,
   "intro": "Pay monthly, cancel any time. Every plan includes bank sync, receipt capture and deadline reminders.",
   "items": [
    {
     "title": "Solo",
     "price": "$19/mo",
     "text": "One bank connection, automatic bookkeeping and one annual tax return for sole traders."
    },
    {
     "title": "Studio",
     "price": "$39/mo",
     "text": "Unlimited connections, quarterly estimates, VAT returns and an invoice tool for growing businesses."
    },
    {
     "title": "Company",
     "price": "$89/mo",
     "text": "Everything in Studio plus payroll, accountant access and year-end company accounts."
    }
   ],
   "itemTestid": "pricing-tier"
  },
  {
   "id": "security",
   "title": "Security and compliance",
   "faq": false,
   "form": false,
   "intro": "Money is personal, so we keep this part boring and strict. Your financial data is encrypted in transit and at rest with AES-256. We have read-only access to your accounts and can never move money. Our systems are audited annually under SOC 2 Type II, and our filings are reviewed by chartered accountants before submission. Access is limited to you and anyone you invite, and you can export or delete all your data whenever you like. We never sell data and we never show you ads."
  },
  {
   "id": "testimonials",
   "title": "What customers say",
   "faq": false,
   "form": false,
   "intro": "More than twelve thousand freelancers, consultants and small studios use LedgerLeaf for their books and their returns. Here is what a few of them told us after their first filing season, in their own words and with their permission.",
   "items": [
    {
     "title": "Maya Lindqvist",
     "sub": "Freelance designer",
     "text": "I used to dread January. This year I approved my return on the sofa in four minutes."
    },
    {
     "title": "Tom\u00e1s Ferreira",
     "sub": "Owner, Ferreira Joinery",
     "text": "It caught eleven hundred dollars of expenses I had forgotten. It paid for itself in the first quarter."
    },
    {
     "title": "Aisha Rahman",
     "sub": "Consultant",
     "text": "My accountant now gets clean books automatically and charges me half as much."
    }
   ],
   "itemTestid": "testimonial"
  },
  {
   "id": "signup",
   "title": "Start your free month",
   "faq": false,
   "form": true,
   "intro": "Create your account in minutes. No card is needed for the first month, and you can switch plans or cancel at any time. Tell us about your business so we can set up the right defaults."
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
   "q": "Which countries do you support?",
   "a": "We file in the United States, the United Kingdom, Ireland, Canada and Australia, with more on the way."
  },
  {
   "q": "What if a filing turns out to be wrong?",
   "a": "We correct it for free and cover any penalties caused by our error, up to ten thousand dollars per year."
  },
  {
   "q": "Can I cancel any time?",
   "a": "Yes. There are no contracts. Cancel from your account page and we export all your records to a spreadsheet."
  },
  {
   "q": "Do you replace my accountant?",
   "a": "For most sole traders, yes. If you prefer to keep one, you can invite them to your account at no extra cost."
  },
  {
   "q": "How do you protect my bank login?",
   "a": "We use read-only connections through regulated open-banking providers. We never see or store your bank password."
  }
 ],
 "footer": "LedgerLeaf Ltd. Not a law firm. Tax figures shown are illustrative; your own results will depend on your circumstances."
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
