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
STATE = ROOT / '.qualification-evidence/production-pilot-SILV-38-manual-review'
PROFILE = Path('C:/Users/a9sho/.multica/profiles/silv38-production-pilot-manual-review-20260915')
OWNER = '6315deec-17ef-4f66-8885-ddc278c52bba'
WS = '4e3cdf12-463c-4834-9291-c5ff2e422663'
ISSUE = '01a0a5fc-a49d-766b-b483-cf85a129c38c'
CONTROLLER = 'c43dc900-4554-4cbb-a664-759067b81118'
DAEMON = 'silv38-production-pilot-manual-review-20260915'
IMAGE = 'sha256:126d388925dec5a4a4b672e88b826c3cc9d59a8691baed9a11623d4b6efb8483'
BASE = 'http://127.0.0.1:8080'
PROVIDER = Path('C:/Users/a9sho/AppData/Roaming/npm/node_modules/@anthropic-ai/claude-code/bin/claude.exe')
BROKER = ROOT / 'server/bin/silv38-manual-review-broker.exe'
CLI = ROOT / 'server/bin/silv38-multica.exe'

def command(args, **kw):
    r = subprocess.run(args, capture_output=True, **kw)
    if r.returncode:
        raise RuntimeError('Local operation failed: '+str(args[0])+' (details withheld to protect credentials)')
    return r.stdout

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
    if str(ROOT)!=r'C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot':raise RuntimeError('Wrong task root')
    inspect=json.loads(command(['docker','inspect','multica-backend-1'],text=True))[0]
    env=dict(x.split('=',1) for x in inspect['Config']['Env'])
    if inspect['Image']!=IMAGE or env.get('MULTICA_WORKFLOW_CONTROLLER_ID')!=CONTROLLER:raise RuntimeError('Wrong reviewed controller')
    if api('GET','/health')['commit']!='60fea34b9770-local-6f0af10d34dd':raise RuntimeError('Wrong source identity')

def sha(path):
    if path.is_symlink() or not path.is_file():raise RuntimeError('Expected a regular executable')
    return hashlib.sha256(path.read_bytes()).hexdigest()

def provision_daemon():
    exact_host()
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
    exact_host(); identity=json.loads((STATE/'daemon-identity.json').read_text())
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
    if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}' AND id NOT IN ('01a0a634-ff16-700a-b631-0726ac8e2229','01a0a642-564f-7946-a7a2-90bb8183ed47','01a0a647-e6c4-7b38-acf3-23930226f396','01a0a64d-27e3-73c5-9c56-02f90bc84fc3','01a0a658-4e03-7ce3-b651-d2be994b0c87','01a0a663-73d7-7a25-b5d1-f2c886ec0e90')")!='0':raise RuntimeError('Issue dispatch already occurred; staging is closed')
    runtime_id=pilot_runtime(require_online=True)['id']
    if state.get('runtime_id')!=runtime_id or state.get('runtime_owner_id')!=OWNER:raise RuntimeError('Pilot runtime differs from provisioned identity')
    roles=[]
    for label,name,kinds in [('planner','CRM Handoff Planner SILV-38 ManualReview',['leader']),('writer','CRM Handoff Writer SILV-38 ManualReview',['implementation','correction']),('reviewer','CRM Handoff Reviewer SILV-38 ManualReview',['review'])]:
        checkpoint(state,pending='agent_'+label)
        agent_id=one_id(f"SELECT id FROM agent WHERE workspace_id='{WS}' AND owner_id='{OWNER}' AND name='{name}' AND archived_at IS NULL")
        if not agent_id:
            agent=api('POST','/api/agents/',{'name':name,'description':'Temporary CRM handoff role for SILV-38; expires at pilot closure.','runtime_id':runtime_id,'instructions':'','runtime_config':{},'custom_args':[],'mcp_config':{},'permission_mode':'private','invocation_targets':[],'max_concurrent_tasks':1,'model':'claude-sonnet-4-6','thinking_level':'medium','composio_toolkit_allowlist':[]},token)
            agent_id=str(uuid.UUID(agent['id']))
        roles.append({'id':agent_id,'name':name,'kinds':kinds,'label':label})
        checkpoint(state,roles=roles)
    checkpoint(state,pending='squad')
    squad_id=one_id(f"SELECT id FROM squad WHERE workspace_id='{WS}' AND name='CRM Handoff SILV-38 ManualReview'")
    if not squad_id:
        squad=api('POST','/api/squads/',{'name':'CRM Handoff SILV-38 ManualReview','description':'One controlled manual read-only CRM handoff.','leader_id':roles[0]['id']},token)
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
    if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}' AND id NOT IN ('01a0a634-ff16-700a-b631-0726ac8e2229','01a0a642-564f-7946-a7a2-90bb8183ed47','01a0a647-e6c4-7b38-acf3-23930226f396','01a0a64d-27e3-73c5-9c56-02f90bc84fc3','01a0a658-4e03-7ce3-b651-d2be994b0c87','01a0a663-73d7-7a25-b5d1-f2c886ec0e90')")!='0':raise RuntimeError('Parked staging unexpectedly created a task')
    now=datetime.now(timezone.utc)
    contract={'runtime_id':runtime_id,'squad_id':squad_id,'broker_path':str(BROKER),'broker_sha256':sha(BROKER),'provider_path':str(PROVIDER),'provider_sha256':sha(PROVIDER),'provider_profile_path':str(STATE/'provider-home'),'evidence_id':'silv38-provider-free-review-pending','issued_at':now.isoformat(),'expires_at':(now+timedelta(hours=3)).isoformat(),'roles':roles}
    if not (STATE/'authority-contract.json').exists():save('authority-contract.json',contract)
    checkpoint(state,pending=None,staged=True)
    print('SILV-38 staged in Backlog: three private agents, one exact squad, zero tasks.')

