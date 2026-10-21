# Task: understand the business, then predict churn

You are a data scientist at a subscription software company. Four parquet exports of the business are in
`./data/` (the data is messy: duplicated rows, orphan records, mixed date and country formats; clean it
yourself and say what you did). Work by writing and running scripts (no notebook). `python` already has
duckdb, pandas, pyarrow, numpy, scikit-learn and matplotlib installed; do not install anything.

The data was exported at the **cutoff date T = 2025-12-31**. The business started in January 2023.

## Inputs (`./data/*.parquet`)

- `customers.parquet`: `customer_id`, `name`, `email`, `signup_date` (text, mixed formats: `2024-03-05`,
  `05/03/2024` which is **day first**, `Mar 05, 2024`), `country`, `segment`, `channel`.
- `subscriptions.parquet`: `customer_id`, `plan`, `start_date`, `end_date` (null = still active at T),
  `monthly_price` (text like `€29.00` or `29.00 EUR`), `churn_reason`.
- `payments.parquet`: `payment_id`, `customer_id`, `paid_at` (mixed text timestamp formats, UTC), `amount`
  (EUR), `status` (`paid`, `failed`, `refunded`; inconsistent letter case; `refunded` rows are refund
  transactions). Duplicated rows and payments for unknown customers exist.
- `tickets.parquet`: `ticket_id`, `customer_id`, `opened_at`, `category`, `severity`, `resolution_hours`.

## What to deliver (create `./out/`)

1. **Explore and explain.** Find what drives this business: seasonality, growth trend, what makes customers
   churn (acquisition channel, support experience, segment, lifecycle stage, pricing events, anything else),
   and the segment economics (revenue per customer vs churn).
2. **Predict churn.** For every customer who is **active at T** (subscription `end_date` is null), predict the
   probability that they **cancel within the next 90 days after T** (between 2026-01-01 and 2026-03-31).
   Build features, try more than one scikit-learn model, validate honestly, and include a sensitivity
   analysis (how much do the results depend on your choices, features or inputs?).
   Write `out/predictions.csv` with columns `customer_id,churn_probability` (a probability between 0 and 1)
   with exactly one row for every customer active at T. It is scored against what really happened after T,
   which you cannot see.
3. **Present.** Write `out/report.md`: your key findings, each justified with numbers and evidence from the
   data (and the chart that shows it), how you framed and validated the model and why you believe it is not
   leaking information, the sensitivity analysis, recommendations, and limitations. At least three charts
   as PNG files in `out/charts/` (for example seasonality, churn drivers, model or sensitivity).

## Scoring

Hidden outcomes drive most of the score (predictions); the report is judged on whether it finds the real
patterns in the data, backs them with correct numbers, and is honest about limitations (claims the data does
not support count against you). Stop when `predictions.csv`, `report.md` and the charts exist. Reply with a
one-line summary.
