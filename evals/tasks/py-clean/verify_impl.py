"""Hidden verifier for py-clean. Recomputes every output from the generator's clean world.

python verify_impl.py <workspace>  -> last stdout line is JSON {"score","total","checks"}.
Whole-column matches earn full weight at >=99.9% matching rows, otherwise weight * fraction**3.
"""
import json
import re
import sys
from datetime import date
from pathlib import Path

import numpy as np
import pandas as pd

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "_pydata"))
import gen  # noqa: E402

ws = Path(sys.argv[1])
out = ws / "out"
checks = []


def add(name, weight, frac, detail=""):
    frac = max(0.0, min(1.0, float(frac)))
    pts = weight if frac >= 0.999 else weight * frac ** 3
    checks.append({"name": name, "pass": frac >= 0.999, "detail": detail or f"{frac:.1%} match",
                   "points": round(pts, 2), "weight": weight})


def read(name, kind):
    p = out / name
    if not p.exists():
        return None
    try:
        return pd.read_parquet(p) if kind == "parquet" else pd.read_csv(p)
    except Exception as e:  # unreadable file counts as missing
        checks.append({"name": f"read {name}", "pass": False, "detail": str(e)[:200], "points": 0, "weight": 0})
        return None


def col_match(got, want, key, col, money=False):
    m = want[[key, col]].merge(got[[key, col]], on=key, how="left", suffixes=("_w", "_g"))
    a, b = m[f"{col}_w"], m[f"{col}_g"]
    if money:
        ok = (pd.to_numeric(b, errors="coerce") - a.astype(float)).abs() <= 0.01
    else:
        ok = a.astype(str) == b.astype(str)
    return float(ok.fillna(False).mean())


w = gen.build()
_, counts = gen.dirty(w)

# ---------------- truth ----------------
cust = w.customers.copy()
cust["email"] = cust["email"].str.lower()
subs = w.subs.copy()
pay = w.payments.copy()
pay["month"] = pay["paid_at"].dt.strftime("%Y-%m")
months = pd.period_range("2023-01", "2025-12", freq="M")
rows = []
for m in months:
    last = m.end_time.date()
    pm = pay[pay["month"] == str(m)]
    paid = round(float(pm.loc[pm.status == "paid", "amount"].sum()), 2)
    ref = round(float(pm.loc[pm.status == "refunded", "amount"].sum()), 2)
    act = int(((subs["start_date"] <= last) & (subs["end_date"].isna() | (subs["end_date"] > last))).sum())
    new = int(((cust["signup_date"] >= m.start_time.date()) & (cust["signup_date"] <= last)).sum())
    rows.append(dict(month=str(m), paid_revenue_eur=paid, refunds_eur=ref, net_eur=round(paid - ref, 2),
                     active_customers=act, new_customers=new))
t_month = pd.DataFrame(rows)

ok_pay = pay[pay["status"] == "paid"].groupby("customer_id")["amount"].agg(total_paid="sum", n_payments="size")
tk = w.tickets.merge(cust[["customer_id", "signup_date"]], on="customer_id")
tk["d"] = (tk["opened_at"].dt.normalize() - pd.to_datetime(tk["signup_date"])).dt.days
t360 = cust[["customer_id", "signup_date", "country", "segment", "channel"]].merge(
    subs[["customer_id", "end_date"]], on="customer_id")
t360["signup_month"] = pd.to_datetime(t360["signup_date"]).dt.strftime("%Y-%m")
t360["churned"] = t360["end_date"].notna()
t360["tenure_days"] = [((e if c else gen.T) - s).days for s, e, c in
                       zip(t360["signup_date"], t360["end_date"], t360["churned"])]
t360 = t360.merge(ok_pay, left_on="customer_id", right_index=True, how="left")
t360["total_paid"] = t360["total_paid"].fillna(0.0).round(2)
t360["n_payments"] = t360["n_payments"].fillna(0).astype(int)
t360["n_tickets"] = t360["customer_id"].map(tk.groupby("customer_id").size()).fillna(0).astype(int)
t360["tickets_first_60d"] = t360["customer_id"].map(tk[tk["d"] <= 60].groupby("customer_id").size()).fillna(0).astype(int)

# ---------------- 1. clean_customers (25) ----------------
cc = read("clean_customers.parquet", "parquet")
if cc is None:
    add("clean_customers.parquet exists and is readable", 25, 0, "missing")