def authority(mode):
    exact_host()
    c=json.loads((STATE/'authority-contract.json').read_text())
    if sha(BROKER)!=c['broker_sha256'] or sha(PROVIDER)!=c['provider_sha256']:raise RuntimeError('Receipt executable drift')
    helper=ROOT/'server/bin/silv38-manual-review-authority'
    command(['docker','cp',str(helper),'multica-backend-1:/tmp/silv38-manual-review-authority'])
    command(['docker','exec','--user','root','multica-backend-1','chmod','0555','/tmp/silv38-manual-review-authority'])
    try:
        result=subprocess.run(['docker','exec','-i','multica-backend-1','/tmp/silv38-manual-review-authority',mode],input=json.dumps(c),capture_output=True,text=True)
        if result.returncode:raise RuntimeError('Authority operation rejected: '+result.stderr.strip())
        print(result.stdout.strip())
    finally:
        command(['docker','exec','--user','root','multica-backend-1','rm','-f','/tmp/silv38-manual-review-authority'])

def prepare_provider():
    exact_host();c=json.loads((STATE/'authority-contract.json').read_text())
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

def stop_pilot_process():
    path=STATE/'daemon-process.json'
    if not path.exists():return
    process=json.loads(path.read_text())
    ps="""$ErrorActionPreference='Stop'
$pilotProcess=Get-CimInstance Win32_Process -Filter ('ProcessId='+$env:SILV38_PID)
if($null -ne $pilotProcess){
 if($pilotProcess.ExecutablePath -cne $env:SILV38_EXE){throw 'PID identity changed'}
 if($pilotProcess.CommandLine -notlike '*silv38-production-pilot-manual-review-20260915*'){throw 'Daemon task identity changed'}
 & taskkill /PID $env:SILV38_PID /T /F | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Pilot stop failed'}
}
"""
    env=os.environ.copy();env.update({'SILV38_PID':str(process['pid']),'SILV38_EXE':process['executable']})
    command(['pwsh','-NoProfile','-Command',ps],env=env,text=True)

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
            if query(f"SELECT count(*) FROM agent_task_queue WHERE issue_id='{ISSUE}' AND id NOT IN ('01a0a634-ff16-700a-b631-0726ac8e2229','01a0a642-564f-7946-a7a2-90bb8183ed47','01a0a647-e6c4-7b38-acf3-23930226f396','01a0a64d-27e3-73c5-9c56-02f90bc84fc3','01a0a658-4e03-7ce3-b651-d2be994b0c87','01a0a663-73d7-7a25-b5d1-f2c886ec0e90')")=='0':
                token=json.loads((STATE/'owner-session.json').read_text())['token']
                current=api('GET','/api/issues/'+ISSUE,token=token)
                if current.get('assignee_type')=='squad' and current.get('assignee_id')==state.get('squad_id'):
                    api('PUT','/api/issues/'+ISSUE,state['original_issue'],token)
        attempt('unlaunched_issue_restored',restore_unlaunched_issue)
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
    parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['provision-daemon','start-daemon','stage-issue','prepare-authority','issue-authority','prepare-provider','cleanup']);args=parser.parse_args()
    private_dir(STATE)
    {'provision-daemon':provision_daemon,'start-daemon':start_daemon,'stage-issue':stage_issue,'prepare-authority':lambda:authority('prepare'),'issue-authority':lambda:authority('issue'),'prepare-provider':prepare_provider,'cleanup':cleanup}[args.mode]()

if __name__=='__main__':
    try:main()
    except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
