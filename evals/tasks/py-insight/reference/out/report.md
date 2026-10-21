# Subscription churn: findings and 90-day churn model

Data: 3679 customers, 35341 payments after de-duplication, cutoff 2025-12-31. All numbers below come
from the cleaned tables (duplicates removed, orphan payments dropped, dates and categories normalised).

## Key findings

1. **Seasonality (chart: seasonality_and_trend.png).** New-customer signups peak in November and December
   (about 175/month on average) and dip in July and August (74/month): a
   Nov-Dec / Jul-Aug ratio of 2.4x. Revenue follows with a lag.
2. **Growth trend.** Signups rose from 841 in 2023 to 1711 in 2025
   (103% up), about 3% compounded growth per month once seasonality is
   averaged out. Monthly paid revenue went from EUR 2,497 to EUR 123,197.
3. **Churn by acquisition channel (churn_drivers.png).** Among customers older than 180 days the churned share
   is 66% for **paid_social** versus 33% for referral:
   paid_social customers churn roughly twice as often as referral/partner/organic. Full table:
   paid_social 66%, paid_search 51%, organic 39%, partner 35%, referral 33%.
4. **Support tickets in the first 60 days.** Customers who opened 3 or more tickets in their first 60 days churn
   at 50% versus 24% for those with fewer (among customers who survived 60 days, so early
   churners do not bias the comparison): a strong early-warning signal.
5. **Enterprise economics.** Enterprise average monthly price is EUR 164 versus EUR 32
   for SMB, each before and after the 15% rise (about 5.2x ARPU) and its churned share is the lowest
   (23% versus 49% for SMB and 41% for Mid).
6. **Price increase of 2024-09.** A 15% price increase took effect in September 2024: 98 churn events that
   month versus about 36 per month in the surrounding months, and 77% of that month's
   churners cite "price" as the reason (churn_timeline_and_model.png).
7. **Early-life churn.** 73% of all churn events happen within 90 days of signup
   (lifetime_of_churners.png): the first three months are the critical window for retention.

## Churn model (predict: active at 2025-12-31 and churns within the next 90 days)

- **Framing and leakage.** A customer's current "has churned" flag cannot be a feature or the label for people who
  are still active. I built **point-in-time snapshots**: for each of 8 quarterly dates S (T-90d ... T-720d) I take
  customers active at S, compute features using only data up to S (tenure, tickets, failed payments in the last
  30/60 days, payment history, plan price, segment, channel, signup month) and label whether they churned in
  (S, S+90d]. The most recent snapshot (T-90d) is held out for validation; the final model trains on all
  snapshots and scores customers active at T.
- **Model.** Gradient boosting (HistGradientBoostingClassifier). Validation AUC on the held-out snapshot is
  **0.737**, versus 0.718 for a tenure-only baseline.
- **Sensitivity.** AUC when training on 1 / 3 / 7 historical snapshots: 0.706 / 0.725
  / 0.737 (more history helps then flattens). Permutation importance (AUC drop): tenure (0.051), n_paid (0.049), ch_paid_social (0.016), tix60 (0.013), failed30 (0.012).
  Predicted average 90-day churn probability for customers active at T is 6.2%; setting a failed
  payment in the last 30 days for everybody lifts it to 21.4%, removing
  failed payments gives 5.4%, and ageing everyone by 90 days gives
  4.4%.
- **Output.** predictions.csv has one churn probability for each of the 2242 customers active at T.

## Recommendations

Invest in onboarding for the first 90 days, watch paid_social cohorts and customers with 3+ early tickets,
trigger dunning/outreach on failed payments, and treat price changes as churn events to be staged.

## Limitations

One price change and no usage data, so pricing elasticity is a single observation; the
channel and ticket effects are associations, not causal; the validation snapshot overlaps in time with training
snapshots only through customers who appear in both (different dates, same people).
