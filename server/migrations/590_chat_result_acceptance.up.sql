DO $$ BEGIN
ALTER TABLE issue_owner_directive DROP CONSTRAINT issue_owner_directive_directive_type_check;
ALTER TABLE issue_owner_directive ADD CONSTRAINT issue_owner_directive_directive_type_check CHECK (directive_type IN ('correct','request_changes','partial_acceptance','change_scope','cancel','accept_result'));
END $$;
