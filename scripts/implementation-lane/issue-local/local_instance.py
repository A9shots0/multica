"""A disposable local Multica for proving that an issue starts the lane.

Nothing here touches the live private instance: its own pgvector container on a
private loopback port, a locally built server and daemon, and a state directory
under .qualification-evidence (gitignored). Secrets it generates (database
password, JWT secret, development login code) stay in that directory and are
never printed.

    python local_instance.py up        # database, migrations, server
    python local_instance.py bootstrap # owner login, workspace, daemon, roles, squad
    python local_instance.py issue <spec.json> <title>  # create and launch one issue
    python local_instance.py inspect <issue-id>         # read the five surfaces
    python local_instance.py down      # stop daemon, server and database
"""
import hashlib
import json
import os
import secrets
import shutil
import subprocess
import sys
import time
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[3]
LANE = ROOT / 'scripts' / 'implementation-lane'
BIN = ROOT / 'server' / 'bin'
STATE = ROOT / '.qualification-evidence' / 'issue-lane-local'
# Where the lane creates task worktrees (lane.WORKTREE_PARENT).
WORKTREE_PARENT = Path('C:/Users/a9sho/.codex/worktrees/implementation-lane')
DB_CONTAINER = 'lane-issue-local-pg'
DB_PORT = 55434
SERVER_PORT = 18190
BASE = f'http://127.0.0.1:{SERVER_PORT}'
OWNER_EMAIL = 'lane-owner@example.invalid'
DAEMON_ID = 'lane-issue-local-daemon'
IMPLEMENTER, REVIEWER, LEADER = 'Lane Implementer', 'Lane Evidence Reviewer', 'Lane Planner'
# The reviewer turn runs under a workflow-broker grant (option B, as in the #40
# native acceptance): the controller routes it as a bounded task, the daemon
# launches the review broker named in the grant, and the broker asks one
# tool-free Claude turn to classify its deterministic verification.
LANE_RUNS = ROOT / '.qualification-evidence' / 'implementation-lane'
REVIEW_BROKER = BIN / 'implementation-lane-review-broker.exe'
REVIEW_AUTHORITY = BIN / 'implementation-lane-review-authority.exe'
PROVIDER = Path(r'C:\Users\a9sho\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe')
CREDENTIAL_SOURCE = Path.home() / '.claude' / '.credentials.json'
PROVIDER_HOME = STATE / 'provider-home'
HOST_ATTESTATION = 'alienware_windows'
GRANT_HOURS = 3


def now():
    return datetime.now(timezone.utc).isoformat()


def load(name, default=None):
    path = STATE / name
    return json.loads(path.read_text(encoding='utf-8')) if path.exists() else default


def save(name, value):
    STATE.mkdir(parents=True, exist_ok=True)
    (STATE / name).write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


def secret(name, factory):
    value = load('secrets.json', {})
    if name not in value:
        value[name] = factory()
        save('secrets.json', value)
    return value[name]


