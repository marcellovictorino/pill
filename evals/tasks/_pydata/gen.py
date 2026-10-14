"""Synthetic subscription-business data shared by the py-clean and py-insight tasks.

Deterministic (seed 7). `build()` simulates the clean world; `dirty()` turns it into the messy
parquet tables the agent receives. Ground truth and the hidden churn labels are only ever
recomputed by verify.py through this module and never written into a workspace.

CLI (run with the _pydata venv python):
  gen.py write <dir>     write the four dirty parquet tables into <dir>
"""
import calendar
import json
import sys
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from pathlib import Path

import numpy as np
import pandas as pd
from faker import Faker

SEED = 7
START = date(2023, 1, 1)
T = date(2025, 12, 31)            # observation cutoff: nothing after this date is visible
LABEL_END = T + timedelta(days=90)  # churn label window: (T, T + 90 days]
PRICE_RISE = date(2024, 9, 1)     # +15% on every plan from this month; hazard spike that month

SEASONALITY = {1: 1.0, 2: 0.95, 3: 1.0, 4: 1.0, 5: 0.95, 6: 0.85,
               7: 0.7, 8: 0.7, 9: 1.0, 10: 1.1, 11: 1.5, 12: 1.55}
BASE_SIGNUPS = 55
GROWTH = 0.03
SEGMENTS = {"SMB": 0.65, "Mid": 0.28, "Enterprise": 0.07}
PLAN = {"SMB": ("Starter", 29.0), "Mid": ("Growth", 59.0), "Enterprise": ("Enterprise", 149.0)}
CHANNELS = {"organic": 0.30, "paid_search": 0.20, "paid_social": 0.20, "referral": 0.15, "partner": 0.15}
CHANNEL_MULT = {"organic": 0.8, "paid_search": 1.1, "paid_social": 2.2, "referral": 0.6, "partner": 0.7}
SEGMENT_MULT = {"SMB": 1.2, "Mid": 0.8, "Enterprise": 0.4}
COUNTRIES = ["GB", "DE", "FR", "US", "ES", "NL", "IT", "PL", "SE", "IE"]
COUNTRY_P = [0.28, 0.14, 0.12, 0.16, 0.07, 0.06, 0.05, 0.04, 0.04, 0.04]
COUNTRY_VARIANTS = {
    "GB": ["GB", "UK", "United Kingdom", "uk", "Great Britain"],
    "DE": ["DE", "Germany", "Deutschland", "de"],
    "FR": ["FR", "France", "fr"],
    "US": ["US", "USA", "United States", "us"],
    "ES": ["ES", "Spain", "Espana"],
    "NL": ["NL", "Netherlands", "The Netherlands"],
    "IT": ["IT", "Italy", "Italia"],
    "PL": ["PL", "Poland", "Polska"],
    "SE": ["SE", "Sweden", "Sverige"],
    "IE": ["IE", "Ireland", "Eire"],
}
CHURN_REASONS = ["missing_feature", "support", "competitor", "no_longer_needed", "price"]
TICKET_CATEGORIES = ["billing", "bug", "how_to", "outage", "feature_request"]


@dataclass
class World:
    customers: pd.DataFrame      # clean, one row per customer
    subs: pd.DataFrame           # clean, end_date NaT when active at T
    payments: pd.DataFrame       # clean, truncated at T, refunds as separate rows
    tickets: pd.DataFrame        # clean, truncated at T
    labels: pd.DataFrame         # customer_id, label for customers active at T (HIDDEN)
    full_end: pd.Series          # untruncated end date by customer (HIDDEN)


def _month_starts():
    d, out = date(START.year, START.month, 1), []
    while d <= T:
        out.append(d)
        d = date(d.year + (d.month == 12), d.month % 12 + 1, 1)
    return out


def _days_in(d):
    return calendar.monthrange(d.year, d.month)[1]


