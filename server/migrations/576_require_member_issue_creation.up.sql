-- Enforce the workspace's member-only issue creation rule at the insert
-- boundary. This covers root issues, sub-issues, quick-create, autopilot, and
-- any future direct writer without relying on each caller to remember a guard.
CREATE FUNCTION enforce_member_issue_creation_policy()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    workspace_settings jsonb;
BEGIN
    SELECT settings INTO workspace_settings
    FROM workspace
    WHERE id = NEW.workspace_id
    -- A policy change updates a non-key column and therefore takes a
    -- FOR NO KEY UPDATE lock. FOR KEY SHARE is compatible with that lock and
    -- lets an insert observe the old setting while enablement is uncommitted.
    -- FOR SHARE conflicts with the update, so the trigger waits and then
    -- evaluates the committed policy before admitting the issue row.
    FOR SHARE;

    IF NEW.creator_type = 'agent'
       AND COALESCE(workspace_settings, '{}'::jsonb) @> '{"require_member_issue_creation": true}'::jsonb THEN
        RAISE EXCEPTION USING
            ERRCODE = '42501',
            MESSAGE = 'workspace policy requires a member to create issues',
            CONSTRAINT = 'issue_agent_creation_policy';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER issue_require_member_creation
BEFORE INSERT ON issue
FOR EACH ROW
EXECUTE FUNCTION enforce_member_issue_creation_policy();
