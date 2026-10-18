CREATE FUNCTION capture_brain_task_event() RETURNS trigger LANGUAGE plpgsql AS $body$
BEGIN
INSERT INTO brain_event_outbox (event_id,workspace_id,project_id,payload)
SELECT e.event_id,l.workspace_id,l.project_id,
  jsonb_build_object('project',l.project_id,'taskId',t.id::text,
    'kind',e.kind,'summary',e.summary,'observedAt',e.observed_at,
    'evidenceRefs',jsonb_build_array('multica://workspace/'||l.workspace_id::text||'/task/'||t.id::text),
    'targetRevision',l.target_revision,'sourceEventId',e.event_id,
    'ownerSystem','multica','idempotencyKey',e.event_id)
FROM brain_task_link l
JOIN agent_task_queue t ON t.id=l.task_id AND t.attempt=l.attempt
JOIN agent a ON a.id=t.agent_id AND a.workspace_id=l.workspace_id
CROSS JOIN LATERAL (
 SELECT 'multica:'||l.workspace_id::text||':'||t.id::text||':'||t.attempt::text||':started' AS event_id,
 'started'::text AS kind, t.started_at AS observed_at,
 'Multica saved a run start. Source evidence is the canonical run record; task acceptance is not established.'::text AS summary
 WHERE t.started_at IS NOT NULL
 UNION ALL
 SELECT 'multica:'||l.workspace_id::text||':'||t.id::text||':'||t.attempt::text||':'||t.status,
 t.status,t.completed_at,
 'Multica saved run status '||t.status||'. Result availability: '||CASE WHEN t.result IS NULL THEN 'missing' ELSE 'recorded' END||'. This observation does not accept the issue or certify a deployment. Read the canonical run for outcome details.'
 WHERE t.completed_at IS NOT NULL AND t.status IN ('completed','failed','cancelled')
) e
WHERE t.id=NEW.id AND NOT EXISTS (SELECT 1 FROM brain_event_outbox o WHERE o.event_id=e.event_id)
ORDER BY e.observed_at,e.event_id LIMIT 100
ON CONFLICT (event_id) DO NOTHING;
RETURN NEW;
END;
$body$;
CREATE TRIGGER brain_task_event AFTER UPDATE OF status,started_at,completed_at ON agent_task_queue
FOR EACH ROW EXECUTE FUNCTION capture_brain_task_event();
