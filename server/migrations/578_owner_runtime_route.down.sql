-- Do not remove admitted authority while an ordinary route still exists.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM issue_workflow_plan WHERE owner_runtime_route IS NOT NULL) THEN
    RAISE EXCEPTION 'owner runtime routes must be retired before removing their authority';
  END IF;
END $$;
ALTER TABLE issue_workflow_plan DROP COLUMN owner_runtime_route;
