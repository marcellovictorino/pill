import { useState } from "react";
import type { FormEvent } from "react";
import "./App.css";

type Item = { title: string; text: string; price?: string; sub?: string };
type Section = { id: string; title: string; intro?: string; items?: Item[]; itemTestid?: string; cols?: number; faq?: boolean; form?: boolean };
const content = {
 "brand": "Little Acorn Montessori",
 "tagline": "A gentle first school where your child is seen, safe and curious",
 "promise": "Little Acorn is a small Montessori nursery for children from three months to five years, with warm teachers who know every child by name and a calm, unhurried day built around what your child is ready to learn.",
 "cta": "Book a visit",
 "extraLabel": "Your child's age",
 "extraOptions": [
  "Under 1 year",
  "1 to 2 years",
  "2 to 3 years",
  "3 to 5 years"
 ],
 "sections": [
  {
   "id": "philosophy",
   "title": "Our philosophy",
   "faq": false,
   "form": false,
   "intro": "Maria Montessori believed that children learn best when they are free to choose purposeful work in a prepared environment. At Little Acorn that is not a slogan; it shapes every room, shelf and routine. Your child will pour water, fold napkins, sort shells and trace sandpaper letters, because real tasks build real confidence. We follow the child, not the clock, and we never rush a milestone. Mixed-age rooms let the youngest watch and the oldest lead, so kindness and patience grow alongside early reading and number sense."
  },
  {
   "id": "day",
   "title": "A day in the life",
   "faq": false,
   "form": false,
   "intro": "We keep a steady rhythm so little ones always know what comes next. Morning arrivals are slow and personal, with a hug at the door and time to settle.",
   "items": [
    {
     "title": "8:00 Settling in",
     "text": "Parents are welcome to stay a few minutes. Children greet their teacher and choose a quiet activity."
    },
    {
     "title": "9:00 Work cycle",
     "text": "A long, uninterrupted stretch of self-chosen activity: practical life, sensorial materials, early language and number."
    },
    {
     "title": "11:30 Lunch together",
     "text": "Children set the table and serve themselves a freshly cooked vegetarian meal. Allergies are handled with care."
    },
    {
     "title": "13:00 Rest and stories",
     "text": "Babies and toddlers nap in the quiet room while older children rest, listen to stories or look at books."
    },
    {
     "title": "15:00 Garden time",
     "text": "Growing vegetables, mud kitchens and climbing, whatever the weather. Pick-up is unhurried, with a short note on your child's day."
    }
   ],
   "itemTestid": "day-slot"
  },
  {
   "id": "teachers",
   "title": "Meet our teachers",
   "faq": false,
   "form": false,
   "intro": "Every lead teacher holds a recognised Montessori diploma, a paediatric first aid certificate and an enhanced background check. Our ratio is never worse than one adult to four children.",
   "items": [
    {
     "title": "Hannah Okafor",
     "sub": "Head of nursery",
     "text": "Twenty years in early years. Hannah loves watching a shy child discover that they can do it themselves."
    },
    {
     "title": "Luis Marchetti",
     "sub": "Toddler room lead",
     "text": "Trained in Bologna, Luis brings songs, patience and a very large collection of wooden animals."
    },
    {
     "title": "Priya Raman",
     "sub": "Pre-school guide",
     "text": "Priya prepares children for primary school through maps, music and the joy of writing their own name."
    }
   ],
   "itemTestid": "teacher-card"
  },
  {
   "id": "enrol",
   "title": "Enrol or visit us",
   "faq": false,
   "form": true,
   "intro": "The best way to know if we are right for your family is to come and see a normal morning. Visits last forty minutes, there is always tea, and there is no obligation. Tell us a little about your child and we will call you to choose a time."
  },
  {
   "id": "faq",
   "title": "Questions parents ask us",
   "faq": true,
   "form": false
  }
 ],
 "faq": [
  {
   "q": "What are your fees?",
   "a": "Full days are 68 pounds and half days 42 pounds. We accept government funding for children aged three and over and will explain exactly how it applies to you."
  },
  {
   "q": "How do you help children settle in?",
   "a": "We plan a gradual start over two weeks, with short visits first and a key person who stays with your child. Parents can phone us at any time."
  },
  {
   "q": "What about food and allergies?",
   "a": "Meals are cooked on site, vegetarian and nut-free. We keep a written plan for every child with an allergy and all staff are trained to follow it."
  },
  {
   "q": "What is your adult to child ratio?",
   "a": "We are never below one adult to four children for under twos and one to six for older children, which is better than the legal minimum."
  },
  {
   "q": "Do you take babies?",
   "a": "Yes, from three months. Our baby room is quiet and softly lit, with floor beds and low mirrors, so little ones can move freely."
  }
 ],
 "footer": "Little Acorn Montessori, 14 Orchard Lane. Open Monday to Friday, 8:00 to 18:00. Registered with the local early years authority."
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