def _add_months(d, n):
    m = d.month - 1 + n
    return date(d.year + m // 12, m % 12 + 1, 1)


def price_at(seg, when):
    base = PLAN[seg][1]
    return round(base * 1.15, 2) if when >= PRICE_RISE else base


def build(seed=SEED):
    rng = np.random.default_rng(seed)
    fk = Faker("en_GB")
    Faker.seed(seed)
    fk.seed_instance(seed)
    months = _month_starts()
    cust, subs, pays, tix = [], [], [], []
    pay_n = tix_n = cid = 0

    for mi, m0 in enumerate(months):
        n = rng.poisson(BASE_SIGNUPS * (1 + GROWTH) ** mi * SEASONALITY[m0.month])
        for _ in range(n):
            cid += 1
            cust_id = f"C{cid:06d}"
            seg = rng.choice(list(SEGMENTS), p=list(SEGMENTS.values()))
            ch = rng.choice(list(CHANNELS), p=list(CHANNELS.values()))
            country = rng.choice(COUNTRIES, p=COUNTRY_P)
            day = int(rng.integers(1, _days_in(m0) + 1))
            signup = date(m0.year, m0.month, day)
            first, last = fk.first_name(), fk.last_name()
            email = f"{first}.{last}{cid}@{fk.free_email_domain()}".lower().replace("'", "")
            cust.append(dict(customer_id=cust_id, name=f"{first} {last}", email=email, signup_date=signup,
                             country=country, segment=seg, channel=ch))

            lam = {"SMB": 1.1, "Mid": 1.2, "Enterprise": 1.5}[seg]
            n60 = int(rng.poisson(lam))
            frailty = float(rng.gamma(4.0, 0.25))

            # monthly hazard simulation: k = calendar months since signup month
            churn_k, end = None, None
            k = 0
            while True:
                mk = _add_months(date(signup.year, signup.month, 1), k)
                if mk > LABEL_END:
                    break
                age = 3.5 if k < 3 else (1.0 if k < 6 else 0.5)
                h = 0.028 * age * CHANNEL_MULT[ch] * SEGMENT_MULT[seg] * frailty
                if k >= 1 and n60 >= 3:
                    h *= 3.0
                if mk == PRICE_RISE:
                    h *= 3.0
                if rng.random() < 1 - np.exp(-h):
                    churn_k = k
                    if k == 0:
                        left = _days_in(mk) - signup.day
                        end = signup + timedelta(days=int(rng.integers(1, left + 1)) if left > 0 else 1)
                    else:
                        end = mk + timedelta(days=int(rng.integers(0, _days_in(mk))))
                    break
                k += 1

            reason = None
            if churn_k is not None:
                reason = "price" if (end.year, end.month) == (PRICE_RISE.year, PRICE_RISE.month) and rng.random() < 0.7 \
                    else str(rng.choice(CHURN_REASONS))
            subs.append(dict(customer_id=cust_id, plan=PLAN[seg][0], start_date=signup,
                             end_date=end, monthly_price=price_at(seg, signup), churn_reason=reason))

            # payments on the signup day-of-month (clipped to 28) while the subscription is alive
            bday = min(signup.day, 28)
            j = 0
            while True:
                mj = _add_months(date(signup.year, signup.month, 1), j)
                d = signup if j == 0 else date(mj.year, mj.month, bday)
                if d > T or (end is not None and d >= end):
                    break
                p_fail = 0.025
                if churn_k is not None and j == churn_k - 1:
                    p_fail = 0.30
                elif churn_k is not None and j == churn_k:
                    p_fail = 0.50
                ts = datetime(d.year, d.month, d.day, int(rng.integers(0, 24)), int(rng.integers(0, 60)),
                              int(rng.integers(0, 60)))
                amt = price_at(seg, d)
                pay_n += 1
                status = "failed" if rng.random() < p_fail else "paid"
                pays.append(dict(payment_id=f"P{pay_n:08d}", customer_id=cust_id, paid_at=ts, amount=amt,
                                 status=status))
                if status == "paid" and rng.random() < 0.015:
                    rts = ts + timedelta(days=int(rng.integers(1, 10)), hours=int(rng.integers(0, 6)))
                    if rts.date() <= T:
                        pay_n += 1
                        pays.append(dict(payment_id=f"P{pay_n:08d}", customer_id=cust_id, paid_at=rts,
                                         amount=amt, status="refunded"))
                j += 1

            # tickets: n60 in the first 60 days, then a trickle
            for _ in range(n60):
                td = signup + timedelta(days=int(rng.integers(0, 60)))
                if td <= T and (end is None or td < end):
                    tix_n += 1
                    tix.append((tix_n, cust_id, td))
            life_end = min(end - timedelta(days=1) if end else T, T)
            d = signup + timedelta(days=60)
            while d <= life_end:
                if rng.random() < 0.12 / 30:
                    tix_n += 1
                    tix.append((tix_n, cust_id, d))
                d += timedelta(days=1)

    customers = pd.DataFrame(cust)
    subs = pd.DataFrame(subs)
    full_end = subs.set_index("customer_id")["end_date"]
    subs_vis = subs.copy()
    subs_vis["end_date"] = [e if (e is not None and e <= T) else None for e in subs["end_date"]]
    subs_vis["churn_reason"] = [r if e is not None else None for r, e in zip(subs["churn_reason"], subs_vis["end_date"])]
    payments = pd.DataFrame(pays).sort_values("paid_at", kind="stable").reset_index(drop=True)

    tk = pd.DataFrame(tix, columns=["n", "customer_id", "day"])
    trows = []
    for n_, c, d in tk.itertuples(index=False):
        h = int(rng.integers(0, 24))
        sev = str(rng.choice(["low", "medium", "high"], p=[0.5, 0.35, 0.15]))
        res = None if rng.random() < 0.04 else round(float(rng.lognormal(2.2, 0.8)), 1)
        trows.append(dict(ticket_id=f"T{n_:07d}", customer_id=c,
                          opened_at=datetime(d.year, d.month, d.day, h, int(rng.integers(0, 60))),
                          category=str(rng.choice(TICKET_CATEGORIES)), severity=sev, resolution_hours=res))
    tickets = pd.DataFrame(trows)

    active = [(c, e) for c, e in zip(subs_vis["customer_id"], subs_vis["end_date"]) if e is None]
    lab = []
    for c, _ in active:
        fe = full_end[c]
        lab.append(dict(customer_id=c, label=int(fe is not None and T < fe <= LABEL_END)))
    return World(customers, subs_vis, payments, tickets, pd.DataFrame(lab), full_end)


def _fmt_date(d, rng):
    r = rng.random()
    if r < 0.6:
        return d.strftime("%Y-%m-%d")
    if r < 0.8:
        return d.strftime("%d/%m/%Y")        # day first
    return d.strftime("%b %d, %Y")


def _fmt_ts(t, rng):
    r = rng.random()
    if r < 0.5:
        return t.strftime("%Y-%m-%dT%H:%M:%SZ")
    if r < 0.8:
        return t.strftime("%Y-%m-%d %H:%M:%S")
    return t.strftime("%d/%m/%Y %H:%M")      # day first, minutes only


def _noisy_email(e, rng):
    r = rng.random()
    if r < 0.08:
        return e.upper()
    if r < 0.14:
        return f"  {e} "
    if r < 0.18:
        return e.title()
    return e


def dirty(w, seed=SEED):
    rng = np.random.default_rng(seed + 1000)
    c = w.customers
    rows = []
    for r in c.itertuples(index=False):
        seg = r.segment
        ch = r.channel
        x = rng.random()
        seg_d = seg.lower() if x < 0.04 else (f" {seg} " if x < 0.07 else seg)
        y = rng.random()
        ch_d = ch.replace("_", " ").title() if y < 0.04 else (ch.replace("_", "-") if y < 0.07 else ch)
        rows.append(dict(customer_id=r.customer_id, name=r.name, email=_noisy_email(r.email, rng),
                         signup_date=_fmt_date(r.signup_date, rng),
                         country=str(rng.choice(COUNTRY_VARIANTS[r.country])), segment=seg_d, channel=ch_d))
    cust = pd.DataFrame(rows)
    dup_idx = rng.choice(len(cust), size=int(len(cust) * 0.03), replace=False)
    dups = cust.iloc[dup_idx].copy()
    dups["email"] = [_noisy_email(e.strip().lower(), rng) for e in dups["email"]]
    dups["signup_date"] = [_fmt_date(_parse_any(sv), rng) for sv in dups["signup_date"]]
    cust = pd.concat([cust, dups], ignore_index=True).sample(frac=1, random_state=seed).reset_index(drop=True)
    n_dup_cust = len(dups)

    s = w.subs.copy()
    s["start_date"] = [d.isoformat() for d in s["start_date"]]
    s["end_date"] = [d.isoformat() if d is not None and not pd.isna(d) else None for d in s["end_date"]]
    s["monthly_price"] = [f"€{p:.2f}" if rng.random() < 0.7 else f"{p:.2f} EUR" for p in s["monthly_price"]]
    s = s.sample(frac=1, random_state=seed + 1).reset_index(drop=True)

    p = w.payments.copy()
    p["paid_at"] = [_fmt_ts(t, rng) for t in p["paid_at"]]
    p["status"] = [st if rng.random() > 0.08 else str(rng.choice([st.upper(), st.title()])) for st in p["status"]]
    n_orphan = int(len(p) * 0.006)
    base = p.sample(n=n_orphan, random_state=seed + 2).copy()
    base["customer_id"] = [f"C9{int(rng.integers(0, 99999)):05d}" for _ in range(n_orphan)]
    base["payment_id"] = [f"PX{i:07d}" for i in range(n_orphan)]
    n_dup_pay = int(len(p) * 0.02)
    dp = p.sample(n=n_dup_pay, random_state=seed + 3)
    p = pd.concat([p, base, dp], ignore_index=True).sample(frac=1, random_state=seed + 4).reset_index(drop=True)

    t = w.tickets.copy()
    t["opened_at"] = [x.strftime("%Y-%m-%dT%H:%M:%S") for x in t["opened_at"]]

    counts = dict(duplicate_customer_rows=int(n_dup_cust), duplicate_payment_rows=int(n_dup_pay),
                  orphan_payment_rows=int(n_orphan))
    return dict(customers=cust, subscriptions=s, payments=p, tickets=t), counts


def _parse_any(sv):
    sv = sv.strip()
    for f in ("%Y-%m-%d", "%d/%m/%Y", "%b %d, %Y"):
        try:
            return datetime.strptime(sv, f).date()
        except ValueError:
            pass
    raise ValueError(sv)


def truth_patterns(w):
    """The planted patterns with empirical evidence, used by judge.py and the keyword check."""
    c = w.customers.merge(w.subs[["customer_id", "end_date"]], on="customer_id")
    c["churned"] = c["end_date"].notna()
    c["signup_date"] = pd.to_datetime(c["signup_date"])
    c["month"] = c["signup_date"].dt.month
    new = c.groupby(c["signup_date"].dt.to_period("M")).size()
    peak = new[new.index.month.isin([11, 12])].mean()
    dip = new[new.index.month.isin([7, 8])].mean()
    # trend: compare same calendar months a year apart
    y1 = new[new.index.year == 2023].sum()
    y3 = new[new.index.year == 2025].sum()
    old = c[c["signup_date"] <= pd.Timestamp(T - timedelta(days=180))]
    by_ch = {str(k): v for k, v in old.groupby("channel")["churned"].mean().round(3).to_dict().items()}
    tk = w.tickets.copy()
    tk = tk.merge(w.customers[["customer_id", "signup_date"]], on="customer_id")
    tk["d"] = (pd.to_datetime(tk["opened_at"]).dt.normalize() - pd.to_datetime(tk["signup_date"])).dt.days
    n60 = tk[tk["d"] <= 60].groupby("customer_id").size()
    old = old.assign(n60=old["customer_id"].map(n60).fillna(0))
    # tickets only exist while a customer is alive, so compare customers who survived their first 60 days
    surv = old[old["end_date"].isna() | (pd.to_datetime(old["end_date"]) - old["signup_date"]).dt.days.gt(60)]
    t3 = surv[surv["n60"] >= 3]["churned"].mean()
    t_lo = surv[surv["n60"] < 3]["churned"].mean()
    by_seg = {str(k): v for k, v in old.groupby("segment")["churned"].mean().round(3).to_dict().items()}
    ch = c[c["churned"]].copy()
    ch["end_date"] = pd.to_datetime(ch["end_date"])
    ch["life"] = (ch["end_date"] - ch["signup_date"]).dt.days
    early = float((ch["life"] <= 90).mean())
    per_month = ch.groupby(ch["end_date"].dt.to_period("M")).size()
    sep = int(per_month.get(pd.Period("2024-09"), 0))
    around = float(np.mean([per_month.get(pd.Period(p), 0) for p in ("2024-07", "2024-08", "2024-10", "2024-11")]))
    return {
        "seasonality": f"New-customer signups peak in Nov-Dec (avg {peak:.0f}/month, ~1.5x index) and dip in Jul-Aug "
                       f"(avg {dip:.0f}/month, ~0.7x); revenue follows with a lag. Planted index: {SEASONALITY}.",
        "growth_trend": f"Steady growth in monthly signups of ~3%/month compounding (2023: {y1} signups, 2025: {y3}).",
        "churn_by_channel": f"paid_social churns far more than other channels (churned share by channel for "
                            f"customers >=180 days old: {by_ch}); referral/partner/organic are lowest.",
        "churn_by_tickets": f"Customers with >=3 support tickets in their first 60 days churn much more "
                            f"({t3:.0%} vs {t_lo:.0%} for <3 tickets, among customers who survived 60 days).",
        "enterprise": f"Enterprise has ~5x the ARPU of SMB (EUR 149 vs 29 list price) and the lowest churn "
                      f"(churned share by segment: {by_seg}).",
        "price_increase": f"A 15% price increase took effect 2024-09: churn spikes that month ({sep} churn events vs "
                          f"~{around:.0f}/month around it) and 'price' dominates churn reasons.",
        "early_churn": f"Most churn happens early: {early:.0%} of all churn events occur within 90 days of signup.",
    }


def write(outdir):
    w = build()
    d, _ = dirty(w)
    out = Path(outdir)
    out.mkdir(parents=True, exist_ok=True)
    for name, df in d.items():
        df.to_parquet(out / f"{name}.parquet", index=False)


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "write":
        write(sys.argv[2])
    elif len(sys.argv) == 3 and sys.argv[1] == "truth":
        Path(sys.argv[2]).write_text(json.dumps(truth_patterns(build()), indent=1))
    else:
        sys.exit(__doc__)
