// Static display samples only: no workflow execution, daemon, model or provider.
// This script is intentionally fixed to the isolated local candidate project.
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import pg from 'pg';

const project = 'smk-local-candidate-20260915';
const base = 'http://127.0.0.1:18084';
const email = 'preview-owner@local.invalid';
const slug = 'local-candidate-samples';
const container = execFileSync('docker', ['ps', '-q', '--filter', `label=com.docker.compose.project=${project}`,
  '--filter', 'label=com.docker.compose.service=gateway'], { encoding: 'utf8' }).trim();
if (!/^[a-f0-9]{12,64}$/.test(container)) throw new Error('Expected exactly one candidate gateway container.');
const binding = JSON.parse(execFileSync('docker', ['inspect', '--format', '{{json .NetworkSettings.Ports}}', container], { encoding: 'utf8' }));
for (const [inside, outside] of [['5432/tcp', '15437'], ['8080/tcp', '18084'], ['3000/tcp', '13004']]) {
  if (binding[inside]?.length !== 1 || binding[inside][0].HostIp !== '127.0.0.1' ||
      binding[inside][0].HostPort !== outside) throw new Error('Unexpected candidate gateway port.');
}

let token;
async function api(path, body) {
  const response = await fetch(base + path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  if (!response.ok) throw new Error(`Local fixture API ${path}: HTTP ${response.status}`);
  return response.json();
}

const client = new pg.Client({ host: '127.0.0.1', port: 15437, database: 'multica_preview',
  user: 'multica', password: 'local-preview-only' });
await client.connect();
try {
  const dbName = (await client.query('SELECT current_database() AS name')).rows[0].name;
  if (dbName !== 'multica_preview') throw new Error('Unexpected database.');
  const counts = (await client.query(`SELECT (SELECT count(*) FROM agent) AS agents,
    (SELECT count(*) FROM agent_runtime) AS runtimes, (SELECT count(*) FROM agent_task_queue) AS tasks,
    (SELECT count(*) FROM workspace WHERE slug <> $1) AS other_workspaces`, [slug])).rows[0];
  if (Object.values(counts).some(value => Number(value) !== 0)) throw new Error('Candidate contains non-sample operational data.');
  const existing = (await client.query('SELECT id FROM workspace WHERE slug=$1', [slug])).rows[0];
  if (existing && Number((await client.query('SELECT count(*) AS n FROM issue WHERE workspace_id=$1', [existing.id])).rows[0].n) !== 0) {
    throw new Error('Samples already exist. Preserve them; this is a one-time initializer.');
  }

  await api('/auth/send-code', { email });
  const login = await api('/auth/verify-code', { email, code: '654321' });
  token = login.token;
  const user = login.user;
  const workspace = existing ?? await api('/api/workspaces', {
    name: 'Local candidate samples', slug, issue_prefix: 'SAMPLE',
    description: 'Synthetic display samples. No agent has executed these issues and no release occurred.',
  });

  await client.query('BEGIN');
  const owner = await client.query('SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2 AND role=$3', [workspace.id, user.id, 'owner']);
  if (owner.rowCount !== 1 || user.email !== email) throw new Error('Sample owner mismatch.');
  await client.query('UPDATE "user" SET name=$1, onboarded_at=now(), starter_content_state=$2 WHERE id=$3 AND email=$4',
    ['Preview Owner', 'skipped', user.id, email]);

  async function insert(table, values) {
    const columns = Object.keys(values);
    const params = columns.map((_, i) => `$${i + 1}`);
    const data = Object.values(values).map(value => typeof value === 'object' && value !== null ? JSON.stringify(value) : value);
    await client.query(`INSERT INTO ${table} (${columns.join(',')}) VALUES (${params.join(',')})`, data);
  }

  const now = new Date().toISOString();
  const samples = [
    { title: '[Sample] Inspect a stopped workflow', status: 'in_progress', execution: 'failed', outcome: 'open', stage: 'technical_stop' },
    { title: '[Sample] Local review completed', status: 'done', execution: 'completed', outcome: 'satisfied', stage: 'complete' },
    { title: '[Sample] New request without workflow status', status: 'todo', stage: 'status_unavailable' },
  ];
  const receipt = { created_at: now, workspace_id: workspace.id, slug, email, issues: [],
    boundary: 'Static display fixtures; zero agents, runtimes, tasks or provider calls.' };

  for (const [index, sample] of samples.entries()) {
    const id = randomUUID();
    await insert('issue', { id, workspace_id: workspace.id, title: sample.title,
      description: 'Synthetic local browser sample only. This record is not evidence of a completed workflow, provider run, deployment or business outcome.',
      status: sample.status, priority: 'none', creator_type: 'member', creator_id: user.id,
      number: index + 1, position: index, last_activity_at: now });
    receipt.issues.push({ id, number: index + 1, title: sample.title, expected_stage: sample.stage });
    if (!sample.execution) continue;

    const outcomeId = randomUUID();
    await insert('issue_workflow_outcome', { id: outcomeId, workspace_id: workspace.id, issue_id: id,
      revision: 1, state: sample.outcome, acceptance_version: 1, created_at: now });
    const change = { id: outcomeId, revision: 1, record_type: 'outcome', change_code: 'outcome_changed',
      result_code: 'recorded', occurred_at: now };
    const document = {
      contract_version: 2, owner_summary_schema_version: 1,
      original_business_request: { description: 'Static display sample' },
      outcome: { id: outcomeId, revision: 1, state: sample.outcome }, historical_completion: {},
      scope: { revision: 1, requires_deployment: false }, authority: { revision: 1 },
      execution: { state: sample.execution }, active_branches: [], primary_branch: {},
      release: { state: 'not_required' }, health: 'unverified', attention: { resolution_state: 'resolved' },
      current_milestone: sample.stage, latest_meaningful_change: change, next_automatic_action: {},
      active_actor: {}, wait_reason: {}, candidate: {}, target: {}, evidence_summary: {}, authority_summary: {},
      unresolved_decisions: [], unresolved_incidents: [], unresolved_dependencies: [], budgets: [],
      supersession: {}, terminal_reason: '', next_actor: {},
    };
    await insert('issue_current_picture', {
      workspace_id: workspace.id, issue_id: id, contract_version: 2, revision: 1,
      scope_revision: 1, authority_revision: 1, outcome_state: sample.outcome, outcome_open: sample.outcome === 'open',
      execution_state: sample.execution, release_state: 'not_required', health_state: 'unverified', attention_state: 'resolved',
      current_milestone: sample.stage, latest_meaningful_change: change, next_automatic_action: {}, active_actor: {},
      wait_reason: {}, candidate: {}, target: {}, evidence_summary: {}, authority_summary: {},
      unresolved_decisions: [], unresolved_incidents: [], unresolved_dependencies: [], budget_summary: [],
      supersession_summary: {}, terminal_reason: '', next_actor: {}, source_revisions: [], document, generated_at: now,
    });
    await insert('inbox_item', { id: randomUUID(), workspace_id: workspace.id, recipient_type: 'member', recipient_id: user.id,
      actor_type: 'system', type: index === 0 ? 'task_failed' : 'task_completed',
      severity: index === 0 ? 'attention' : 'info', issue_id: id, title: sample.title,
      body: 'Synthetic display notification. Inspect the linked sample; no task was executed.',
      details: { owner_relevant: 'true', owner_category: index === 0 ? 'incident' : 'completion', sample: 'true' } });

    if (index === 0) {
      const shared = { workspace_id: workspace.id, issue_id: id, created_at: now };
      await insert('issue_workflow_decision_response', { ...shared, id: randomUUID(), decision_id: randomUUID(),
        decision_revision: 7, response_type: 'approve', actor_id: user.id, idempotency_key: randomUUID(), request_hash: 'static-display-sample' });
      await insert('issue_workflow_review', { ...shared, id: randomUUID(), candidate_id: randomUUID(), candidate_revision: 3,
        scope_id: randomUUID(), scope_revision: 2, reviewer_id: user.id, outcome: 'passed' });
      await insert('issue_workflow_evidence', { ...shared, id: randomUUID(), concern_type: 'candidate', evidence_type: 'display_fixture',
        level: 'local_code', source: 'static-display-sample', producer_type: 'member', observed_at: now,
        payload: { sample: true, limitation: 'No workflow was executed.' } });
    }
  }
  await client.query('UPDATE workspace SET issue_counter=$1 WHERE id=$2', [samples.length, workspace.id]);
  await client.query('COMMIT');
  writeFileSync(new URL('../.qualification-evidence/local-integration-20260915/preview-fixtures.json', import.meta.url), JSON.stringify(receipt, null, 2) + '\n');
  console.log(JSON.stringify(receipt, null, 2));
} catch (error) {
  await client.query('ROLLBACK');
  throw error;
} finally {
  token = undefined;
  await client.end();
}