else:
    need = ["customer_id", "name", "email", "signup_date", "country", "segment", "channel"]
    missing = [c for c in need if c not in cc.columns]
    if missing:
        add("clean_customers columns", 25, 0, f"missing {missing}")
    else:
        add("clean_customers: readable with required columns", 3, 1)
        uniq = cc["customer_id"].is_unique and set(cc["customer_id"]) == set(cust["customer_id"])
        add("clean_customers: one row per customer, exact id set", 6, 1 if uniq else
            min(len(set(cc["customer_id"]) & set(cust["customer_id"])) / len(cust), 0.9),
            f"{len(cc)} rows, {cc['customer_id'].nunique()} unique, expected {len(cust)}")
        g = cc.drop_duplicates("customer_id")
        add("clean_customers: country ISO-2", 5, col_match(g, cust, "customer_id", "country"))
        sd = g.copy()
        sd["signup_date"] = pd.to_datetime(sd["signup_date"], errors="coerce").dt.date
        want = cust.copy()
        add("clean_customers: signup_date values (day-first parsing)", 4, col_match(sd, want, "customer_id", "signup_date"))
        import pyarrow.parquet as pq
        t = str(pq.read_schema(out / "clean_customers.parquet").field("signup_date").type)
        add("clean_customers: signup_date is a date/timestamp column", 1, 1 if ("date" in t or "timestamp" in t) else 0, t)
        add("clean_customers: email trimmed and lower-case", 3, col_match(g, cust, "customer_id", "email"))
        add("clean_customers: segment and channel normalised", 3,
            (col_match(g, cust, "customer_id", "segment") + col_match(g, cust, "customer_id", "channel")) / 2)

# ---------------- 2. monthly_revenue (40) ----------------
mr = read("monthly_revenue.csv", "csv")
if mr is None:
    add("monthly_revenue.csv exists and is readable", 40, 0, "missing")
else:
    cols = list(t_month.columns)
    if any(c not in mr.columns for c in cols):
        add("monthly_revenue columns", 40, 0, f"need {cols}, got {list(mr.columns)}")
    else:
        mr["month"] = mr["month"].astype(str)
        shape_ok = list(mr["month"]) == list(t_month["month"])
        add("monthly_revenue: 36 ascending months 2023-01..2025-12", 6, 1 if shape_ok else
            len(set(mr["month"]) & set(t_month["month"])) / 36, f"{len(mr)} rows")
        for col, wt, money in (("paid_revenue_eur", 9, True), ("refunds_eur", 5, True), ("net_eur", 4, True),
                               ("active_customers", 8, False), ("new_customers", 8, False)):
            if money:
                f = col_match(mr, t_month, "month", col, money=True)
            else:
                m = t_month[["month", col]].merge(mr[["month", col]], on="month", how="left", suffixes=("_w", "_g"))
                f = float((m[f"{col}_w"] == m[f"{col}_g"]).fillna(False).mean())
            add(f"monthly_revenue: {col}", wt, f)

# ---------------- 3. customer_360 (30) ----------------
c3 = read("customer_360.parquet", "parquet")
if c3 is None:
    add("customer_360.parquet exists and is readable", 30, 0, "missing")
else:
    need = ["customer_id", "signup_month", "country", "segment", "channel", "tenure_days", "total_paid",
            "n_payments", "n_tickets", "tickets_first_60d", "churned"]
    missing = [c for c in need if c not in c3.columns]
    if missing:
        add("customer_360 columns", 30, 0, f"missing {missing}")
    else:
        ids_ok = c3["customer_id"].is_unique and set(c3["customer_id"]) == set(t360["customer_id"])
        add("customer_360: one row per customer, exact id set, required columns", 5, 1 if ids_ok else 0.3,
            f"{len(c3)} rows")
        c3 = c3.drop_duplicates("customer_id")
        for col, wt in (("signup_month", 2), ("country", 1), ("segment", 1), ("channel", 1), ("tenure_days", 4),
                        ("n_payments", 3), ("n_tickets", 3), ("tickets_first_60d", 4), ("churned", 2)):
            g = c3.copy()
            if col in ("tenure_days", "n_payments", "n_tickets", "tickets_first_60d"):
                g[col] = pd.to_numeric(g[col], errors="coerce").round().astype("Int64")
            if col == "churned":
                g[col] = g[col].astype(bool)
            add(f"customer_360: {col}", wt, col_match(g, t360, "customer_id", col))
        add("customer_360: total_paid", 4, col_match(c3, t360, "customer_id", "total_paid", money=True))

# ---------------- 4. data_quality.md (5) ----------------
dq = out / "data_quality.md"
if not dq.exists():
    add("data_quality.md exists", 5, 0, "missing")
else:
    text = dq.read_text()
    nums = set(re.findall(r"\d+", text))
    hits = [str(counts[k]) in nums for k in ("duplicate_customer_rows", "duplicate_payment_rows", "orphan_payment_rows")]
    f = (sum(hits) / 3) * (1 if len(text) >= 300 else 0.5)
    add("data_quality.md reports the true duplicate/orphan counts", 5, f, f"{sum(hits)}/3 true counts found, truth={counts}")

score = round(sum(c["points"] for c in checks))
print(json.dumps({"score": score, "total": 100, "checks": checks}))
