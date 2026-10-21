# Data quality report

- Duplicate customer rows (same customer_id): 110 removed.
- Duplicate payment rows (same payment_id): 706 removed.
- Orphan payment rows (customer_id not in customers): 212 excluded.
- signup_date came in three text formats (ISO, day-first dd/mm/yyyy, 'Mon dd, yyyy'); parsed to DATE.
- paid_at came in three formats (ISO with T/Z, space separated, day-first dd/mm/yyyy HH:MM); parsed as UTC.
- country had many spellings (UK, United Kingdom, GB, Deutschland, USA, ...); mapped to ISO-2.
- segment and channel had case and whitespace variants and separators (Paid Social, paid-social); normalised.
- email had mixed case and padding whitespace; trimmed and lower-cased.
- payment status had mixed case (PAID, Paid); compared case-insensitively.
- monthly_price strings (euro sign or EUR suffix) were not needed for the outputs.
