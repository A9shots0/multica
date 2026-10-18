"""Recover the release artifact into isolated Docker resources and test both images."""
import hashlib
import json
import os
import re
import secrets
import shutil
import subprocess
import tarfile
import time
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / '.qualification-evidence/production-adoption-20260916'
STATE = Path('C:/Dev/SMK-multica-selfhost-pilot/recovery/state')
KEY = Path('C:/Users/a9sho/.multica-recovery/recovery.key')

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


PWSH = powershell7()
IDENTITY = json.loads((ROOT/'deploy/silver-mediums/adoption-20260916/release-identity.json').read_text(encoding='utf-8'))
SOURCE = IDENTITY['source_id']
NEW_BACK = IDENTITY['images']['backend']
NEW_WEB = IDENTITY['images']['frontend']
OLD_BACK = IDENTITY['rollback_images']['backend']
OLD_WEB = IDENTITY['rollback_images']['frontend']

drill = 'app' + secrets.token_hex(5)
database = 'multica_item13_restore_' + drill
network = 'multica_release_recovery_' + drill
EVIDENCE.mkdir(parents=True, exist_ok=True)
stage = EVIDENCE / ('app-recovery-' + drill)
assert stage.resolve().is_relative_to(EVIDENCE.resolve()) and not stage.exists()
stage.mkdir()
created = []
network_id = None
restored = False
test_session = {'jwt_secret': secrets.token_hex(32), 'token': None}
record = {'started_at': datetime.now(timezone.utc).isoformat(), 'source_id': SOURCE,
          'drill_id': drill, 'status': 'in_progress', 'cleanup_verified': False}

def command(args, *, stdin=None, env=None, timeout=240):
    result = subprocess.run(args, cwd=ROOT, input=stdin, capture_output=True,
                            text=True, encoding='utf-8', errors='replace', env=env, timeout=timeout)
    if result.returncode:
        # Do not put process output containing configuration or login material into receipts.
        raise RuntimeError(f'{Path(str(args[0])).name} failed with exit {result.returncode}')
    return result.stdout.strip()

def docker(*args, **kwargs):
    return command(['docker', *args], **kwargs)

def read_db(sql):
    return docker('exec', database, 'psql', '-U', 'multica_restore', '-d', 'multica_restore',
                  '-At', '-v', 'ON_ERROR_STOP=1', '-c', sql)

def facts():
    return json.loads(read_db("SELECT json_build_object('users',(SELECT count(*) FROM \"user\"),'workspaces',(SELECT count(*) FROM workspace),'issues',(SELECT count(*) FROM issue),'comments',(SELECT count(*) FROM comment),'tasks',(SELECT count(*) FROM agent_task_queue),'runnable_tasks',(SELECT count(*) FROM agent_task_queue WHERE status NOT IN ('completed','failed','cancelled')))::text;"))

def remove_container(name, expected_id):
    info = json.loads(docker('inspect', name))[0]
    assert info['Id'] == expected_id and info['Config']['Labels'].get('silver-mediums.recovery') == drill
    docker('rm', '-f', expected_id)

