"""Reference solution for py-clean (DuckDB). Usage: python solve.py [workspace]  (default: cwd)."""
import sys
from pathlib import Path

import duckdb

ws = Path(sys.argv[1] if len(sys.argv) > 1 else ".")
data, out = ws / "data", ws / "out"
out.mkdir(exist_ok=True)
con = duckdb.connect()
for t in ("customers", "subscriptions", "payments", "tickets"):
    con.execute(f"create view raw_{t} as select * from read_parquet('{data}/{t}.parquet')")

DATE = "coalesce(try_strptime(trim({c}), '%Y-%m-%d'), try_strptime(trim({c}), '%d/%m/%Y'), try_strptime(trim({c}), '%b %d, %Y'))::date"
TS = ("coalesce(try_strptime({c}, '%Y-%m-%dT%H:%M:%SZ'), try_strptime({c}, '%Y-%m-%d %H:%M:%S'), "
      "try_strptime({c}, '%d/%m/%Y %H:%M'))")
COUNTRY = """case lower(trim(country))
  when 'gb' then 'GB' when 'uk' then 'GB' when 'united kingdom' then 'GB' when 'great britain' then 'GB'
  when 'de' then 'DE' when 'germany' then 'DE' when 'deutschland' then 'DE'
  when 'fr' then 'FR' when 'france' then 'FR'
  when 'us' then 'US' when 'usa' then 'US' when 'united states' then 'US'
  when 'es' then 'ES' when 'spain' then 'ES' when 'espana' then 'ES'
  when 'nl' then 'NL' when 'netherlands' then 'NL' when 'the netherlands' then 'NL'
  when 'it' then 'IT' when 'italy' then 'IT' when 'italia' then 'IT'
  when 'pl' then 'PL' when 'poland' then 'PL' when 'polska' then 'PL'
  when 'se' then 'SE' when 'sweden' then 'SE' when 'sverige' then 'SE'
  when 'ie' then 'IE' when 'ireland' then 'IE' when 'eire' then 'IE' end"""

con.execute(f"""
create table clean_customers as
select customer_id, any_value(name) as name, lower(trim(any_value(email))) as email,
       any_value({DATE.format(c='signup_date')}) as signup_date,
       any_value({COUNTRY}) as country,
       any_value(case lower(trim(segment)) when 'smb' then 'SMB' when 'mid' then 'Mid' when 'enterprise' then 'Enterprise' end) as segment,
       any_value(replace(replace(lower(trim(channel)), ' ', '_'), '-', '_')) as channel
from raw_customers group by customer_id order by customer_id
""")
con.execute(f"""
create table subs as
select customer_id, {DATE.format(c='start_date')} as start_date, {DATE.format(c='end_date')} as end_date
from raw_subscriptions
""")
con.execute(f"""
create table pay as
select distinct on (payment_id) payment_id, customer_id, {TS.format(c='paid_at')} as paid_at,
       amount, lower(trim(status)) as status
from raw_payments where customer_id in (select customer_id from clean_customers)
""")
con.execute(f"""
create table tix as
select customer_id, try_strptime(opened_at, '%Y-%m-%dT%H:%M:%S')::date as opened
from raw_tickets
""")
con.execute("copy clean_customers to '%s' (format parquet)" % (out / "clean_customers.parquet"))

con.execute("""
create table months as
select strftime(m, '%Y-%m') as month, m::date as first_day, (m + interval 1 month - interval 1 day)::date as last_day
from generate_series(date '2023-01-01', date '2025-12-01', interval 1 month) t(m)
""")
con.execute("""
copy (
  select m.month,
    round(coalesce((select sum(amount) from pay where status='paid' and strftime(paid_at,'%Y-%m')=m.month),0),2) as paid_revenue_eur,
    round(coalesce((select sum(amount) from pay where status='refunded' and strftime(paid_at,'%Y-%m')=m.month),0),2) as refunds_eur,
    round(coalesce((select sum(amount) from pay where status='paid' and strftime(paid_at,'%Y-%m')=m.month),0)
        - coalesce((select sum(amount) from pay where status='refunded' and strftime(paid_at,'%Y-%m')=m.month),0),2) as net_eur,
    (select count(*) from subs s where s.start_date <= m.last_day and (s.end_date is null or s.end_date > m.last_day)) as active_customers,
    (select count(*) from clean_customers c where strftime(c.signup_date,'%Y-%m')=m.month) as new_customers
  from months m order by m.month
) to '{OUT}' (format csv, header)
""".replace("{OUT}", str(out / "monthly_revenue.csv")))

con.execute("""
copy (
  select c.customer_id, strftime(c.signup_date,'%Y-%m') as signup_month, c.country, c.segment, c.channel,
    datediff('day', c.signup_date, coalesce(case when s.end_date <= date '2025-12-31' then s.end_date end, date '2025-12-31'))::int as tenure_days,
    round(coalesce((select sum(amount) from pay p where p.customer_id=c.customer_id and p.status='paid'),0),2) as total_paid,
    (select count(*) from pay p where p.customer_id=c.customer_id and p.status='paid')::int as n_payments,
    (select count(*) from tix t where t.customer_id=c.customer_id)::int as n_tickets,
    (select count(*) from tix t where t.customer_id=c.customer_id and datediff('day', c.signup_date, t.opened) <= 60)::int as tickets_first_60d,
    (s.end_date is not null and s.end_date <= date '2025-12-31') as churned
  from clean_customers c join subs s using (customer_id) order by c.customer_id
) to '{OUT}' (format parquet)
""".replace("{OUT}", str(out / "customer_360.parquet")))

q = lambda sql: con.execute(sql).fetchone()[0]
n_dup_c = q("select count(*) from raw_customers") - q("select count(*) from clean_customers")
n_dup_p = q("select count(*) from raw_payments") - q("select count(distinct payment_id) from raw_payments")
n_orph = q("select count(*) from (select distinct payment_id, customer_id from raw_payments) where customer_id not in (select customer_id from clean_customers)")
(out / "data_quality.md").write_text(f"""# Data quality report

- Duplicate customer rows (same customer_id): {n_dup_c} removed.
- Duplicate payment rows (same payment_id): {n_dup_p} removed.
- Orphan payment rows (customer_id not in customers): {n_orph} excluded.
- signup_date came in three text formats (ISO, day-first dd/mm/yyyy, 'Mon dd, yyyy'); parsed to DATE.
- paid_at came in three formats (ISO with T/Z, space separated, day-first dd/mm/yyyy HH:MM); parsed as UTC.
- country had many spellings (UK, United Kingdom, GB, Deutschland, USA, ...); mapped to ISO-2.
- segment and channel had case and whitespace variants and separators (Paid Social, paid-social); normalised.
- email had mixed case and padding whitespace; trimmed and lower-cased.
- payment status had mixed case (PAID, Paid); compared case-insensitively.
- monthly_price strings (euro sign or EUR suffix) were not needed for the outputs.
""")
print("done", out)
