# TIMESTAMP vs TIMESTAMPTZ in PostgreSQL

## The Short Answer

**Always use `TIMESTAMPTZ`** in web applications. There's almost no reason to use plain `TIMESTAMP`.

## What's the Difference?

| | `TIMESTAMP` (without time zone) | `TIMESTAMPTZ` (with time zone) |
|---|---|---|
| Stores | The literal date/time you gave it | The date/time converted to UTC |
| On retrieval | Returns exactly what was stored | Converts from UTC to your session's time zone |
| Time zone aware? | No | Yes |

## Why It Matters

```sql
-- Your Go server writes in UTC. Someone reads from psql in US/Eastern.

-- TIMESTAMP: stores the raw value, no context
INSERT INTO events (created_at) VALUES ('2026-04-01 14:00:00');
SELECT created_at FROM events;
-- Always returns: 2026-04-01 14:00:00 (is that UTC? Eastern? Who knows?)

-- TIMESTAMPTZ: stores as UTC, translates on read
INSERT INTO events (created_at) VALUES ('2026-04-01 14:00:00');
SELECT created_at FROM events;
-- UTC session:     2026-04-01 14:00:00+00
-- Eastern session: 2026-04-01 10:00:00-04
```

With `TIMESTAMP`, the value `14:00:00` has no context about which time zone that 2pm belongs to. If your Go server writes it in UTC but someone reads it from a different timezone, they'll misinterpret the time.

With `TIMESTAMPTZ`, Postgres always knows the real moment in time and translates correctly for whoever is reading.

## In DocGoat

All our migrations use `TIMESTAMPTZ`:

```sql
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

`now()` returns the current time in the session's time zone, and Postgres converts it to UTC for storage. On read, it converts back. No ambiguity.

## When Would You Use Plain TIMESTAMP?

Almost never in a web app. The rare exception is when time zone truly doesn't matter — like "this event starts at 9am" where 9am means 9am local time in every time zone. That's unusual for backend services.

## Go Tip

Go's `time.Time` is always time-zone-aware. When you use `TIMESTAMPTZ` + a Postgres driver like `pgx`, the mapping is clean — Go `time.Time` ↔ Postgres `TIMESTAMPTZ`, no conversion bugs. With plain `TIMESTAMP`, the driver has to guess, and that's where subtle bugs creep in.