def api(method, path, body=None, token=None, workspace=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if workspace:
        headers['X-Workspace-ID'] = workspace
    data = None if body is None else json.dumps(body).encode()
    try:
        with urlopen(Request(BASE + path, data=data, headers=headers, method=method), timeout=60) as r:
            raw = r.read()
    except HTTPError as e:
        raise RuntimeError(f'{method} {path}: HTTP {e.code}: {e.read(400).decode(errors="replace")}') from None
    return json.loads(raw) if raw.strip() else None


def database_url():
    password = secret('db_password', lambda: secrets.token_hex(16))
    return f'postgres://multica:{password}@127.0.0.1:{DB_PORT}/multica?sslmode=disable'


def daemon_launch_dir(work):
    """Directory the daemon process starts in: never its own workspaces root.

    On its first start the daemon writes a permanent guard marker,
    <workspaces root>/.multica/daemon_task_context.json, so that a task
    subprocess escaping its workdir still fails closed. The CLI looks for that
    marker from its working directory upwards. Started from inside the
    workspaces root, every later `daemon start` found it and refused: "daemon
    start is not available inside a daemon-managed task". The first proof
    worked only because the root did not exist yet.
    """
    launch = STATE
    work, launch = Path(work).resolve(), Path(launch).resolve()
    if launch == work or work in launch.parents:
        raise RuntimeError(f'daemon launch directory {launch} is inside its workspaces root {work}')
    return launch


def server_env():
    env = dict(os.environ)
    for name in list(env):
        if 'DATABASE_URL' in name or name.startswith('CLAUDE_CODE_MESSAGING'):
            env.pop(name)
    env.update({
        'DATABASE_URL': database_url(), 'PORT': str(SERVER_PORT), 'APP_ENV': 'development',
        'JWT_SECRET': secret('jwt', lambda: secrets.token_hex(32)),
        'MULTICA_DEV_VERIFICATION_CODE': secret('dev_code', lambda: f'{secrets.randbelow(900000) + 100000}'),
        'FRONTEND_ORIGIN': f'http://127.0.0.1:{SERVER_PORT + 1}', 'MULTICA_APP_URL': f'http://127.0.0.1:{SERVER_PORT + 1}',
        'LOCAL_UPLOAD_DIR': str(STATE / 'uploads'), 'ALLOW_SIGNUP': 'true',
        # Enables the permission receipt store, so a stored workflow-broker grant
        # turns the reviewer's transition task into a guarded broker execution.
        'MULTICA_WORKFLOW_CONTROLLER_ID': secret('controller_id', lambda: str(uuid.uuid4())),
    })
    return env


def up():
    STATE.mkdir(parents=True, exist_ok=True)
    running = subprocess.run(['docker', 'ps', '-q', '-f', f'name={DB_CONTAINER}'], capture_output=True, text=True).stdout.strip()
    if not running:
        # A fresh database has no users or workspaces: sessions and ids saved
        # against an earlier database would point at rows that no longer exist.
        for name in ('owner-session.json', 'bootstrap.json', 'proof-issue.json'):
            (STATE / name).unlink(missing_ok=True)
        subprocess.run(['docker', 'run', '-d', '--rm', '--name', DB_CONTAINER, '-e', 'POSTGRES_USER=multica',
                        '-e', 'POSTGRES_PASSWORD=' + secret('db_password', lambda: secrets.token_hex(16)),
                        '-e', 'POSTGRES_DB=multica', '-p', f'127.0.0.1:{DB_PORT}:5432', 'pgvector/pgvector:pg17'],
                       check=True, capture_output=True)
    for _ in range(90):
        if subprocess.run(['docker', 'exec', DB_CONTAINER, 'pg_isready', '-U', 'multica', '-d', 'multica'],
                          capture_output=True).returncode == 0:
            break
        time.sleep(1)
    migrate = subprocess.run([str(BIN / 'lane-local-migrate.exe'), 'up'], env=server_env(), capture_output=True, text=True,
                             cwd=str(ROOT / 'server'))
    if migrate.returncode:
        raise RuntimeError('migrations failed: ' + migrate.stderr[-800:])
    if not load('server-process.json'):
        log = (STATE / 'server.log').open('ab')
        proc = subprocess.Popen([str(BIN / 'lane-local-server.exe')], env=server_env(), cwd=str(ROOT / 'server'),
                                stdout=log, stderr=subprocess.STDOUT, creationflags=subprocess.CREATE_NO_WINDOW)
        save('server-process.json', {'pid': proc.pid, 'started_at': now()})
    for _ in range(90):
        try:
            health = api('GET', '/health')
            print('server healthy:', {k: health.get(k) for k in ('status', 'commit')})
            return
        except Exception:
            time.sleep(1)
    raise RuntimeError('local server did not become healthy; see server.log')


def daemon_token_record(daemon_token, workspace_id, expires_at):
    """What is stored for a scoped daemon token: its SHA-256, never the token."""
    if not daemon_token.startswith('mdt_') or len(daemon_token) < 44:
        raise RuntimeError('a scoped daemon token must be a random mdt_ token')
    return {'token_hash': hashlib.sha256(daemon_token.encode()).hexdigest(), 'workspace_id': str(uuid.UUID(workspace_id)),
            'daemon_id': DAEMON_ID, 'expires_at': expires_at}


def lane_db(sql):
    result = subprocess.run(['docker', 'exec', '-i', DB_CONTAINER, 'psql', '-U', 'multica', '-d', 'multica', '-Atq',
                             '-v', 'ON_ERROR_STOP=1', '-f', '-'], input=sql, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError('local database operation failed')
    return result.stdout.strip()


def provision_daemon_identity(token, ws, profile):
    """Give the proof daemon a scoped daemon identity, as the #40 acceptance did
    (manage.py provision_daemon). Guarded broker executions require an
    authenticated daemon id, which only an mdt_ daemon token carries; the first
    option-B proof ran the daemon on a personal access token and its guarded
    reviewer start was refused.

    The owner session registers the runtime offline so it is owned by the owner;
    the daemon then comes online with the scoped token. Only the token's SHA-256
    is stored, bound to this daemon id, the workspace and a short expiry, and the
    row is deleted at shutdown."""
    api('POST', '/api/daemon/register', {'workspace_id': ws, 'daemon_id': DAEMON_ID,
                                         'device_name': 'Alienware lane issue proof', 'cli_version': 'task-bound-source-build',
                                         'runtimes': [{'name': 'Implementation lane bridge', 'type': 'claude',
                                                       'version': '2.1.272', 'status': 'offline'}]}, token, ws)
    daemon_token = 'mdt_' + secrets.token_hex(20)
    expires = (datetime.now(timezone.utc) + timedelta(hours=GRANT_HOURS + 1)).replace(microsecond=0).isoformat()
    record = daemon_token_record(daemon_token, ws, expires)
    lane_db(f"DELETE FROM daemon_token WHERE daemon_id='{record['daemon_id']}';"
            f"INSERT INTO daemon_token(id, token_hash, workspace_id, daemon_id, expires_at) VALUES (gen_random_uuid(), "
            f"'{record['token_hash']}', '{record['workspace_id']}', '{record['daemon_id']}', '{record['expires_at']}'::timestamptz);")
    profile.mkdir(parents=True, exist_ok=True)
    (profile / 'config.json').write_text(json.dumps({'server_url': BASE, 'workspace_id': ws, 'token': daemon_token}),
                                         encoding='utf-8')


def remove_daemon_identity():
    lane_db(f"DELETE FROM daemon_token WHERE daemon_id='{DAEMON_ID}';")


def owner_token():
    session = load('owner-session.json')
    if session:
        return session['token']
    api('POST', '/auth/send-code', {'email': OWNER_EMAIL})
    login = api('POST', '/auth/verify-code', {'email': OWNER_EMAIL, 'code': secret('dev_code', lambda: '')})
    save('owner-session.json', {'token': login['token']})
    return login['token']


def bootstrap():
    token = owner_token()
    state = load('bootstrap.json', {})
    if 'workspace_id' not in state:
        workspaces = api('GET', '/api/workspaces', token=token) or []
        ws = next((w for w in workspaces if w.get('slug') == 'lane-local'), None)
        ws = ws or api('POST', '/api/workspaces', {'name': 'Lane Local', 'slug': 'lane-local'}, token)
        state['workspace_id'] = ws['id']
        save('bootstrap.json', state)
    ws = state['workspace_id']
    profile = Path.home() / '.multica' / 'profiles' / DAEMON_ID
    if not (profile / 'config.json').exists():
        provision_daemon_identity(token, ws, profile)
    if not load('daemon-process.json'):
        env = dict(server_env())
        for name in ('DATABASE_URL', 'JWT_SECRET', 'MULTICA_DEV_VERIFICATION_CODE'):
            env.pop(name, None)
        env.update({
            'MULTICA_CLAUDE_PATH': str(BIN / 'implementation-lane-bridge.exe'),
            'LANE_BRIDGE_PYTHON': sys.executable,
            'LANE_BRIDGE_RUN_TASK': str(LANE / 'run_task.py'),
            'LANE_BRIDGE_STATE_DIR': str(STATE / 'bridge'),
            'LANE_BRIDGE_REPOSITORIES': 'C:/Dev/multica;C:/Dev/CRM',
            # Guarded broker executions are admitted only for the attested host.
            'MULTICA_HOST_ATTESTATION': HOST_ATTESTATION,
        })
        work = STATE / 'daemon-workspaces'
        work.mkdir(parents=True, exist_ok=True)
        log = (STATE / 'daemon.log').open('ab')
        proc = subprocess.Popen([str(BIN / 'lane-local-multica.exe'), 'daemon', 'start', '--foreground',
                                 '--daemon-id', DAEMON_ID, '--device-name', 'Alienware lane issue proof',
                                 '--runtime-name', 'Implementation lane bridge', '--workspaces-root', str(work),
                                 '--agent-timeout', '110m0s', '--max-concurrent-tasks', '1',
                                 '--no-auto-update', '--no-auto-reload', '--profile', DAEMON_ID],
                                env=env, cwd=str(daemon_launch_dir(work)), stdout=log, stderr=subprocess.STDOUT,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        save('daemon-process.json', {'pid': proc.pid, 'started_at': now()})
    runtime = None
    for _ in range(60):
        runtimes = api('GET', '/api/runtimes', token=token, workspace=ws) or []
        runtimes = runtimes.get('runtimes', runtimes) if isinstance(runtimes, dict) else runtimes
        runtime = next((r for r in runtimes if r.get('provider') == 'claude' and r.get('status') == 'online'), None)
        if runtime:
            break
        time.sleep(2)
    if not runtime:
        raise RuntimeError('the daemon did not register an online claude runtime; see daemon.log')
    state['runtime_id'] = runtime['id']
    roles = state.get('roles', {})
    for label, name in (('leader', LEADER), ('implementer', IMPLEMENTER), ('reviewer', REVIEWER)):
        if label not in roles:
            agent = api('POST', '/api/agents/', {
                'name': name, 'description': 'Implementation lane role on the local proof instance.',
                'runtime_id': runtime['id'], 'instructions': '', 'runtime_config': {}, 'custom_args': [],
                'mcp_config': {}, 'permission_mode': 'private', 'invocation_targets': [],
                'max_concurrent_tasks': 1, 'model': 'claude-sonnet-4-6', 'thinking_level': 'medium',
                'composio_toolkit_allowlist': []}, token, ws)
            roles[label] = agent['id']
            state['roles'] = roles
            save('bootstrap.json', state)
    if 'squad_id' not in state:
        squad = api('POST', '/api/squads/', {'name': 'Implementation Lane', 'description': 'Issue-started implementation lane.',
                                             'leader_id': roles['leader']}, token, ws)
        state['squad_id'] = squad['id']
        for label in ('implementer', 'reviewer'):
            api('POST', f"/api/squads/{squad['id']}/members",
                {'member_type': 'agent', 'member_id': roles[label], 'role': label}, token, ws)
        api('PUT', f"/api/squads/{squad['id']}/workflow-template",
            {'implementer_id': roles['implementer'], 'reviewer_id': roles['reviewer'], 'requires_owner': False,
             'requires_deployment': False, 'max_corrections': 1}, token, ws)
        save('bootstrap.json', state)
    print('bootstrap ready: workspace, daemon runtime online, three roles, squad and workflow template')
    return state, token, ws


def create_and_launch(state, token, ws, spec_path, title):
    spec = json.loads(Path(spec_path).read_text(encoding='utf-8'))
    description = ('Owner-started implementation lane proof. The lane task below is the whole instruction; '
                   'the bridge runs it and publishes the result here.\n\n```lane-task\n'
                   + json.dumps(spec, indent=2) + '\n```\n')
    issue = api('POST', '/api/issues/', {'workspace_id': ws, 'title': title, 'description': description,
                                         'status': 'backlog', 'priority': 'none'}, token, ws)
    roles = state['roles']
    api('PUT', f"/api/issues/{issue['id']}", {'status': 'backlog', 'assignee_type': 'squad',
                                             'assignee_id': state['squad_id']}, token, ws)
    api('PUT', f"/api/issues/{issue['id']}/workflow-plan", {
        'squad_id': state['squad_id'], 'implementer_id': roles['implementer'], 'reviewer_id': roles['reviewer'],
        'requires_owner': False, 'requires_deployment': False}, token, ws)
    # The review grant must exist before the issue has any task: the reviewer's
    # transition task picks it up when it is enqueued.
    prepare_review_grant(state, token, ws, issue['id'])
    # Start the workflow the way the product does: through the squad. The
    # mention wakes the squad leader with a squad-scoped task, the leader
    # delegates to the implementer, and that worker task inherits the squad, so
    # the workflow controller recognises it. A direct @implementer mention
    # creates a task with no squad: its result comment then woke the leader as
    # ordinary agent chatter and the workflow stopped (proof 2026-09-17 06:42Z).
    content = (f"Please run the implementation lane task in this issue. "
               f"[@Implementation Lane](mention://squad/{state['squad_id']})")
    api('POST', f"/api/issues/{issue['id']}/comments",
        {'content': content, 'suppress_agent_ids': [roles['reviewer']]}, token, ws)
    save('proof-issue.json', {'issue_id': issue['id'], 'title': title, 'launched_at': now(), 'spec': str(spec_path)})
    return issue['id']


def sha256_file(path):
    with open(path, 'rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def windows_path(path):
    return str(Path(path).resolve()).replace('/', '\\')


def build_review_broker(state, issue_id):
    """Build the review broker bound to this one issue, its three roles and this instance."""
    roles = state['roles']
    ldflags = ' '.join([
        f'-X main.laneIssueID={issue_id}',
        f"-X main.laneRoleIDs={roles['leader']},{roles['implementer']},{roles['reviewer']}",
        f"-X 'main.reviewerAgentName={REVIEWER}'",
        f'-X main.stateDirectory={windows_path(STATE / "bridge")}',
        f'-X main.evidenceRoot={windows_path(LANE_RUNS)}',
        f'-X main.serverPort={SERVER_PORT}',
    ])
    subprocess.run(['go', 'build', '-p', '2', '-trimpath', '-ldflags', ldflags, '-o', str(REVIEW_BROKER),
                    './cmd/implementation-lane-review-broker'], cwd=str(ROOT / 'server'), check=True)
    return sha256_file(REVIEW_BROKER)


def private_dir(path):
    if path.is_symlink():
        raise RuntimeError('a private directory cannot be a link')
    path.mkdir(parents=True, exist_ok=True)
    who = subprocess.run(['whoami'], capture_output=True, text=True, check=True).stdout.strip()
    subprocess.run(['icacls', str(path), '/inheritance:r', '/grant:r', who + ':(OI)(CI)F'],
                   capture_output=True, text=True, check=True)


def take_provider_custody():
    """Opaque copy of the existing Claude subscription sign-in into a private,
    task-local profile, as in the #40 acceptance. The bytes are never read,
    parsed, hashed or printed, and the profile is removed at shutdown."""
    if not CREDENTIAL_SOURCE.is_file() or CREDENTIAL_SOURCE.is_symlink():
        raise RuntimeError('the existing Claude subscription sign-in is unavailable')
    if PROVIDER_HOME.exists():
        remove_provider_profile()
    private_dir(PROVIDER_HOME)
    (PROVIDER_HOME / '.claude').mkdir()
    shutil.copyfile(CREDENTIAL_SOURCE, PROVIDER_HOME / '.claude' / '.credentials.json')


def remove_provider_profile():
    home = PROVIDER_HOME
    if home.is_symlink() or STATE.is_symlink():
        raise RuntimeError('provider profile cleanup target is a link')
    if not home.exists():
        return
    if home.resolve().parent != STATE.resolve():
        raise RuntimeError('provider profile cleanup target is outside this instance')
    shutil.rmtree(home)


def authority_env():
    env = {k: v for k, v in os.environ.items() if k.upper() in {'SYSTEMROOT', 'WINDIR', 'PATH', 'TEMP', 'TMP', 'USERPROFILE'}}
    env.update({'DATABASE_URL': database_url(), 'APP_ENV': 'development',
                'MULTICA_WORKFLOW_CONTROLLER_ID': secret('controller_id', lambda: str(uuid.uuid4()))})
    return env


def review_authority(mode, contract):
    result = subprocess.run([str(REVIEW_AUTHORITY), mode], input=json.dumps(contract), capture_output=True, text=True,
                            env=authority_env())
    if result.returncode:
        raise RuntimeError(f'review grant {mode} refused: {result.stderr.strip()[:400]}')
    print(result.stdout.strip(), flush=True)


def review_grant_contract(state, ws, owner_id, issue_id, broker_sha, provider_sha, issued):
    return {
        'controller_id': secret('controller_id', lambda: str(uuid.uuid4())), 'workspace_id': ws, 'owner_id': owner_id,
        'issue_id': issue_id, 'squad_id': state['squad_id'], 'runtime_id': state['runtime_id'],
        'implementer_id': state['roles']['implementer'], 'reviewer_id': state['roles']['reviewer'], 'reviewer_name': REVIEWER,
        'broker_path': windows_path(REVIEW_BROKER), 'broker_sha256': broker_sha,
        'provider_path': windows_path(PROVIDER), 'provider_sha256': provider_sha,
        'provider_profile_path': windows_path(PROVIDER_HOME),
        'evidence_receipt_id': 'implementation-lane-review-broker-sha256.' + broker_sha,
        'issued_at': issued.isoformat().replace('+00:00', 'Z'),
        'expires_at': (issued + timedelta(hours=GRANT_HOURS)).isoformat().replace('+00:00', 'Z'),
    }


def prepare_review_grant(state, token, ws, issue_id):
    broker_sha = build_review_broker(state, issue_id)
    take_provider_custody()
    me = api('GET', '/api/me', token=token)
    issued = datetime.now(timezone.utc).replace(microsecond=0)
    contract = review_grant_contract(state, ws, me['id'], issue_id, broker_sha, sha256_file(PROVIDER), issued)
    save('review-grant-contract.json', contract)
    review_authority('prepare', contract)
    review_authority('issue', contract)


def revoke_review_grant():
    contract = load('review-grant-contract.json')
    if not contract:
        return
    try:
        review_authority('revoke', contract)
    finally:
        (STATE / 'review-grant-contract.json').unlink(missing_ok=True)


TERMINAL = {'completed', 'failed', 'cancelled'}


def wait_for_outcome(issue_id, token, ws, timeout_minutes=110):
    deadline = time.time() + timeout_minutes * 60
    seen = {}
    while time.time() < deadline:
        issue = api('GET', f'/api/issues/{issue_id}', token=token, workspace=ws)
        runs = api('GET', f'/api/issues/{issue_id}/task-runs', token=token, workspace=ws) or []
        runs = runs.get('tasks', runs) if isinstance(runs, dict) else runs
        for run in runs:
            if seen.get(run['id']) != run['status']:
                seen[run['id']] = run['status']
                print(f"{now()} task {run['id'][:8]} {run.get('agent_name') or run.get('agent_id', '')[:8]} -> {run['status']}",
                      flush=True)
        if issue.get('status') == 'done':
            return 'done'
        failed = [r for r in runs if r['status'] in ('failed', 'cancelled')]
        if failed and all(r['status'] in TERMINAL for r in runs):
            return 'failed'
        time.sleep(30)
    return 'timeout'


def lane_for(issue):
    """Python port of operationalLaneForIssue (packages/views/issues/components/operational-workflow.tsx)."""
    picture = issue.get('current_picture') or {}
    summary = picture.get('owner_summary')
    if not summary or summary.get('current_picture_revision') != picture.get('revision'):
        return 'status_unavailable'
    stage = summary.get('stage')
    # Signals are objects ({code, references}); the product matches on code.
    signals = {signal.get('code') for signal in (summary.get('signals') or []) if isinstance(signal, dict)}
    mapping = {'needs_owner': 'needs_owner', 'recovering': 'recovering', 'technical_stop': 'technical_stop',
               'blocked': 'technical_stop', 'verifying': 'deployed_checking', 'releasing': 'releasing',
               'in_review': 'in_review'}
    if stage in (None, 'status_unavailable'):
        return 'status_unavailable'
    if stage in mapping:
        return mapping[stage]
    if stage == 'complete':
        return 'verified_live' if 'verified_live' in signals else 'done'
    if 'waiting_automatic' in signals or 'automatic_continuation' in signals or \
            (summary.get('next_action') or {}).get('automatic') is True:
        return 'waiting_automatic'
    return 'active'


def inspect(issue_id, token=None, ws=None):
    token = token or owner_token()
    ws = ws or load('bootstrap.json')['workspace_id']
    errors = {}

    def read(label, path, key=None, default=None):
        try:
            value = api('GET', path, token=token, workspace=ws)
        except RuntimeError as error:
            errors[label] = str(error)[:300]
            return default
        if key and isinstance(value, dict):
            value = value.get(key, value)
        return value if value is not None else default

    issue = read('issue', f'/api/issues/{issue_id}', default={})
    picture = read('current_picture', f'/api/issues/{issue_id}/current-picture', default={})
    inbox = read('inbox', '/api/inbox', key='items', default=[])
    board = read('board', '/api/issues?status=done', key='issues', default=[])
    runs = read('tasks', f'/api/issues/{issue_id}/task-runs', key='tasks', default=[])
    comments = read('comments', f'/api/issues/{issue_id}/comments', key='comments', default=[])
    issue_with_picture = dict(issue, current_picture=issue.get('current_picture') or picture)
    entries = [i for i in inbox if i.get('issue_id') == issue_id]
    try:
        now_lane = lane_for(issue_with_picture)
    except Exception as error:  # a surface we cannot read is recorded, never fatal
        errors['now'] = f'{type(error).__name__}: {error}'[:300]
        now_lane = 'status_unavailable'
    surfaces = {
        'issue': {'status': issue.get('status'), 'revision': issue.get('revision')},
        'current_picture': {k: picture.get(k) for k in ('stage', 'outcome', 'execution', 'attention', 'milestone',
                                                        'terminal_reason', 'revision') if isinstance(picture, dict)},
        'now': {'lane': now_lane,
                'owner_summary': (issue_with_picture.get('current_picture') or {}).get('owner_summary')},
        'inbox': [{k: e.get(k) for k in ('type', 'severity', 'issue_status', 'title', 'body', 'read')} for e in entries],
        'board': {'in_done_query': any(i.get('id') == issue_id for i in board)},
        'tasks': [{k: r.get(k) for k in ('id', 'agent_id', 'status', 'error')} for r in runs],
        'comment_count': len(comments),
    }
    consistent = (surfaces['issue']['status'] == 'done' and surfaces['now']['lane'] == 'done'
                  and surfaces['board']['in_done_query'] and any(e.get('type') == 'task_completed' for e in entries)
                  and all(r['status'] == 'completed' for r in runs) and len(runs) >= 2)
    surfaces['read_errors'] = errors
    surfaces['all_five_surfaces_agree_done'] = consistent and not errors
    save('proof-surfaces.json', surfaces)
    print(json.dumps({k: surfaces[k] for k in ('issue', 'board', 'all_five_surfaces_agree_done')}, indent=2))
    print('now lane:', surfaces['now']['lane'], '| inbox entries:', len(entries), '| tasks:',
          [(r['status']) for r in surfaces['tasks']])
    return surfaces


def stop_pid(record_name, expected_exe):
    record = load(record_name)
    if not record:
        return
    exe = subprocess.run(['powershell', '-NoProfile', '-Command',
                          f"(Get-CimInstance Win32_Process -Filter 'ProcessId={record['pid']}').ExecutablePath"],
                         capture_output=True, text=True).stdout.strip()
    if exe.lower().endswith(expected_exe):
        subprocess.run(['taskkill', '/PID', str(record['pid']), '/T', '/F'], capture_output=True)
    (STATE / record_name).unlink(missing_ok=True)


def down():
    # Revoke the review grant while the database is still up, then remove the
    # provider profile whatever else fails.
    try:
        revoke_review_grant()
    except Exception as error:  # recorded, never blocks the rest of shutdown
        print('review grant revocation failed:', error)
    try:
        remove_provider_profile()
    except Exception as error:
        print('provider profile removal failed:', error)
    stop_pid('daemon-process.json', 'lane-local-multica.exe')
    try:
        remove_daemon_identity()
    except Exception as error:
        print('daemon token removal failed:', error)
    stop_pid('server-process.json', 'lane-local-server.exe')
    subprocess.run(['docker', 'stop', DB_CONTAINER], capture_output=True)
    profile = Path.home() / '.multica' / 'profiles' / DAEMON_ID / 'config.json'
    profile.unlink(missing_ok=True)
    for name in ('owner-session.json', 'bootstrap.json'):
        (STATE / name).unlink(missing_ok=True)
    print('local instance stopped; daemon profile and sessions removed')


def ensure_binaries(rebuild=False):
    targets = {'lane-local-server.exe': './cmd/server', 'lane-local-multica.exe': './cmd/multica',
               'lane-local-migrate.exe': './cmd/migrate', 'implementation-lane-bridge.exe': './cmd/implementation-lane-bridge',
               'implementation-lane-review-authority.exe': './cmd/implementation-lane-review-authority'}
    for name, package in targets.items():
        # A proof always rebuilds (Go's cache keeps it fast): a stale binary once
        # ran old bridge code against new expectations.
        if rebuild or not (BIN / name).is_file():
            print('building', name, flush=True)
            subprocess.run(['go', 'build', '-p', '2', '-o', str(BIN / name), package], cwd=str(ROOT / 'server'), check=True)


def prove(spec_path):
    """The whole issue-started proof in one step: up, bootstrap, one issue, wait, read the five surfaces, down."""
    started = now()
    # The lane refuses to admit a run while another run's worktree exists, so
    # starting the instance would only end in a host refusal thirty seconds in.
    busy = sorted(str(child) for child in WORKTREE_PARENT.iterdir()) if WORKTREE_PARENT.is_dir() else []
    if busy:
        raise SystemExit('another implementation-lane run is active or left a worktree; '
                         'wait for it to finish, then run prove again: ' + ', '.join(busy))
    ensure_binaries(rebuild=True)
    try:
        up()
        state, token, ws = bootstrap()
        issue_id = create_and_launch(state, token, ws, spec_path, 'Validate the agent list response (lane proof)')
        print('launched issue', issue_id, flush=True)
        result = wait_for_outcome(issue_id, token, ws)
        surfaces = inspect(issue_id, token, ws)
        save('proof-result.json', {'started_at': started, 'finished_at': now(), 'issue_id': issue_id,
                                   'result': result, 'all_five_surfaces_agree_done': surfaces['all_five_surfaces_agree_done'],
                                   'bridge_state': str(STATE / 'bridge')})
        print('PROOF RESULT:', result, '| five surfaces agree done:', surfaces['all_five_surfaces_agree_done'])
    finally:
        down()


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else ''
    if mode == 'up':
        up()
    elif mode == 'bootstrap':
        bootstrap()
    elif mode == 'prove':
        prove(sys.argv[2] if len(sys.argv) > 2 else str(LANE / 'tasks' / 'issue-proof-list-agents.json'))
    elif mode == 'inspect':
        inspect(sys.argv[2])
    elif mode == 'down':
        down()
    else:
        raise SystemExit(__doc__)


if __name__ == '__main__':
    main()
