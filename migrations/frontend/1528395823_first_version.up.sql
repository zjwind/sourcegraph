BEGIN;

ALTER TABLE versions ADD COLUMN first_version_major int;
ALTER TABLE versions ADD COLUMN first_version_minor int;

UPDATE versions SET
    first_version_major = split_part(version, '.', 1)::int,
    first_version_minor = split_part(version, '.', 2)::int;

ALTER TABLE versions ALTER COLUMN first_version_major SET NOT NULL;
ALTER TABLE versions ALTER COLUMN first_version_minor SET NOT NULL;

COMMIT;
