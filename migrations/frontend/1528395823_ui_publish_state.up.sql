BEGIN;

CREATE TYPE
    batch_changes_changeset_desired_publication_state
AS ENUM (
    'unpublished',
    'draft',
    'published',
    'ui-unpublished',
    'ui-draft',
    'ui-published'
);

-- We need to add a desired publication state column, but we also need to
-- calculate it as part of the migration.
ALTER TABLE
    changesets
ADD COLUMN IF NOT EXISTS
    desired_publication_state batch_changes_changeset_desired_publication_state NOT NULL DEFAULT 'unpublished';

-- Calculate the desired publication state for created changesets (ie changesets
-- with a spec).
UPDATE
    changesets
SET
    desired_publication_state = (
        SELECT
            CASE
                WHEN changeset_specs.spec->>'published' = 'draft'
                    THEN 'draft'
                WHEN changeset_specs.spec->>'published' = 'true'
                    THEN 'published'
                ELSE 'unpublished'
            END
        FROM
            changeset_specs
        WHERE
            changeset_specs.id = changesets.current_spec_id
    )::batch_changes_changeset_desired_publication_state
WHERE
    current_spec_id IS NOT NULL;

-- Calculate the desired publication state for imported changesets, which should
-- always match the actual publication state.
UPDATE
    changesets
SET
    desired_publication_state = LOWER(publication_state)::batch_changes_changeset_desired_publication_state
WHERE
    current_spec_id IS NULL;

COMMIT;
