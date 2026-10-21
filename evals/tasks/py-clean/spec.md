# Task: clean, join and aggregate four dirty tables

You are a data engineer at a subscription software company. Four parquet exports of the business are in
`./data/`. They are messy. Produce clean, joined, aggregated outputs by writing and running scripts
(no notebook). `python` already has duckdb, pandas, pyarrow, numpy, scikit-learn and matplotlib installed;
use whichever you prefer (DuckDB SQL is a good fit). Do not install anything.

The data was exported at the **cutoff date T = 2025-12-31** (nothing after it is visible). The business
started in January 2023.

## Inputs (`./data/*.parquet`)

- `customers.parquet`: `customer_id`, `name`, `email`, `signup_date` (text, mixed formats: `2024-03-05`,
  `05/03/2024` which is **day first**, `Mar 05, 2024`), `country` (many spellings of the same country),
  `segment`, `channel`.
- `subscriptions.parquet`: `customer_id`, `plan`, `start_date`, `end_date` (null = still active at T),
  `monthly_price` (text like `€29.00` or `29.00 EUR`), `churn_reason`.
- `payments.parquet`: `payment_id`, `customer_id`, `paid_at` (text, mixed formats: ISO with `T`/`Z`,
  `2024-03-05 09:12:33`, and day-first `05/03/2024 09:12`; all UTC), `amount` (EUR), `status`
  (`paid`, `failed` or `refunded`, with inconsistent letter case). A `refunded` row is a refund transaction:
  money returned, `amount` is positive.
- `tickets.parquet`: `ticket_id`, `customer_id`, `opened_at`, `category`, `severity`, `resolution_hours`.

Explore the data first: there are duplicated rows, spelling variants, orphan records and whitespace.
A customer's `signup_date` equals their subscription `start_date`.

## Outputs (create `./out/`)

### 1. `out/clean_customers.parquet`

One row per customer (`customer_id` unique; duplicate rows removed), columns:
`customer_id`, `name`, `email` (trimmed and lower-case), `signup_date` (a real DATE column),
`country` (ISO 3166-1 alpha-2, upper case: `GB DE FR US ES NL IT PL SE IE`),
`segment` (exactly `SMB`, `Mid` or `Enterprise`), `channel` (lower-case snake_case: `organic`,
`paid_search`, `paid_social`, `referral`, `partner`).

### 2. `out/monthly_revenue.csv`

One row per calendar month from `2023-01` to `2025-12` (36 rows, ascending), columns:
`month` (`YYYY-MM`), `paid_revenue_eur`, `refunds_eur`, `net_eur`, `active_customers`, `new_customers`.

Rules:
- Remove duplicate payment rows (same `payment_id`) before anything else.
- Ignore payments whose `customer_id` is not in the customers table (orphans).
- Compare `status` case-insensitively. `paid_revenue_eur` = sum of `amount` where status is `paid`;
  `refunds_eur` = sum of `amount` where status is `refunded` (a positive number); `failed` payments are
  ignored; `net_eur` = `paid_revenue_eur` - `refunds_eur`. Round money to 2 decimals.
- A payment belongs to the month of its `paid_at` (parse it as UTC, no timezone shifting).
- `active_customers` = customers whose `start_date` is on or before the last day of the month and whose
  `end_date` is null or **after** the last day of the month (active at month end).
- `new_customers` = customers whose `signup_date` falls in that month.

### 3. `out/customer_360.parquet`

One row per customer (same ids as `clean_customers`), columns exactly:
`customer_id`, `signup_month` (`YYYY-MM`), `country`, `segment`, `channel`,
`tenure_days` (integer: days from `signup_date` to `end_date` if the customer churned, else to T),
`total_paid` (sum of `amount` over that customer's `paid` payments, after removing duplicates; 2 decimals),
`n_payments` (count of `paid` payments), `n_tickets` (all tickets), `tickets_first_60d` (tickets opened
within 60 days after signup: the whole-day difference between the ticket date and `signup_date` is at most
60), `churned` (boolean: `end_date` is not null and on or before T).

### 4. `out/data_quality.md`

A short report listing every data-quality problem you found and fixed, with **counts** (at least:
duplicated customer rows, duplicated payment rows, orphan payment rows, plus the formatting problems).

## Scoring

Hidden checks recompute every output from the true clean data: whole-column matches (money to within
0.01, counts exact). Partial matches earn partial credit, near misses little. Work only inside this directory,
run your scripts to produce the files, and stop when all four outputs exist and are correct. Reply with a
one-line summary.
