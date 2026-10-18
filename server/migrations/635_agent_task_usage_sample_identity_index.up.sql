CREATE UNIQUE INDEX CONCURRENTLY agent_task_usage_sample_identity_key ON agent_task_usage_sample (workspace_id, task_id, provider_attempt, backend_attempt, provider, model, sequence);
