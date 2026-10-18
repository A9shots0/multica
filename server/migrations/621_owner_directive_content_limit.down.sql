-- Refuse rollback if longer directives exist; never truncate owner evidence.
ALTER TABLE issue_owner_directive
    DROP CONSTRAINT issue_owner_directive_content_check,
    ADD CONSTRAINT issue_owner_directive_content_check
        CHECK (length(btrim(content)) BETWEEN 1 AND 4000);
