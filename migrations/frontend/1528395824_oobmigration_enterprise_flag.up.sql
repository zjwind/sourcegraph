BEGIN;

ALTER TABLE out_of_band_migrations ADD COLUMN is_enterprise boolean DEFAULT false;

-- The only non-enterprise migrations are #3 and #6.
UPDATE out_of_band_migrations SET is_enterprise = true WHERE id NOT IN (3, 6);

COMMIT;
