-- The migration 465 relationship repair is intentionally irreversible: rolling
-- back workflow v2 must not restore forbidden foreign keys or cascades.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM issue_current_picture LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_target_head LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_attention LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_budget LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_action_receipt LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_command LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_incident LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_evidence LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_release LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_review LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_candidate LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_decision_response LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_decision LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_execution LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_authority LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_outcome LIMIT 1)
        OR EXISTS (SELECT 1 FROM issue_workflow_scope LIMIT 1)
    THEN
        RAISE EXCEPTION 'workflow v2 contains durable records; export or remove them through application cleanup before rollback';
    END IF;
END $$;

DROP TABLE IF EXISTS issue_current_picture;
DROP TABLE IF EXISTS issue_workflow_target_head;
DROP TABLE IF EXISTS issue_workflow_attention;
DROP TABLE IF EXISTS issue_workflow_budget;
DROP TABLE IF EXISTS issue_workflow_action_receipt;
DROP TABLE IF EXISTS issue_workflow_command;
DROP TABLE IF EXISTS issue_workflow_incident;
DROP TABLE IF EXISTS issue_workflow_evidence;
DROP TABLE IF EXISTS issue_workflow_release;
DROP TABLE IF EXISTS issue_workflow_review;
DROP TABLE IF EXISTS issue_workflow_candidate;
DROP TABLE IF EXISTS issue_workflow_decision_response;
DROP TABLE IF EXISTS issue_workflow_decision;
DROP TABLE IF EXISTS issue_workflow_execution;
DROP TABLE IF EXISTS issue_workflow_authority;
DROP TABLE IF EXISTS issue_workflow_outcome;
DROP TABLE IF EXISTS issue_workflow_scope;
