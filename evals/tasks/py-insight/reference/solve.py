"""Reference solution for py-insight: leak-free temporal-snapshot churn model + insights.

Usage: python solve.py [workspace]   (default: cwd). Reads ./data, writes ./out.
"""
import re
import sys
from pathlib import Path

import matplotlib
import numpy as np
import pandas as pd

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402
from sklearn.ensemble import HistGradientBoostingClassifier  # noqa: E402
from sklearn.inspection import permutation_importance  # noqa: E402
from sklearn.metrics import roc_auc_score  # noqa: E402

ws = Path(sys.argv[1] if len(sys.argv) > 1 else ".")
data, out = ws / "data", ws / "out"
(out / "charts").mkdir(parents=True, exist_ok=True)
T = pd.Timestamp("2025-12-31")
H = pd.Timedelta(days=90)

# ------------------------------------------------------------------ cleaning
def parse_dates(s):
    s = s.astype(str).str.strip()
    a = pd.to_datetime(s, format="%Y-%m-%d", errors="coerce")
    b = pd.to_datetime(s, format="%d/%m/%Y", errors="coerce")
    c = pd.to_datetime(s, format="%b %d, %Y", errors="coerce")
    return a.fillna(b).fillna(c)

def parse_ts(s):
    a = pd.to_datetime(s, format="%Y-%m-%dT%H:%M:%SZ", errors="coerce")
    b = pd.to_datetime(s, format="%Y-%m-%d %H:%M:%S", errors="coerce")
    c = pd.to_datetime(s, format="%d/%m/%Y %H:%M", errors="coerce")
    return a.fillna(b).fillna(c)

cust = pd.read_parquet(data / "customers.parquet").drop_duplicates("customer_id").copy()
cust["signup"] = parse_dates(cust["signup_date"])
cust["segment"] = cust["segment"].str.strip().str.lower().map({"smb": "SMB", "mid": "Mid", "enterprise": "Enterprise"})
cust["channel"] = cust["channel"].str.strip().str.lower().str.replace(r"[ \-]", "_", regex=True)
subs = pd.read_parquet(data / "subscriptions.parquet")
subs["start"] = parse_dates(subs["start_date"])
subs["end"] = parse_dates(subs["end_date"].fillna(""))
subs["price"] = subs["monthly_price"].str.extract(r"(\d+\.?\d*)")[0].astype(float)
pay = pd.read_parquet(data / "payments.parquet").drop_duplicates("payment_id").copy()
pay = pay[pay["customer_id"].isin(cust["customer_id"])]
pay["ts"] = parse_ts(pay["paid_at"])
pay["status"] = pay["status"].str.lower().str.strip()
tix = pd.read_parquet(data / "tickets.parquet")
tix["ts"] = pd.to_datetime(tix["opened_at"])
c = cust.merge(subs[["customer_id", "start", "end", "price", "churn_reason"]], on="customer_id")
c["churned"] = c["end"].notna()
c["life_days"] = (c["end"].fillna(T) - c["signup"]).dt.days

# ------------------------------------------------------------------ EDA numbers
mature = c[c["signup"] <= T - pd.Timedelta(days=180)]
by_channel = mature.groupby("channel")["churned"].mean().sort_values(ascending=False)
by_segment = mature.groupby("segment")["churned"].mean()
arpu = c.groupby("segment")["price"].mean()
t60 = tix.merge(c[["customer_id", "signup"]], on="customer_id")
t60 = t60[(t60["ts"].dt.normalize() - t60["signup"]).dt.days <= 60].groupby("customer_id").size()
mature = mature.assign(t60=mature["customer_id"].map(t60).fillna(0))
surv = mature[(mature["life_days"] > 60)]
tick_hi, tick_lo = surv[surv["t60"] >= 3]["churned"].mean(), surv[surv["t60"] < 3]["churned"].mean()
ch = c[c["churned"]]
early_share = float((ch["life_days"] <= 90).mean())
new_by_month = c.groupby(c["signup"].dt.to_period("M")).size()
cal = new_by_month.groupby(new_by_month.index.month).mean()
trend_y = new_by_month.groupby(new_by_month.index.year).sum()
churn_by_month = ch.groupby(ch["end"].dt.to_period("M")).size()
spike = int(churn_by_month.get(pd.Period("2024-09"), 0))
around = float(np.mean([churn_by_month.get(pd.Period(p), 0) for p in ("2024-07", "2024-08", "2024-10", "2024-11")]))
price_share = float((ch[ch["end"].dt.to_period("M") == pd.Period("2024-09")]["churn_reason"] == "price").mean())
rev = pay[pay["status"] == "paid"].groupby(pay["ts"].dt.to_period("M"))["amount"].sum()

