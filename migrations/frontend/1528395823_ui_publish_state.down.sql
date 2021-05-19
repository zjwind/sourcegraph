BEGIN;

-- We don't want to allow the downgrade if there are missing published fields in
-- the spec, since that will cause errors in Sourcegraph. We'll temporarily
-- define a function to check this and call it: if the function raises an
-- exception, then the entire transaction will be rolled back, including the
-- function creation itself. Otherwise, we can drop the function again before we
-- commit.
CREATE FUNCTION
    check_for_ui_controlled_publication_states()
    RETURNS void
    LANGUAGE plpgsql
    AS $$
    BEGIN
        -- Rather than going right into changeset_specs, we can more simply
        -- query the desired publication state on the changeset: there may be
        -- detached changeset specs that have missing published fields, but
        -- those shouldn't be fatal errors, and this avoids us needing to join
        -- and dig through the spec JSON.
        IF EXISTS(
            SELECT
                id
            FROM
                changesets
            WHERE
                desired_publication_state::text LIKE 'ui-%'
        ) THEN
            RAISE EXCEPTION
                'cannot downgrade if changesets have been created with a UI controlled published field';
        END IF;
    END;
$$;

SELECT
    check_for_ui_controlled_publication_states();

DROP FUNCTION
    check_for_ui_controlled_publication_states;

ALTER TABLE
    changesets
DROP COLUMN IF EXISTS
    desired_publication_state;

DROP TYPE IF EXISTS
    batch_changes_changeset_desired_publication_state;

COMMIT;