def app_pair(back_image, web_image, label):
    back = database + '_' + label + '_backend'
    web = database + '_' + label + '_web'
    test_env = os.environ.copy()
    db_env = dict(item.split('=', 1) for item in json.loads(docker('inspect', database))[0]['Config']['Env'])
    test_env['DATABASE_URL'] = f"postgres://multica_restore:{db_env['POSTGRES_PASSWORD']}@postgres:5432/multica_restore?sslmode=disable"
    test_env['JWT_SECRET'] = test_session['jwt_secret']
    back_id = docker('run', '-d', '--pull', 'never', '--name', back, '--network', network,
                     '--network-alias', 'backend', '--label', 'silver-mediums.recovery=' + drill,
                     '-v', database + '_uploads:/app/data/uploads', '-e', 'DATABASE_URL', '-e', 'JWT_SECRET',
                     '-e', 'APP_ENV=production', '-e', 'PORT=8080', '-e', 'ALLOW_SIGNUP=false',
                     '-e', 'MULTICA_OWNER_SMS_MODE=off', '-e', 'MULTICA_LLM_MAX_RETRIES=0',
                     '-e', 'MULTICA_VCS_INTEGRATION_ENABLED=false', back_image, env=test_env)
    created.append((back, back_id))
    del test_env, db_env
    ready = None
    for _ in range(60):
        try:
            ready = json.loads(docker('exec', back, 'wget', '-qO-', 'http://127.0.0.1:8080/healthz', timeout=8))
            if ready.get('checks', {}).get('migrations') == 'ok':
                break
        except (RuntimeError, subprocess.TimeoutExpired, json.JSONDecodeError):
            pass
        time.sleep(1)
    assert ready and ready.get('checks', {}).get('migrations') == 'ok', 'Application migrations/readiness failed'
    health = json.loads(docker('exec', back, 'wget', '-qO-', 'http://127.0.0.1:8080/health'))
    if label == 'candidate':
        assert health.get('commit') == SOURCE
    web_id = docker('run', '-d', '--pull', 'never', '--name', web, '--network', network,
                    '--label', 'silver-mediums.recovery=' + drill, '-e', 'HOSTNAME=0.0.0.0',
                    '-e', 'REMOTE_API_URL=http://backend:8080', '-e', 'NEXT_TELEMETRY_DISABLED=1', web_image)
    created.append((web, web_id))
    node = lambda script: docker('exec', '-i', web, 'node', '--input-type=module', stdin=script)
    for _ in range(40):
        try:
            if node("const r=await fetch('http://127.0.0.1:3000/login'); if(!r.ok) process.exit(1); console.log(r.status);") == '200':
                break
        except RuntimeError:
            time.sleep(1)
    else:
        raise RuntimeError('Recovered web application did not become ready')
    owner = read_db('SELECT email FROM "user" ORDER BY id;')
    assert '\n' not in owner and owner, 'Expected one existing owner'
    if test_session['token'] is None:
        body = json.dumps({'email': owner})
        status = int(node("const r=await fetch('http://backend:8080/auth/send-code',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(" + body + ")}); console.log(r.status);"))
        assert status == 200, f'Isolated code request returned HTTP {status}'
        logs = docker('logs', '--since', '30s', back)
        codes = re.findall(r'^\[DEV\] Verification code for ' + re.escape(owner) + r': (\d{6})$', logs, re.M)
        assert len(codes) == 1, 'Expected one isolated one-time code'
        login_body = json.dumps({'email': owner, 'code': codes[0]})
        del codes, logs
    else:
        login_body = json.dumps({'token': test_session['token']})
    # The generated test code and session token stay inside the isolated flow.
    api = json.loads(node("""
const base='http://backend:8080';
const credentials=LOGIN;
let login=credentials;
if(!credentials.token){const loginResponse=await fetch(base+'/auth/verify-code',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(credentials)});if(!loginResponse.ok)throw Error('isolated sign-in failed');login=await loginResponse.json();}
let workspace;
async function get(path){const r=await fetch(base+path,{headers:{Authorization:'Bearer '+login.token,...(workspace?{'X-Workspace-ID':workspace}:{})}});if(!r.ok)throw Error('read failed '+path+' '+r.status);return r.json();}
const workspaces=await get('/api/workspaces'); if(workspaces.length!==1)throw Error('workspace count'); workspace=workspaces[0].id;
const issues=await get('/api/issues?workspace_id='+workspace+'&limit=100');
const known=await get('/api/issues/01a08808-f3e0-7bbe-bbdc-bf7954849af3');
if(known.title!=='Verify local pilot connection')throw Error('existing issue changed');
console.log(JSON.stringify({workspace_count:workspaces.length,issue_count:issues.issues.length,known_issue_id:known.id,known_issue_read:true,session_token:login.token}));
""".replace('LOGIN', login_body)))
    test_session['token'] = api.pop('session_token')
    assert api['issue_count'] == record['before']['issues']
    for name, image in [(back, back_image), (web, web_image)]:
        info = json.loads(docker('inspect', name))[0]
        assert info['Image'] == image and not info['HostConfig']['PortBindings']
        assert set(info['NetworkSettings']['Networks']) == {network}
    outcome = {'health': health, 'readiness': ready, 'api': api, 'backend_image': back_image, 'web_image': web_image}
    remove_container(web, web_id); created.remove((web, web_id))
    remove_container(back, back_id); created.remove((back, back_id))
    return outcome

