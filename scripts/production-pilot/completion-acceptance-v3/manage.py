"""Host-local SILV-38 operations. Secrets stay in memory or private temp profiles."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time
import uuid
from datetime import datetime, timedelta, timezone
from urllib.request import Request, build_opener, HTTPRedirectHandler
from urllib.error import HTTPError

ROOT = Path(__file__).resolve().parents[3]
STATE = ROOT / '.qualification-evidence/production-pilot-SILV-38-completion-acceptance-v3'
PROFILE = Path('C:/Users/a9sho/.multica/profiles/silv38-production-pilot-completion-acceptance-v3-20260916')
OWNER = '6315deec-17ef-4f66-8885-ddc278c52bba'
WS = '4e3cdf12-463c-4834-9291-c5ff2e422663'
ISSUE = json.loads((STATE/'acceptance-issue.json').read_text(encoding='utf-8'))['id'] if (STATE/'acceptance-issue.json').exists() else None
CONTROLLER = 'c43dc900-4554-4cbb-a664-759067b81118'
DAEMON = 'silv38-production-pilot-completion-acceptance-v3-20260916'
IMAGE = json.loads((STATE/'release-identity.json').read_text(encoding='utf-8'))['backend_image'] if (STATE/'release-identity.json').exists() else None
BASE = 'http://127.0.0.1:8080'
PROVIDER = Path('C:/Users/a9sho/AppData/Roaming/npm/node_modules/@anthropic-ai/claude-code/bin/claude.exe')
BROKER = ROOT / 'server/bin/silv38-completion-acceptance-v3-broker.exe'
CLI = ROOT / 'server/bin/completion-acceptance-v3-multica.exe'
TITLE = 'Verify reviewed handoff publication with the bounded summary sentence'
CANDIDATE_SHA256 = '2df9d83d52cf882f7b399b4b823d6d0cc975d41be6856b8838b276d6f494541c'
SOURCE_REPORTS = Path('C:/Users/a9sho/.codex/worktrees/3228/SMK-multica-selfhost-pilot/docs/silver-mediums/pilots/SILV-38/manual-review-final')
REPORTS = ROOT/'docs/silver-mediums/pilots/SILV-38/completion-acceptance-v3'

def command(args, **kw):
    r = subprocess.run(args, capture_output=True, **kw)
    if r.returncode:
        raise RuntimeError('Local operation failed: '+str(args[0])+' (details withheld to protect credentials)')
    return r.stdout

_POWERSHELL7 = None

def powershell7():
    """Resolve the real PowerShell 7 executable used by guarded cleanup.

    Cleanup must not depend on `pwsh` being on PATH. Candidates are tried in a
    fixed order and each one is confirmed by asking the executable for its own
    major version, so Windows PowerShell 5.1 can never silently satisfy this.
    A zero-length candidate is skipped: a Microsoft Store app-execution alias
    satisfies is_file() and then opens the Store instead of answering.
    """
    global _POWERSHELL7
    if _POWERSHELL7:return _POWERSHELL7
    def usable(path):
        try:
            if not path.is_file() or path.stat().st_size==0:return False
            probe=subprocess.run([str(path),'-NoProfile','-NonInteractive','-Command','$PSVersionTable.PSVersion.Major'],capture_output=True,text=True,timeout=120)
        except (OSError,subprocess.SubprocessError):
            return False
        major=probe.stdout.strip()
        return not probe.returncode and major.isdigit() and int(major)>=7
    # An explicit override that does not answer is an operator mistake, not a
    # hint to look elsewhere.
    override=os.environ.get('MULTICA_POWERSHELL7')
    if override:
        if not usable(Path(override)):
            raise RuntimeError('MULTICA_POWERSHELL7 is set to '+override+', which is not a working PowerShell 7 executable.')
        _POWERSHELL7=override
        return override
    searched=[]
    def candidates():
        for name in ('pwsh','pwsh.exe'):
            found=shutil.which(name)
            if found:yield Path(found)
        for root in [os.environ.get(name) for name in ('ProgramFiles','ProgramW6432','ProgramFiles(x86)','LOCALAPPDATA','ProgramData')]:
            if not root:continue
            for pattern in ('PowerShell/*/pwsh.exe','Microsoft/PowerShell/*/pwsh.exe','Microsoft/WindowsApps/pwsh.exe','chocolatey/bin/pwsh.exe'):
                yield from sorted(Path(root).glob(pattern),reverse=True)
        yield from sorted(Path.home().glob('.cache/codex-runtimes/*/dependencies/native/powershell/pwsh.exe'))
        yield from sorted(Path.home().glob('.dotnet/tools/pwsh.exe'))
    for candidate in candidates():
        text=str(candidate)
        if text in searched:continue
        searched.append(text)
        if not usable(Path(text)):continue
        _POWERSHELL7=text
        return text
    raise RuntimeError('PowerShell 7 is required for guarded cleanup and was not found. Searched: '+'; '.join(searched or ['no candidate path'])+'. Set MULTICA_POWERSHELL7 to the exact pwsh.exe.')

def private_dir(path):
    if path.is_symlink() or path.is_junction():
        raise RuntimeError('Private directory cannot be a link')
    path.mkdir(parents=True, exist_ok=True)
    who = command(['whoami'], text=True).strip()
    command(['icacls', str(path), '/inheritance:r', '/grant:r', who+':(OI)(CI)F'], text=True)

def save(name, value):
    target=STATE/name
    temporary=target.with_suffix(target.suffix+'.tmp')
    temporary.write_text(json.dumps(value, indent=2)+'\n', encoding='utf-8')
    os.replace(temporary,target)

def journal():
    path=STATE/'setup-journal.json'
    if path.exists():return json.loads(path.read_text())
    value={'issue_id':ISSUE,'daemon_id':DAEMON,'started_at':datetime.now(timezone.utc).isoformat(),'roles':[]}
    save('setup-journal.json',value)
    return value

def checkpoint(value, **changes):
    value.update(changes)
    save('setup-journal.json',value)

def one_id(sql):
    rows=query(sql).splitlines()
    if len(rows)>1:raise RuntimeError('Ambiguous pilot resource identity')
    return str(uuid.UUID(rows[0])) if rows else None

def pilot_runtime(require_online=False):
    raw=query(f"SELECT coalesce(json_agg(json_build_object('id',id,'provider',provider,'owner_id',owner_id,'visibility',visibility,'status',status)),'[]') FROM agent_runtime WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}'")
    rows=json.loads(raw)
    if len(rows)!=1 or rows[0]['provider']!='claude' or rows[0]['owner_id']!=OWNER or rows[0]['visibility']!='private':
        raise RuntimeError('Expected exactly one owner-bound private Claude runtime')
    if require_online and rows[0]['status']!='online':raise RuntimeError('Pilot runtime is not online')
    return rows[0]

def query(sql):
    return command(['docker','exec','-i','multica-postgres-1','psql','-U','multica','-d','multica','-Atq','-v','ON_ERROR_STOP=1','-f','-'],input=sql,text=True).strip()

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):
        raise RuntimeError('Redirect refused')

def api(method,path,body=None,token=None):
    headers={'Content-Type':'application/json','X-Workspace-ID':WS}
    if token:headers['Authorization']='Bearer '+token
    data=None if body is None else json.dumps(body).encode()
    try:
        with build_opener(NoRedirect).open(Request(BASE+path,data=data,headers=headers,method=method),timeout=30) as r:
            raw=r.read(2<<20)
    except HTTPError as e:
        # Safe application errors have no request credentials. Keep bounded text.
        raw=e.read(500).decode(errors='replace')
        raise RuntimeError(f'{method} {path}: HTTP {e.code}: {raw}') from None
    return json.loads(raw) if raw.strip() else None

def owner_session():
    file=STATE/'owner-session.json'
    if file.exists():return json.loads(file.read_text())['token']
    inspect=json.loads(command(['docker','inspect','multica-backend-1'],text=True))[0]
    env=dict(x.split('=',1) for x in inspect['Config']['Env'])
    if any(env.get(k) for k in ['RESEND_API_KEY','SMTP_HOST','SMTP_PASSWORD']):raise RuntimeError('Unexpected mail route; no code request issued')
    email='turtlesnshit02@outlook.com'
    started=datetime.now(timezone.utc).isoformat()
    api('POST','/auth/send-code',{'email':email})
    logs=subprocess.run(['docker','logs','--since',started,'multica-backend-1'],capture_output=True,text=True)
    matches=re.findall(r'\[DEV\] Verification code for '+re.escape(email)+r': ([0-9]{6})',logs.stdout+logs.stderr)
    if len(matches)!=1:raise RuntimeError('Normal local sign-in code is unavailable')
    login=api('POST','/auth/verify-code',{'email':email,'code':matches[0]})
    token=login['token']
    # Reuse only this normal short-lived session; no JWT minting or browser extraction.
    save('owner-session.json',{'token':token})
    return token

def exact_host():
    if str(ROOT)!=r'C:\Users\a9sho\.codex\worktrees\completion-20260915\SMK-multica-selfhost-pilot':raise RuntimeError('Wrong task root')
    inspect=json.loads(command(['docker','inspect','multica-backend-1'],text=True))[0]
    env=dict(x.split('=',1) for x in inspect['Config']['Env'])
    if inspect['Image']!=IMAGE or env.get('MULTICA_WORKFLOW_CONTROLLER_ID')!=CONTROLLER:raise RuntimeError('Wrong reviewed controller')
    release=json.loads((STATE/'release-identity.json').read_text(encoding='utf-8'))
    if api('GET','/health')['commit']!=release['source_id']:raise RuntimeError('Wrong source identity')
    qualify()

def qualify():
    receipt=json.loads((STATE/'qualification.json').read_text(encoding='utf-8'))
    if receipt.get('status')!='reviewed_and_tested' or not re.fullmatch(r'[0-9a-f]{64}',receipt.get('review_sha256','')):raise RuntimeError('Reviewed provider-free qualification is required')
    expected={str(p.relative_to(ROOT)).replace('\\','/') for p in (ROOT/'scripts/production-pilot/completion-acceptance-v3').rglob('*') if p.is_file() and p.suffix in {'.py','.go','.md'}}
    expected.add('server/pkg/brokerreply/reply.go')
    if set(receipt['source_hashes'])!=expected:raise RuntimeError('Qualification source scope changed')
    for relative,digest in receipt['source_hashes'].items():
        if sha(ROOT/relative)!=digest:raise RuntimeError('Reviewed acceptance source drift')

def prepare_resources():
    global ISSUE
    exact_host();token=owner_session();state=journal()
    if query("SELECT count(*) FROM agent_task_queue WHERE status NOT IN ('completed','failed','cancelled')")!='0':raise RuntimeError('Other work is active')
    if ISSUE is None:
        marker=state.get('creation_marker') or str(uuid.uuid4())
        checkpoint(state,creation_marker=marker,pending='new_acceptance_issue')
        description='One bounded publication acceptance using the unchanged independently reviewed SILV-38 handoff. Dated captured evidence only; no fresh CRM access. Candidate SHA256: '+CANDIDATE_SHA256+'. One publisher and one reviewer call; no correction or broad dispatch. Acceptance marker: '+marker
        existing=one_id(f"SELECT id FROM issue WHERE workspace_id='{WS}' AND creator_id='{OWNER}' AND title='{TITLE}' AND description LIKE '%Acceptance marker: {marker}'")
        issue={'id':existing} if existing else api('POST','/api/issues/',{'workspace_id':WS,'title':TITLE,'description':description,'status':'backlog','priority':'none'},token)
        ISSUE=str(uuid.UUID(issue['id']));save('acceptance-issue.json',{'id':ISSUE,'title':TITLE,'creation_marker':marker})
        checkpoint(state,issue_id=ISSUE,pending=None)
    if query(f"SELECT count(*) FROM agent_runtime WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}'")=='0':
        api('POST','/api/daemon/register',{'workspace_id':WS,'daemon_id':DAEMON,'device_name':'Alienware publication acceptance','cli_version':'task-bound-source-build','runtimes':[{'name':'Publication acceptance Claude','type':'claude','version':'2.1.272','status':'offline'}]},token)
    runtime=pilot_runtime();checkpoint(state,runtime_id=runtime['id'],runtime_owner_id=runtime['owner_id'])
    REPORTS.mkdir(parents=True,exist_ok=True)
    for name in ['CRM-HANDOFF-EVIDENCE.json','CRM-ENGINEERING-HANDOFF-v2.md']:
        source=SOURCE_REPORTS/name;target=REPORTS/name
        if name.endswith('.md') and sha(source)!=CANDIDATE_SHA256:raise RuntimeError('Original accepted candidate drift')
        if target.exists():
            if sha(target)!=sha(source):raise RuntimeError('Preserved acceptance evidence drift')
        else:shutil.copyfile(source,target)
    stage_issue()

def build_bound():
    qualify()
    verify_local_build_source()
    if not ISSUE:raise RuntimeError('A new native acceptance issue must be staged first')
    c=json.loads((STATE/'authority-contract.json').read_text())
    roles=[str(uuid.UUID(r['id'])) for r in c['roles']]
    if len(set(roles))!=3:raise RuntimeError('Exact role bindings missing')
    broker_dir=ROOT/'scripts/production-pilot/completion-acceptance-v3/broker'
    sources=[str(broker_dir/name) for name in ['main.go','api.go','inventory.go','transport.go']]
    BROKER.parent.mkdir(exist_ok=True)
    command(['go','build','-trimpath','-ldflags',f'-X main.pilotIssueID={ISSUE} -X main.suppressedAgentIDs={",".join(roles)}','-o',str(BROKER),*sources],cwd=ROOT/'server')
    command(['go','build','-trimpath','-o',str(CLI),'./cmd/multica'],cwd=ROOT/'server')
    # Internal Go imports require the helper to reside beneath the server module.
    staging=ROOT/'server/.qualification-evidence/completion-acceptance-v3-authority';staging.mkdir(parents=True,exist_ok=True)
    helper_source=ROOT/'scripts/production-pilot/completion-acceptance-v3/authority/main.go'
    staged=staging/'main.go';shutil.copyfile(helper_source,staged)
    if sha(staged)!=sha(helper_source):raise RuntimeError('Authority staging drift')
    build_env=os.environ.copy();build_env.update({'GOOS':'linux','GOARCH':'amd64','CGO_ENABLED':'0'})
    helper=ROOT/'server/bin/silv38-completion-acceptance-v3-authority'
    command(['go','build','-trimpath','-ldflags',f'-X main.issueID={ISSUE}','-o',str(helper),str(staged)],cwd=ROOT/'server',env=build_env)
    verify_local_build_source()
    c.update(broker_sha256=sha(BROKER),provider_sha256=sha(PROVIDER),evidence_id='silv38-provider-free-review-sha256.'+sha(STATE/'qualification.json'))
    save('authority-contract.json',c)
    save('bound-binaries.json',{'issue_id':ISSUE,'roles':roles,'broker_sha256':sha(BROKER),'cli_sha256':sha(CLI),'authority_sha256':sha(helper),'qualification_sha256':sha(STATE/'qualification.json'),'provider_started':False})
    print('Exact issue and three role IDs bound into reviewed binaries; no provider contact.')

def verify_local_build_source():
    source=json.loads(command([sys.executable,str(ROOT/'scripts/local-candidate-source.py'),'--check'],cwd=ROOT,text=True))
    release=json.loads((STATE/'release-identity.json').read_text(encoding='utf-8'))
    if source['source_id']!=release['source_id']:raise RuntimeError('Local CLI source differs from reviewed release')

def check_bound_binaries():
    qualify()
    bound=json.loads((STATE/'bound-binaries.json').read_text(encoding='utf-8'))
    contract=json.loads((STATE/'authority-contract.json').read_text(encoding='utf-8'))
    if bound['issue_id']!=ISSUE or bound['roles']!=[r['id'] for r in contract['roles']] or bound['qualification_sha256']!=sha(STATE/'qualification.json'):raise RuntimeError('Reviewed binary binding differs')
    for path,key in [(BROKER,'broker_sha256'),(CLI,'cli_sha256'),(ROOT/'server/bin/silv38-completion-acceptance-v3-authority','authority_sha256')]:
        if sha(path)!=bound[key]:raise RuntimeError('Reviewed bound executable drift: '+key)
    if bound['broker_sha256']!=contract['broker_sha256'] or sha(PROVIDER)!=contract['provider_sha256']:raise RuntimeError('Authority executable binding differs')

def sha(path):
    if path.is_symlink() or not path.is_file():raise RuntimeError('Expected a regular executable')
    return hashlib.sha256(path.read_bytes()).hexdigest()

def provision_daemon():
    exact_host();check_bound_binaries()
    if query("SELECT count(*) FROM agent_task_queue WHERE status NOT IN ('completed','failed','cancelled')")!='0':raise RuntimeError('Other work is active')
    state=journal()
    token=owner_session()
    if 'original_issue' not in state:
        issue=api('GET','/api/issues/'+ISSUE,token=token)
        checkpoint(state,original_issue={key:issue.get(key) for key in ['status','assignee_type','assignee_id']})
    # A normal owner session establishes private ownership. Offline is truthful:
    # the real native daemon will register online using its scoped daemon token.
    existing=query(f"SELECT count(*) FROM agent_runtime WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}'")
    if existing=='0':
        checkpoint(state,pending='owner_offline_runtime_registration')
        api('POST','/api/daemon/register',{'workspace_id':WS,'daemon_id':DAEMON,'device_name':'Alienware SILV-38','cli_version':'task-bound-source-build','runtimes':[{'name':'SILV-38 bounded Claude','type':'claude','version':'2.1.271','status':'offline'}]},token)
    runtime=pilot_runtime()
    checkpoint(state,runtime_id=runtime['id'],runtime_owner_id=runtime['owner_id'],pending='scoped_daemon_token')
    private_dir(PROFILE)
    config_path=PROFILE/'config.json'
    identity_path=STATE/'daemon-identity.json'
    if identity_path.exists():identity=json.loads(identity_path.read_text())
    else:
        identity={'daemon_id':DAEMON,'expires_at':(datetime.now(timezone.utc)+timedelta(hours=4)).isoformat(),'profile':str(PROFILE),'runtime_id':runtime['id'],'owner_id':OWNER,'cli_sha256':sha(CLI),'broker_sha256':sha(BROKER),'provider_sha256':sha(PROVIDER)}
        save('daemon-identity.json',identity)
    if datetime.fromisoformat(identity['expires_at'])<=datetime.now(timezone.utc):raise RuntimeError('Pilot identity expired; cleanup is required')
    if config_path.exists():
        config=json.loads(config_path.read_text())
        if config.get('workspace_id')!=WS or config.get('server_url')!=BASE:raise RuntimeError('Existing pilot config differs')
        daemon_token=config['token']
    else:
        daemon_token='mdt_'+secrets.token_hex(20)
        config_path.write_text(json.dumps({'server_url':BASE,'workspace_id':WS,'token':daemon_token}),encoding='utf-8')
    token_hash=hashlib.sha256(daemon_token.encode()).hexdigest()
    count=query(f"SELECT count(*) FROM daemon_token WHERE daemon_id='{DAEMON}'")
    if count=='0':
        query(f"INSERT INTO daemon_token(id,token_hash,workspace_id,daemon_id,expires_at) VALUES(gen_random_uuid(),'{token_hash}','{WS}','{DAEMON}','{identity['expires_at']}'::timestamptz);")
    elif query(f"SELECT count(*) FROM daemon_token WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}' AND token_hash='{token_hash}'")!='1':
        raise RuntimeError('Scoped daemon identity differs; cleanup is required')
    checkpoint(state,pending=None,daemon_identity_ready=True)
    print('Task-local workspace daemon identity provisioned; provider credential unopened.')

def start_daemon():
    exact_host();check_bound_binaries(); identity=json.loads((STATE/'daemon-identity.json').read_text())
    if sha(CLI)!=identity['cli_sha256'] or sha(BROKER)!=identity['broker_sha256']:raise RuntimeError('Executable drift')
    if (STATE/'daemon-process.json').exists():raise RuntimeError('Daemon start already recorded')
    env={k:v for k,v in os.environ.items() if k.upper() in {'SYSTEMROOT','WINDIR','COMSPEC','PATH','PATHEXT','TEMP','TMP','USERPROFILE','APPDATA','LOCALAPPDATA','PROGRAMDATA'}}
    env.update({'MULTICA_HOST_ATTESTATION':'alienware_windows','MULTICA_CLAUDE_PATH':str(BROKER),'MULTICA_CLAUDE_MODEL':'claude-sonnet-4-6'})
    old=(ROOT/'scripts/item14-integration-replay-exact-launch.ps1').read_text()
    for key in set(re.findall(r"'((?:MULTICA_)[A-Z0-9]+_PATH)'",old)):
        if key!='MULTICA_CLAUDE_PATH':env[key]=str(STATE/'unavailable-provider.exe')
    if (STATE/'unavailable-provider.exe').exists():raise RuntimeError('Provider sentinel exists')
    work=STATE/'empty-daemon-workspaces';work.mkdir(exist_ok=True)
    args=[str(CLI),'daemon','start','--foreground','--daemon-id',DAEMON,'--device-name','Alienware SILV-38','--runtime-name','SILV-38 bounded Claude','--workspaces-root',str(work),'--agent-timeout','20m0s','--max-concurrent-tasks','1','--no-auto-update','--no-auto-reload','--profile',DAEMON]
    with (STATE/'daemon.log').open('wb') as log:
        p=subprocess.Popen(args,env=env,cwd=str(work),stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NO_WINDOW)
    save('daemon-process.json',{'pid':p.pid,'started_at':datetime.now(timezone.utc).isoformat(),'executable':str(CLI)})
    print('Isolated bounded daemon started; provider profile is still absent.')

def stage_issue():
    exact_host();token=owner_session()
    state=journal()
    if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}'")!='0':raise RuntimeError('Issue dispatch already occurred; staging is closed')
    runtime_id=pilot_runtime(require_online=False)['id']
    if state.get('runtime_id')!=runtime_id or state.get('runtime_owner_id')!=OWNER:raise RuntimeError('Pilot runtime differs from provisioned identity')
    roles=[]
    for label,name,kinds in [('planner','CRM Handoff Planner SILV-38 PublicationAcceptanceV3',['leader']),('writer','CRM Handoff Writer SILV-38 PublicationAcceptanceV3',['implementation']),('reviewer','CRM Handoff Reviewer SILV-38 PublicationAcceptanceV3',['review'])]:
        checkpoint(state,pending='agent_'+label)
        agent_id=one_id(f"SELECT id FROM agent WHERE workspace_id='{WS}' AND owner_id='{OWNER}' AND name='{name}' AND archived_at IS NULL")
        if not agent_id:
            agent=api('POST','/api/agents/',{'name':name,'description':'Temporary CRM handoff role for SILV-38; expires at pilot closure.','runtime_id':runtime_id,'instructions':'','runtime_config':{},'custom_args':[],'mcp_config':{},'permission_mode':'private','invocation_targets':[],'max_concurrent_tasks':1,'model':'claude-sonnet-4-6','thinking_level':'medium','composio_toolkit_allowlist':[]},token)
            agent_id=str(uuid.UUID(agent['id']))
        roles.append({'id':agent_id,'name':name,'kinds':kinds,'label':label})
        checkpoint(state,roles=roles)
    checkpoint(state,pending='squad')
    squad_id=one_id(f"SELECT id FROM squad WHERE workspace_id='{WS}' AND name='CRM Handoff SILV-38 PublicationAcceptanceV3'")
    if not squad_id:
        squad=api('POST','/api/squads/',{'name':'CRM Handoff SILV-38 PublicationAcceptanceV3','description':'One controlled manual read-only CRM handoff.','leader_id':roles[0]['id']},token)
        squad_id=str(uuid.UUID(squad['id']))
    checkpoint(state,squad_id=squad_id)
    for role in roles[1:]:
        expected='implementer' if role['label']=='writer' else 'reviewer'
        checkpoint(state,pending='squad_member_'+role['label'])
        existing=one_id(f"SELECT id FROM squad_member WHERE squad_id='{squad_id}' AND member_type='agent' AND member_id='{role['id']}'")
        if not existing:api('POST','/api/squads/'+squad_id+'/members',{'member_type':'agent','member_id':role['id'],'role':expected},token)
    plan={'implementer_id':roles[1]['id'],'reviewer_id':roles[2]['id'],'requires_owner':False,'requires_deployment':False,'max_corrections':1}
    checkpoint(state,pending='workflow_template')
    api('PUT','/api/squads/'+squad_id+'/workflow-template',plan,token)
    checkpoint(state,pending='park_issue')
    api('PUT','/api/issues/'+ISSUE,{'status':'backlog','assignee_type':'squad','assignee_id':squad_id},token)
    checkpoint(state,issue_parked=True,pending='workflow_plan')
    api('PUT','/api/issues/'+ISSUE+'/workflow-plan',{'squad_id':squad_id,**{k:v for k,v in plan.items() if k!='max_corrections'}},token)
    if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}'")!='0':raise RuntimeError('Parked staging unexpectedly created a task')
    now=datetime.now(timezone.utc)
    contract={'runtime_id':runtime_id,'squad_id':squad_id,'broker_path':str(BROKER),'broker_sha256':None,'provider_path':str(PROVIDER),'provider_sha256':None,'provider_profile_path':str(STATE/'provider-home'),'evidence_id':'silv38-provider-free-review-pending','issued_at':now.isoformat(),'expires_at':(now+timedelta(hours=3)).isoformat(),'roles':roles}
    if not (STATE/'authority-contract.json').exists():save('authority-contract.json',contract)
    checkpoint(state,pending=None,staged=True)
    print('New publication acceptance staged in Backlog: three private roles, offline runtime, zero tasks and provider calls.')

def authority(mode):
    exact_host();check_bound_binaries()
    c=json.loads((STATE/'authority-contract.json').read_text())
    if sha(BROKER)!=c['broker_sha256'] or sha(PROVIDER)!=c['provider_sha256']:raise RuntimeError('Receipt executable drift')
    helper=ROOT/'server/bin/silv38-completion-acceptance-v3-authority'
    command(['docker','cp',str(helper),'multica-backend-1:/tmp/silv38-completion-acceptance-v3-authority'])
    command(['docker','exec','--user','root','multica-backend-1','chmod','0555','/tmp/silv38-completion-acceptance-v3-authority'])
    try:
        result=subprocess.run(['docker','exec','-i','multica-backend-1','/tmp/silv38-completion-acceptance-v3-authority',mode],input=json.dumps(c),capture_output=True,text=True)
        if result.returncode:raise RuntimeError('Authority operation rejected: '+result.stderr.strip())
        print(result.stdout.strip())
    finally:
        command(['docker','exec','--user','root','multica-backend-1','rm','-f','/tmp/silv38-completion-acceptance-v3-authority'])

def prepare_provider():
    exact_host();check_bound_binaries();c=json.loads((STATE/'authority-contract.json').read_text())
    if sha(PROVIDER)!=c['provider_sha256'] or sha(BROKER)!=c['broker_sha256']:raise RuntimeError('Executable drift')
    active=query(f"SELECT count(*) FROM qualified_runtime_binding_receipt q WHERE workspace_id='{WS}' AND issue_id='{ISSUE}' AND issued_by_controller_id='{CONTROLLER}' AND expires_at>now() AND NOT EXISTS(SELECT 1 FROM qualified_runtime_binding_revocation r WHERE r.receipt_id=q.id)")
    if active!='3':raise RuntimeError('Fresh exact runtime receipts are required before credential custody')
    home=STATE/'provider-home';target=home/'.claude/.credentials.json'
    if home.exists():raise RuntimeError('Provider custody was already started')
    source=Path('C:/Users/a9sho/.claude/.credentials.json')
    if not source.is_file() or source.is_symlink():raise RuntimeError('Existing approved Claude subscription credential source unavailable')
    private_dir(home);(home/'.claude').mkdir()
    # Opaque copy only. Do not parse, print or hash credential bytes.
    shutil.copyfile(source,target)
    save('provider-custody.json',{'copied_at':datetime.now(timezone.utc).isoformat(),'source':'existing Claude Code subscription file','scope':ISSUE,'credential_bytes_exposed':False,'cleanup':'pending'})
    print('Existing Claude subscription copied opaquely to the private task profile; no provider call.')

def launch_publication():
    exact_host();check_bound_binaries();c=json.loads((STATE/'authority-contract.json').read_text())
    if not ISSUE or not (STATE/'provider-home/.claude/.credentials.json').is_file():raise RuntimeError('Exact issue and scoped provider custody are required')
    if datetime.fromisoformat(c['expires_at'])<=datetime.now(timezone.utc):raise RuntimeError('Authority expired')
    if sha(BROKER)!=c['broker_sha256'] or sha(PROVIDER)!=c['provider_sha256']:raise RuntimeError('Reviewed executable drift')
    pilot_runtime(require_online=True)
    if (STATE/'launch-intent.json').exists() or list(STATE.glob('*.consumed.json')):raise RuntimeError('Launch already attempted; inspect existing evidence, never retry a consumed slot')
    if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}'")!='0':raise RuntimeError('Acceptance already has native tasks')
    writer=c['roles'][1];suppress=[c['roles'][0]['id'],c['roles'][2]['id']]
    content='Owner-authorized bounded publication check. Publish the unchanged, previously reviewed SILV-38 candidate SHA256 '+CANDIDATE_SHA256+' using only its dated captured evidence. Then the native workflow should route one independent review. No fresh CRM work, document rewrite, correction or additional provider slot. [@'+writer['name']+'](mention://agent/'+writer['id']+') Please verify and publish the preserved candidate.'
    save('launch-intent.json',{'issue_id':ISSUE,'writer_id':writer['id'],'at':datetime.now(timezone.utc).isoformat(),'maximum_provider_calls':2})
    result=api('POST','/api/issues/'+ISSUE+'/comments',{'content':content,'suppress_agent_ids':suppress},owner_session())
    save('launch-response.json',result)
    print('One native publisher mention submitted; inspect its task and automatic review transition.')

def stop_pilot_process():
    path=STATE/'daemon-process.json'
    if not path.exists():return
    process=json.loads(path.read_text())
    ps="""$ErrorActionPreference='Stop'
