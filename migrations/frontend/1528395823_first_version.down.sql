BEGIN;

ALTER TABLE versions DROP COLUMN first_version_major;
ALTER TABLE versions DROP COLUMN first_version_minor;

COMMIT;
