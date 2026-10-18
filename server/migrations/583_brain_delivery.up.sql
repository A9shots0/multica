-- References only; no cross-service foreign keys or implicit indexes.
CREATE TABLE brain_task_link (
  workspace_id uuid NOT NULL,
  task_id uuid NOT NULL,
  attempt integer NOT NULL,
  project_id text NOT NULL,
  target_revision text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE brain_event_outbox (
  event_id text NOT NULL,
  workspace_id uuid NOT NULL,
  project_id text NOT NULL,
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  delivered_at timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  last_error text,
  is_blocked boolean NOT NULL DEFAULT false
);