$pilotProcess=Get-CimInstance Win32_Process -Filter ('ProcessId='+$env:SILV38_PID)
if($null -ne $pilotProcess){
 if($pilotProcess.ExecutablePath -cne $env:SILV38_EXE){throw 'PID identity changed'}
 if($pilotProcess.CommandLine -notlike '*silv38-production-pilot-completion-acceptance-v3-20260916*'){throw 'Daemon task identity changed'}
 & taskkill /PID $env:SILV38_PID /T /F | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Pilot stop failed'}
}
"""
    env=os.environ.copy();env.update({'SILV38_PID':str(process['pid']),'SILV38_EXE':process['executable']})
    command([powershell7(),'-NoProfile','-NonInteractive','-Command',ps],env=env,text=True)

def remove_provider_profile():
    home=STATE/'provider-home'
    # Inspect the literal path before resolving, so a junction cannot hide its target.
    if home.is_symlink() or home.is_junction() or STATE.is_symlink() or STATE.is_junction():raise RuntimeError('Provider cleanup target is a link')
    resolved=home.resolve()
    if resolved!=home.absolute() or not resolved.is_relative_to(STATE.resolve()) or resolved.parent!=STATE.resolve():raise RuntimeError('Provider cleanup target is outside this task')
    if home.exists():
        # Same API end-to-end, after checking the exact absolute deletion target.
        shutil.rmtree(home)

def cleanup():
    state=journal();errors=[];completed=[]
    def attempt(name, action):
        try:
            result=action();completed.append(name);return result
        except Exception as e:
            errors.append({'step':name,'error':str(e)})
            return None
    attempt('reviewed_host',exact_host)
    active=attempt('active_task_readback',lambda:query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}' AND status NOT IN ('completed','failed','cancelled')"))
    if active not in ('0',None):raise RuntimeError('Wait for or explicitly cancel the exact active pilot task before cleanup')
    contract_path=STATE/'authority-contract.json'
    if contract_path.exists():attempt('runtime_receipts_revoked',lambda:authority('revoke'))
    attempt('daemon_process_stopped',stop_pilot_process)
    config_path=PROFILE/'config.json'
    if config_path.exists() and state.get('runtime_id'):
        def deregister():
            config=json.loads(config_path.read_text())
            api('POST','/api/daemon/deregister',{'runtime_ids':[state['runtime_id']]},config['token'])
        attempt('runtime_offline',deregister)
    attempt('daemon_token_deleted',lambda:query(f"DELETE FROM daemon_token WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}';"))
    # Restore the original assignment only for setup that never dispatched work.
    if state.get('original_issue') and (STATE/'owner-session.json').exists():
        def restore_unlaunched_issue():
            if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}'")=='0':
                token=json.loads((STATE/'owner-session.json').read_text())['token']
                current=api('GET','/api/issues/'+ISSUE,token=token)
                if current.get('assignee_type')=='squad' and current.get('assignee_id')==state.get('squad_id'):
                    api('PUT','/api/issues/'+ISSUE,state['original_issue'],token)
        attempt('unlaunched_issue_restored',restore_unlaunched_issue)
    if state.get('roles') and (STATE/'owner-session.json').exists():
        def archive_roles():
            token=json.loads((STATE/'owner-session.json').read_text())['token']
            for role in state['roles']:
                if query(f"SELECT count(*) FROM agent WHERE id='{role['id']}' AND workspace_id='{WS}' AND archived_at IS NULL")=='1':api('POST','/api/agents/'+role['id']+'/archive',{},token)
        attempt('temporary_agents_archived',archive_roles)
    # These independent steps always run, even if server cleanup failed.
    attempt('daemon_config_removed',lambda:config_path.unlink(missing_ok=True))
    attempt('owner_session_removed',lambda:(STATE/'owner-session.json').unlink(missing_ok=True))
    attempt('provider_profile_removed',remove_provider_profile)
    remaining=attempt('daemon_token_readback',lambda:query(f"SELECT count(*) FROM daemon_token WHERE workspace_id='{WS}' AND daemon_id='{DAEMON}'"))
    result={'verified_at':datetime.now(timezone.utc).isoformat(),'status':'verified' if not errors else 'incomplete','completed_steps':completed,'errors':errors,'daemon_token_absent':remaining=='0','daemon_config_absent':not config_path.exists(),'owner_session_file_absent':not (STATE/'owner-session.json').exists(),'provider_profile_absent':not (STATE/'provider-home').exists(),'daemon_process_stopped':'daemon_process_stopped' in completed}
    save('cleanup.json',result)
    if errors or not all(result[k] for k in ['daemon_token_absent','daemon_config_absent','owner_session_file_absent','provider_profile_absent','daemon_process_stopped']):raise RuntimeError('Pilot cleanup is incomplete; independent local cleanup ran. See cleanup.json for exact remaining steps.')
    print('Temporary runtime authority revoked; pilot daemon stopped; temporary credential profiles removed.')

def main():
    parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['prepare-resources','build-bound','provision-daemon','start-daemon','stage-issue','prepare-authority','issue-authority','prepare-provider','launch-publication','cleanup']);args=parser.parse_args()
    private_dir(STATE)
    {'prepare-resources':prepare_resources,'build-bound':build_bound,'provision-daemon':provision_daemon,'start-daemon':start_daemon,'stage-issue':stage_issue,'prepare-authority':lambda:authority('prepare'),'issue-authority':lambda:authority('issue'),'prepare-provider':prepare_provider,'launch-publication':launch_publication,'cleanup':cleanup}[args.mode]()

if __name__=='__main__':
    try:main()
    except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
