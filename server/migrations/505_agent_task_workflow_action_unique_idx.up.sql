CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_agent_task_workflow_action ON agent_task_queue (workflow_action_id) WHERE workflow_action_id IS NOT NULL;
