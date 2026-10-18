-- Keep accepted owner receipts intact; rolling back while they exist is refused.
DO $$ BEGIN
IF EXISTS (SELECT 1 FROM issue_owner_directive WHERE directive_type='accept_result') THEN
 RAISE EXCEPTION 'Accepted result receipts prevent rollback';
END IF;
ALTER TABLE issue_owner_directive DROP CONSTRAINT issue_owner_directive_directive_type_check;
ALTER TABLE issue_owner_directive ADD CONSTRAINT issue_owner_directive_directive_type_check CHECK (directive_type IN ('correct','request_changes','partial_acceptance','change_scope','cancel'));
END $$;
