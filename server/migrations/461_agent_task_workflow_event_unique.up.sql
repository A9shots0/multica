CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_agent_task_workflow_event
    ON agent_task_queue (trigger_evidence_ref_id)
    WHERE trigger_evidence_kind = 'workflow_event'
      AND trigger_evidence_ref_id IS NOT NULL;
