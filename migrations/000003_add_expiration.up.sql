ALTER TABLE urls
ADD COLUMN expires_at TIMESTAMPTZ;

UPDATE urls
SET expires_at = now() + INTERVAL '30 days';

ALTER TABLE urls
ALTER COLUMN expires_at SET NOT NULL;
