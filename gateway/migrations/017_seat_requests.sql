-- Seat requests from the public landing page (2026-09-10).
--
-- The page's call to action was a mailto:, which does nothing in many mobile
-- browsers and leaves no record. A creator with millions of followers is
-- about to link the page from her YouTube bio, so the request has to land
-- somewhere durable: this table, written by the public POST /api/seat, read
-- by admins through GET /api/seats.
--
-- email:            the only required field; lower-cased, trimmed.
-- about:            what they make / coach (free text, capped).
-- videos_per_week:  their own estimate, as typed ("1", "2-3", "5+").
-- mac:              "yes" | "no" | "unsure" — Apple Silicon with 16 GB is the
--                   real filter, so ask before promising anything.
-- ref:              who sent them, from the link's ?ref= (e.g. "sara"), so a
--                   referral deal can be attributed. Letters, digits, - _ only.
-- user_agent:       for "was this mobile?" answers later, nothing more.
-- created_at:       epoch seconds.
CREATE TABLE IF NOT EXISTS seat_requests (
    id              TEXT PRIMARY KEY,
    email           TEXT NOT NULL,
    about           TEXT NOT NULL DEFAULT '',
    videos_per_week TEXT NOT NULL DEFAULT '',
    mac             TEXT NOT NULL DEFAULT '',
    ref             TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_seat_requests_created ON seat_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_seat_requests_email ON seat_requests(email);
