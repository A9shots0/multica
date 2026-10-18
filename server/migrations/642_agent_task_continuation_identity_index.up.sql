CREATE UNIQUE INDEX CONCURRENTLY uq_agent_task_continuation_receipt_root_next_step
    ON agent_task_continuation_receipt (root_task_id, next_step);