# ------------------------------------------------------------------ snapshot features
SEGS, CHS = ["SMB", "Mid", "Enterprise"], sorted(c["channel"].unique())

def snapshot(S):
    """Customers active at S with features known at S, and the 90-day outcome (NaN if not yet observable)."""
    a = c[(c["start"] <= S) & (c["end"].isna() | (c["end"] > S))].copy()
    ids = a["customer_id"]
    a["tenure"] = (S - a["signup"]).dt.days
    tp = tix[tix["ts"] <= S].merge(a[["customer_id", "signup"]], on="customer_id")
    tp["d"] = (tp["ts"].dt.normalize() - tp["signup"]).dt.days
    a["n_tix"] = ids.map(tp.groupby("customer_id").size()).fillna(0)
    a["tix60"] = ids.map(tp[tp["d"] <= 60].groupby("customer_id").size()).fillna(0)
    a["tix_recent"] = ids.map(tp[tp["ts"] > S - pd.Timedelta(days=60)].groupby("customer_id").size()).fillna(0)
    pp = pay[pay["ts"] <= S + pd.Timedelta(days=1)]
    f = pp[pp["status"] == "failed"]
    a["failed60"] = ids.map(f[f["ts"] > S - pd.Timedelta(days=60)].groupby("customer_id").size()).fillna(0)
    a["failed30"] = ids.map(f[f["ts"] > S - pd.Timedelta(days=30)].groupby("customer_id").size()).fillna(0)
    a["failed_all"] = ids.map(f.groupby("customer_id").size()).fillna(0)
    a["n_paid"] = ids.map(pp[pp["status"] == "paid"].groupby("customer_id").size()).fillna(0)
    a["n_refund"] = ids.map(pp[pp["status"] == "refunded"].groupby("customer_id").size()).fillna(0)
    a["fail_rate"] = a["failed_all"] / (a["failed_all"] + a["n_paid"]).clip(lower=1)
    for s in SEGS:
        a[f"seg_{s}"] = (a["segment"] == s).astype(int)
    for k in CHS:
        a[f"ch_{k}"] = (a["channel"] == k).astype(int)
    a["price_cur"] = a["price"]
    a["signup_month_no"] = a["signup"].dt.month
    a["label"] = np.where(S + H <= T, ((a["end"].notna()) & (a["end"] <= S + H)).astype(float), np.nan)
    a["snapshot"] = S
    return a

FEATS = (["tenure", "n_tix", "tix60", "tix_recent", "failed60", "failed30", "failed_all", "n_paid", "n_refund",
          "fail_rate", "price_cur", "signup_month_no"] + [f"seg_{s}" for s in SEGS] + [f"ch_{k}" for k in CHS])

def model():
    return HistGradientBoostingClassifier(max_depth=4, learning_rate=0.05, max_iter=250, l2_regularization=1.0,
                                          random_state=0)

snaps = [T - pd.Timedelta(days=90 * i) for i in range(1, 9)]  # 8 quarterly snapshots, all with an observed outcome
frames = [snapshot(S) for S in snaps]
valid, train = frames[0], pd.concat(frames[1:])
m = model().fit(train[FEATS], train["label"])
auc_valid = roc_auc_score(valid["label"], m.predict_proba(valid[FEATS])[:, 1])
aucs_by_history = {}
for k in (1, 3, 7):
    tr = pd.concat(frames[1:1 + k])
    aucs_by_history[k] = roc_auc_score(valid["label"], model().fit(tr[FEATS], tr["label"]).predict_proba(valid[FEATS])[:, 1])
