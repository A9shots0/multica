CREATE UNIQUE INDEX CONCURRENTLY agent_task_loop_observation_identity_key ON agent_task_loop_observation (workspace_id, task_id, task_attempt, provider_attempt, fingerprint_sha256);
