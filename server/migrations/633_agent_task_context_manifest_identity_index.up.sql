CREATE UNIQUE INDEX CONCURRENTLY agent_task_context_manifest_identity_key ON agent_task_context_manifest (workspace_id, task_id, provider_attempt);