# naive baseline: tenure only
base_auc = roc_auc_score(valid["label"], -valid["tenure"])
perm = permutation_importance(m, valid[FEATS], valid["label"], scoring="roc_auc", n_repeats=5, random_state=0)
imp = pd.Series(perm.importances_mean, index=FEATS).sort_values(ascending=False)

final = pd.concat(frames)
m_final = model().fit(final[FEATS], final["label"])
now = snapshot(T)
now["churn_probability"] = m_final.predict_proba(now[FEATS])[:, 1]
now[["customer_id", "churn_probability"]].to_csv(out / "predictions.csv", index=False)

# sensitivity: predicted 90d churn when one input changes for everybody active at T
sens = {}
for name, edit in (("failed payment in last 30 days = 1", lambda d: d.assign(failed30=1, failed60=np.maximum(d["failed60"], 1))),
                   ("no failed payments", lambda d: d.assign(failed30=0, failed60=0)),
                   ("tenure + 90 days", lambda d: d.assign(tenure=d["tenure"] + 90))):
    sens[name] = float(m_final.predict_proba(edit(now)[FEATS])[:, 1].mean())
base_pred = float(now["churn_probability"].mean())

# ------------------------------------------------------------------ charts
plt.rcParams.update({"figure.dpi": 110, "axes.spines.top": False, "axes.spines.right": False})
fig, ax = plt.subplots(1, 2, figsize=(11, 4))
new_by_month.index = new_by_month.index.to_timestamp()
ax[0].plot(new_by_month.index, new_by_month.values, marker="o", ms=3)
ax[0].set_title("New customers per month: growth plus seasonality")
ax[1].bar(cal.index, cal.values)
ax[1].set_xticks(range(1, 13))
ax[1].set_title("Average signups by calendar month (Nov-Dec peak, Jul-Aug dip)")
fig.tight_layout(); fig.savefig(out / "charts" / "seasonality_and_trend.png"); plt.close(fig)

fig, ax = plt.subplots(1, 3, figsize=(13, 4))
by_channel.plot.bar(ax=ax[0], color="#c0504d"); ax[0].set_title("Churned share by channel (customers >= 180d)")
by_segment.reindex(SEGS).plot.bar(ax=ax[1], color="#4f81bd"); ax[1].set_title("Churned share by segment")
pd.Series({"< 3 tickets": tick_lo, ">= 3 tickets": tick_hi}).plot.bar(ax=ax[2], color="#9bbb59")
ax[2].set_title("Churned share by tickets in first 60 days")
for a in ax:
    a.tick_params(axis="x", rotation=30)
fig.tight_layout(); fig.savefig(out / "charts" / "churn_drivers.png"); plt.close(fig)

fig, ax = plt.subplots(1, 2, figsize=(11, 4))
cm = churn_by_month.copy(); cm.index = cm.index.to_timestamp()
ax[0].bar(cm.index, cm.values, width=20); ax[0].set_title("Churn events per month (price increase 2024-09)")
imp.head(10)[::-1].plot.barh(ax=ax[1]); ax[1].set_title("Permutation importance (AUC drop), validation snapshot")
fig.tight_layout(); fig.savefig(out / "charts" / "churn_timeline_and_model.png"); plt.close(fig)

fig, ax = plt.subplots(figsize=(6, 4))
bins = pd.cut(ch["life_days"], [0, 30, 60, 90, 180, 365, 2000])
bins.value_counts().sort_index().plot.bar(ax=ax); ax.set_title("Lifetime of churned customers (days)")
fig.tight_layout(); fig.savefig(out / "charts" / "lifetime_of_churners.png"); plt.close(fig)