try:
    backup = json.loads((STATE / 'backup-latest.json').read_text(encoding='utf-8-sig'))
    assert backup['status'] == 'verified' and 'deployment_images' in backup['components']
    artifact = Path('A:/Multica-Recovery/Backups') / backup['artifact']['name']
    with artifact.open('rb') as stream:
        assert hashlib.file_digest(stream, 'sha256').hexdigest() == backup['artifact']['sha256']
    record.update(backup_run_id=backup['run_id'], artifact_sha256=backup['artifact']['sha256'])
    command([PWSH, '-NoProfile', '-NonInteractive', '-File', str(ROOT/'scripts/disaster-recovery/Restore-MulticaIsolated.ps1'),
             '-RepositoryRoot', str(ROOT), '-StateDirectory', str(stage/'state'), '-ArtifactPath', str(artifact),
             '-PassphraseFile', str(KEY), '-DrillId', drill, '-KeepIsolatedResources'])
    restored = True
    payload = stage/'payload.tar.gz'
    decrypt = stage/'decrypt-images.ps1'
    quote = lambda p: "'" + str(p).replace("'", "''") + "'"
    decrypt.write_text("$ErrorActionPreference='Stop'\nImport-Module " + quote(ROOT/'scripts/disaster-recovery/Recovery.Common.psm1') + " -Force\nUnprotect-RecoveryArchive -InputPath " + quote(artifact) + " -OutputPath " + quote(payload) + " -Passphrase (Get-RecoveryPassphrase -PassphraseFile " + quote(KEY) + ")\n", encoding='utf-8')
    command([PWSH, '-NoProfile', '-NonInteractive', '-File', str(decrypt)])
    images = stage/'deployment-images.tar'
    with tarfile.open(payload) as archive:
        manifest = json.load(archive.extractfile('./manifest.json'))
        with archive.extractfile('./deployment-images.tar') as src, images.open('wb') as dst:
            shutil.copyfileobj(src, dst)
    with images.open('rb') as stream:
        assert hashlib.file_digest(stream, 'sha256').hexdigest() == manifest['components']['deployment_images']['sha256']
    payload.unlink()
    command([shutil.which('python') or 'python', str(ROOT/'scripts/verify-completion-images.py'), str(images), '--identity', str(ROOT/'deploy/silver-mediums/adoption-20260916/release-identity.json'), '--receipt', str(stage/'image-verification.json')])
    docker('image', 'load', '--input', str(images))
    record['image_archive_sha256'] = manifest['components']['deployment_images']['sha256']
    network_id = docker('network', 'create', '--internal', '--label', 'silver-mediums.recovery='+drill, network)
    docker('network', 'connect', '--alias', 'postgres', network, database)
    docker('network', 'disconnect', 'bridge', database)
    assert json.loads(docker('network','inspect',network))[0]['Internal']
    record['before'] = facts()
    assert record['before']['runnable_tasks'] == 0
    before_migrations = read_db('SELECT version FROM schema_migrations ORDER BY version;').splitlines()
    record['candidate'] = app_pair(NEW_BACK, NEW_WEB, 'candidate')
    after_migrations = read_db('SELECT version FROM schema_migrations ORDER BY version;').splitlines()
    assert set(before_migrations) <= set(after_migrations)
    assert len(after_migrations) == 603
    record['migrations'] = {'before': len(before_migrations), 'after': len(after_migrations),
                            'added': sorted(set(after_migrations)-set(before_migrations)), 'removed': []}
    record['rollback_compatibility'] = app_pair(OLD_BACK, OLD_WEB, 'rollback')
    record['after'] = facts()
    assert record['after'] == record['before'], 'Representative business counts changed'
    record['status'] = 'verified'
except Exception as error:
    record['status'] = 'failed'
    record['error'] = str(error)[:400]
    raise
finally:
    for name, identity in reversed(created):
        remove_container(name, identity)
    if restored:
        info = json.loads(docker('inspect', database))[0]
        assert info['Name'] == '/' + database and database.startswith('multica_item13_restore_app')
        mounts = {entry['Name'] for entry in info['Mounts'] if entry['Type']=='volume'}
        assert mounts == {database+'_db'}
        docker('rm','-f',info['Id'])
        for name in [database+'_uploads',database+'_db']:
            assert name.startswith('multica_item13_restore_app')
            docker('volume','rm',name)
    if network_id:
        info = json.loads(docker('network','inspect',network_id))[0]
        assert info['Labels'].get('silver-mediums.recovery') == drill and not info['Containers']
        docker('network','rm',network_id)
    assert not docker('ps','-a','--filter','name=^/'+database,'--format','{{.Names}}')
    assert not docker('volume','ls','--filter','name=^'+database,'--format','{{.Name}}')
    assert stage.resolve().is_relative_to(EVIDENCE.resolve()) and stage.name == 'app-recovery-'+drill
    shutil.rmtree(stage)
    record['cleanup_verified'] = True
    record['completed_at'] = datetime.now(timezone.utc).isoformat()
    (EVIDENCE/'application-recovery-rehearsal.json').write_text(json.dumps(record,indent=2)+'\n',encoding='utf-8')
    print(json.dumps(record))