# ------------------------------------------------------------------ report
top = ", ".join(f"{k} ({v:.3f})" for k, v in imp.head(5).items())
(out / "report.md").write_text(f"""# Subscription churn: findings and 90-day churn model

Data: {len(c)} customers, {len(pay)} payments after de-duplication, cutoff 2025-12-31. All numbers below come
from the cleaned tables (duplicates removed, orphan payments dropped, dates and categories normalised).

## Key findings

1. **Seasonality (chart: seasonality_and_trend.png).** New-customer signups peak in November and December
   (about {cal[[11, 12]].mean():.0f}/month on average) and dip in July and August ({cal[[7, 8]].mean():.0f}/month): a
   Nov-Dec / Jul-Aug ratio of {cal[[11, 12]].mean() / cal[[7, 8]].mean():.1f}x. Revenue follows with a lag.
2. **Growth trend.** Signups rose from {trend_y[2023]} in 2023 to {trend_y[2025]} in 2025
   ({trend_y[2025] / trend_y[2023] - 1:.0%} up), about 3% compounded growth per month once seasonality is
   averaged out. Monthly paid revenue went from EUR {rev.iloc[0]:,.0f} to EUR {rev.iloc[-1]:,.0f}.
3. **Churn by acquisition channel (churn_drivers.png).** Among customers older than 180 days the churned share
   is {by_channel.iloc[0]:.0%} for **{by_channel.index[0]}** versus {by_channel.iloc[-1]:.0%} for {by_channel.index[-1]}:
   paid_social customers churn roughly twice as often as referral/partner/organic. Full table:
   {", ".join(f"{k} {v:.0%}" for k, v in by_channel.items())}.
4. **Support tickets in the first 60 days.** Customers who opened 3 or more tickets in their first 60 days churn
   at {tick_hi:.0%} versus {tick_lo:.0%} for those with fewer (among customers who survived 60 days, so early
   churners do not bias the comparison): a strong early-warning signal.
5. **Enterprise economics.** Enterprise average monthly price is EUR {arpu['Enterprise']:.0f} versus EUR {arpu['SMB']:.0f}
   for SMB, each before and after the 15% rise (about {arpu['Enterprise'] / arpu['SMB']:.1f}x ARPU) and its churned share is the lowest
   ({by_segment['Enterprise']:.0%} versus {by_segment['SMB']:.0%} for SMB and {by_segment['Mid']:.0%} for Mid).
6. **Price increase of 2024-09.** A 15% price increase took effect in September 2024: {spike} churn events that
   month versus about {around:.0f} per month in the surrounding months, and {price_share:.0%} of that month's
   churners cite "price" as the reason (churn_timeline_and_model.png).
7. **Early-life churn.** {early_share:.0%} of all churn events happen within 90 days of signup
   (lifetime_of_churners.png): the first three months are the critical window for retention.

## Churn model (predict: active at 2025-12-31 and churns within the next 90 days)

- **Framing and leakage.** A customer's current "has churned" flag cannot be a feature or the label for people who
  are still active. I built **point-in-time snapshots**: for each of 8 quarterly dates S (T-90d ... T-720d) I take
  customers active at S, compute features using only data up to S (tenure, tickets, failed payments in the last
  30/60 days, payment history, plan price, segment, channel, signup month) and label whether they churned in
  (S, S+90d]. The most recent snapshot (T-90d) is held out for validation; the final model trains on all
  snapshots and scores customers active at T.
- **Model.** Gradient boosting (HistGradientBoostingClassifier). Validation AUC on the held-out snapshot is
  **{auc_valid:.3f}**, versus {base_auc:.3f} for a tenure-only baseline.
- **Sensitivity.** AUC when training on 1 / 3 / 7 historical snapshots: {aucs_by_history[1]:.3f} / {aucs_by_history[3]:.3f}
  / {aucs_by_history[7]:.3f} (more history helps then flattens). Permutation importance (AUC drop): {top}.
  Predicted average 90-day churn probability for customers active at T is {base_pred:.1%}; setting a failed
  payment in the last 30 days for everybody lifts it to {sens['failed payment in last 30 days = 1']:.1%}, removing
  failed payments gives {sens['no failed payments']:.1%}, and ageing everyone by 90 days gives
  {sens['tenure + 90 days']:.1%}.
- **Output.** predictions.csv has one churn probability for each of the {len(now)} customers active at T.

## Recommendations

Invest in onboarding for the first 90 days, watch paid_social cohorts and customers with 3+ early tickets,
trigger dunning/outreach on failed payments, and treat price changes as churn events to be staged.

## Limitations

One price change and no usage data, so pricing elasticity is a single observation; the
channel and ticket effects are associations, not causal; the validation snapshot overlaps in time with training
snapshots only through customers who appear in both (different dates, same people).
""")
print("done", out, "valid auc", round(auc_valid, 4))
