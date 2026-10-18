"""Build and package a reviewed private Multica release candidate.

Steps, each run separately and each writing its own receipt:

  prepare <release>   LF clone at HEAD and the EOL-normalized source manifest
  image <release> backend|frontend
                      one image built from the clone; run each on its own, the
                      host cannot hold both builds at once
  receipt <release>   candidate build receipt, once both images exist
  identity <release>  release identity, production overlay and rollback overlay;
                      rollback targets the images running now
  archive <release>   OCI archive of the release and rollback images, verified
  delta <release>     exact build-input delta from the deployed release, with a
                      reviewable unified diff

Nothing here starts, stops or changes a production service. Deployment stays in
scripts/adoption-release.ps1 -Release <release>, behind its gates.
"""
import argparse
import difflib
import hashlib
import json
import re
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WORKTREES = Path('C:/Users/a9sho/.codex/worktrees')
# Where every deployed release's evidence was written. Releases before
# release-20260917 were built in the pilot checkout; later ones in this repo.
EVIDENCE_ROOTS = [ROOT / '.qualification-evidence',
                  Path('C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot/.qualification-evidence')]

PRODUCTION_CONTAINERS = {'backend': 'multica-backend-1', 'frontend': 'multica-frontend-1', 'postgres': 'multica-postgres-1'}
BUILD_ROOT_INPUTS = {'Dockerfile', 'Dockerfile.web', 'Dockerfile.local-candidate-gateway', '.dockerignore', '.npmrc',
                     'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml', 'turbo.json', 'LICENSE', 'NOTICE',
                     'docker-compose.local-candidate.yml'}


def now():
    return datetime.now(timezone.utc).isoformat()


def run(args, cwd=None, timeout=3600, capture=True):
    result = subprocess.run(args, cwd=cwd, capture_output=capture, text=True, encoding='utf-8',
                            errors='replace', timeout=timeout)
    if result.returncode:
        tail = (result.stderr or result.stdout or '')[-2000:] if capture else ''
        raise SystemExit(f'{Path(str(args[0])).name} {" ".join(map(str, args[1:3]))} failed ({result.returncode})\n{tail}')
    return (result.stdout or '').strip()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8', newline='\n')


def sha256_file(path):
    with open(path, 'rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def is_build_input(path):
    return path in BUILD_ROOT_INPUTS or path.startswith(('server/', 'packages/', 'apps/web/', 'docker/'))


def paths_for(release):
    if not re.fullmatch(r'release-\d{8}(-\d+)?', release):
        raise SystemExit('release must be named release-YYYYMMDD, optionally -N for a second release that day')
    return {
        'evidence': ROOT / '.qualification-evidence' / release,
        'production': ROOT / '.qualification-evidence' / f'production-{release}',
        'deploy': ROOT / 'deploy' / 'silver-mediums' / release,
        'clone': WORKTREES / f'multica-{release}',
        'backend_tag': f'smk-{release}-backend:candidate',
        'web_tag': f'smk-{release}-web:candidate',
    }


def fingerprint(root, manifest, check=False):
    args = [sys.executable, str(ROOT / 'scripts/local-candidate-source.py'), '--root', str(root),
            '--manifest', str(manifest), '--normalize-eol']
    if check:
        args += ['--check', '--content-only']
    return json.loads(run(args))


def require_committed_bytes(clone):
    """Refuse a build clone whose build inputs differ from their committed bytes
    (for example CRLF text on Windows)."""
    listing = run(['git', '-C', str(clone), 'ls-files', '--eol'])
    converted = []
    for line in listing.splitlines():
        # "i/<index eol> w/<working-tree eol> attr/<attributes>\t<path>"
        meta, _, path = line.partition('\t')
        fields = meta.split()
        if len(fields) < 2 or not is_build_input(path.strip()):
            continue
        index_eol, work_eol = fields[0][2:], fields[1][2:]
        if index_eol not in ('', '-text') and work_eol != index_eol:
            converted.append(path.strip())
    if converted:
        raise SystemExit(f'{len(converted)} build inputs differ from their committed line endings, e.g. ' + ', '.join(converted[:5]))


def prepare(release):
    p = paths_for(release)
    head = run(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'])
    dirty = [line for line in run(['git', '-C', str(ROOT), 'status', '--porcelain']).splitlines()
             if is_build_input(line[3:].strip().strip('"'))]
    if dirty:
        raise SystemExit('build inputs have uncommitted changes: ' + ', '.join(dirty[:10]))
    clone = p['clone']
    if clone.exists():
        if run(['git', '-C', str(clone), 'status', '--porcelain']):
            raise SystemExit(f'{clone} has changes; remove it deliberately first')
        run(['git', '-C', str(clone), 'fetch', '--quiet', str(ROOT), head])
    else:
        run(['git', '-c', 'core.autocrlf=false', '-c', 'core.eol=lf', 'clone', '--no-hardlinks', '--no-checkout', str(ROOT), str(clone)])
    # core.autocrlf=false alone still checks text out with the native (CRLF)
    # line ending on Windows. The images embed digest-pinned data, so every file
    # must be checked out byte-for-byte as committed: LF.
    run(['git', '-C', str(clone), 'config', 'core.autocrlf', 'false'])
    run(['git', '-C', str(clone), 'config', 'core.eol', 'lf'])
    run(['git', '-C', str(clone), 'checkout', '--quiet', '--detach', head])
    run(['git', '-C', str(clone), 'rm', '-r', '-q', '--cached', '.'])
    run(['git', '-C', str(clone), 'reset', '-q', '--hard', head])
    require_committed_bytes(clone)
    manifest = p['evidence'] / 'candidate-source-manifest.json'
    source = fingerprint(clone, manifest)
    # The everyday checkout must hold exactly the same build-input content.
    fingerprint(ROOT, manifest, check=True)
    print(json.dumps({k: v for k, v in source.items() if k != 'files'}, indent=2))


def image(release, service):
    p = paths_for(release)
    manifest = json.loads((p['evidence'] / 'candidate-source-manifest.json').read_text(encoding='utf-8'))
    clone = p['clone']
    if run(['git', '-C', str(clone), 'rev-parse', 'HEAD']) != manifest['head']:
        raise SystemExit('the build clone is not at the manifest commit; run prepare again')
    fingerprint(clone, p['evidence'] / 'candidate-source-manifest.json', check=True)
    require_committed_bytes(clone)
    started = time.time()
    if service == 'backend':
        run(['docker', 'build', '--pull=false', '-f', 'Dockerfile', '--build-arg', f'VERSION={release}',
             '--build-arg', f'COMMIT={manifest["source_id"]}', '-t', p['backend_tag'], '.'], cwd=clone, timeout=5400)
    else:
        # The build host sits near its memory limit: one Next build worker and a
        # capped heap keep the web build inside it (see Dockerfile.web).
        run(['docker', 'build', '--pull=false', '-f', 'Dockerfile.web', '--build-arg', f'NEXT_PUBLIC_APP_VERSION={release}',
             '--build-arg', 'NEXT_BUILD_LOW_MEMORY=1', '--build-arg', 'BUILD_NODE_OPTIONS=--max-old-space-size=3072',
             '-t', p['web_tag'], '.'], cwd=clone, timeout=5400)
    fingerprint(clone, p['evidence'] / 'candidate-source-manifest.json', check=True)
    tag = p['backend_tag'] if service == 'backend' else p['web_tag']
    record = {'built_at': now(), 'service': service, 'source_id': manifest['source_id'], 'tag': tag,
              'image': run(['docker', 'image', 'inspect', '--format', '{{.Id}}', tag]),
              'seconds': round(time.time() - started, 1)}
    write_json(p['evidence'] / f'image-{service}.json', record)
    print(json.dumps(record, indent=2))


def receipt(release):
    p = paths_for(release)
    manifest = json.loads((p['evidence'] / 'candidate-source-manifest.json').read_text(encoding='utf-8'))
    built = {service: json.loads((p['evidence'] / f'image-{service}.json').read_text(encoding='utf-8'))
             for service in ('backend', 'frontend')}
    images = {}
    for service, record in built.items():
        if record['source_id'] != manifest['source_id']:
            raise SystemExit(f'the {service} image was built from another source')
        current = run(['docker', 'image', 'inspect', '--format', '{{.Id}}', record['tag']])
        if current != record['image']:
            raise SystemExit(f'the {service} tag now names another image')
        images[service] = current
    fingerprint(p['clone'], p['evidence'] / 'candidate-source-manifest.json', check=True)
    record = {'built_at': now(), 'release': release, 'source_id': manifest['source_id'],
              'source_sha256': manifest['source_sha256'], 'source_head': manifest['head'], 'eol_normalized': True,
              'build_input_count': manifest['build_input_count'], 'clone': p['clone'].as_posix(),
              'images': images, 'tags': {s: r['tag'] for s, r in built.items()},
              'build_seconds': {s: r['seconds'] for s, r in built.items()},
              'issue_bridge_in_image': False}
    write_json(p['evidence'] / 'candidate-build-receipt.json', record)
    print(json.dumps(record, indent=2))


def deployed_release(exclude=None):
    """The release running right now, identified by matching the live container
    images against each recorded release identity. Derived rather than declared,
    so it cannot go stale the moment a release is deployed."""
    running = {service: run(['docker', 'inspect', '--format', '{{.Image}}', name])
               for service, name in PRODUCTION_CONTAINERS.items()}
    for directory in sorted((ROOT / 'deploy' / 'silver-mediums').iterdir(), reverse=True):
        recorded = directory / 'release-identity.json'
        if directory.name == exclude or not recorded.is_file():
            continue
        found = json.loads(recorded.read_text(encoding='utf-8'))
        if found.get('images') == running:
            return directory.name, found
    raise SystemExit('no recorded release identity matches the running production images; '
                     'rollback target unknown')


def deployed_evidence(name):
    for root in EVIDENCE_ROOTS:
        candidate = root / name
        if (candidate / 'candidate-source-manifest.json').is_file():
            return candidate
    raise SystemExit(f'no source manifest recorded for the deployed release {name}')


def identity(release):
    p = paths_for(release)
    receipt = json.loads((p['evidence'] / 'candidate-build-receipt.json').read_text(encoding='utf-8'))
    deployed_name, deployed = deployed_release(exclude=release)
    deployed_dir = ROOT / 'deploy' / 'silver-mediums' / deployed_name
    overlay = (deployed_dir / 'compose.yml').read_text(encoding='utf-8')
    for service in ('backend', 'frontend'):
        if overlay.count(deployed['images'][service]) != 1:
            raise SystemExit(f'deployed overlay does not name the {service} image exactly once')
        overlay = overlay.replace(deployed['images'][service], receipt['images'][service])
    overlay = overlay.replace('Used only through scripts/adoption-release.ps1',
                              f'Used only through scripts/adoption-release.ps1 -Release {release}')
    rollback = ('services:\n  backend:\n    image: ' + deployed['images']['backend'] +
                '\n  frontend:\n    image: ' + deployed['images']['frontend'] + '\n')
    p['deploy'].mkdir(parents=True, exist_ok=True)
    (p['deploy'] / 'compose.yml').write_bytes(overlay.encode('utf-8'))
    (p['deploy'] / 'rollback.yml').write_bytes(rollback.encode('utf-8'))
    record = {
        'release': release,
        'source_id': receipt['source_id'],
        'source_sha256': receipt['source_sha256'],
        'source_head': receipt['source_head'],
        'eol_normalized': True,
        'images': {'backend': receipt['images']['backend'], 'frontend': receipt['images']['frontend'],
                   'postgres': deployed['images']['postgres']},
        'rollback_images': {'backend': deployed['images']['backend'], 'frontend': deployed['images']['frontend']},
        'rollback_source_id': deployed['source_id'],
        'rollback_release': deployed_name,
        'base_compose_sha256': sha256_file(ROOT / 'docker-compose.selfhost.yml'),
        'overlay_sha256': sha256_file(p['deploy'] / 'compose.yml'),
        'rollback_overlay_sha256': sha256_file(p['deploy'] / 'rollback.yml'),
    }
    if record['base_compose_sha256'] != deployed['base_compose_sha256']:
        raise SystemExit('docker-compose.selfhost.yml changed since the deployed release; review it before packaging')
    (p['deploy'] / 'release-identity.json').write_bytes((json.dumps(record, indent=2) + '\n').encode('utf-8'))
    print(json.dumps(record, indent=2))


def archive(release):
    p = paths_for(release)
    record = json.loads((p['deploy'] / 'release-identity.json').read_text(encoding='utf-8'))
    ids = sorted(set(record['images'].values()) | set(record['rollback_images'].values()))
    p['production'].mkdir(parents=True, exist_ok=True)
    target = p['production'] / 'deployment-images.tar'
    run(['docker', 'save', '--output', str(target), *ids], timeout=5400)
    run([sys.executable, str(ROOT / 'scripts/verify-completion-images.py'), str(target),
         '--identity', str(p['deploy'] / 'release-identity.json'),
         '--receipt', str(p['production'] / 'image-archive-verification.json')], timeout=5400)
    print((p['production'] / 'image-archive-verification.json').read_text(encoding='utf-8'))


def _forms(data):
    lf = data.replace(b'\r\n', b'\n')
    return {hashlib.sha256(data).hexdigest(), hashlib.sha256(lf).hexdigest(),
            hashlib.sha256(lf.replace(b'\n', b'\r\n')).hexdigest()}


def _deployed_text(path, expected):
    """The deployed build's content for a path, found by hash in the checkout it
    was built from or that checkout's history. None when it cannot be proven."""
    old = deployed_evidence(_deployed_name()).parents[1]
    candidate = old / path
    if candidate.is_file() and expected in _forms(candidate.read_bytes()):
        return candidate.read_bytes()
    for commit in run(['git', '-C', str(old), 'rev-list', '--all', '--', path]).splitlines()[:50]:
        try:
            data = subprocess.run(['git', '-C', str(old), 'show', f'{commit}:{path}'], capture_output=True, check=True).stdout
        except subprocess.CalledProcessError:
            continue
        if expected in _forms(data):
            return data
    return None


def _deployed_name(exclude=None):
    return deployed_release(exclude=exclude)[0]


def delta(release):
    p = paths_for(release)
    deployed = json.loads((deployed_evidence(_deployed_name(exclude=release))
                           / 'candidate-source-manifest.json').read_text(encoding='utf-8'))
    deployed_hashes = {f['path']: f['sha256'] for f in deployed['files']}
    current = json.loads((p['evidence'] / 'candidate-source-manifest.json').read_text(encoding='utf-8'))
    clone = p['clone']
    if run(['git', '-C', str(clone), 'rev-parse', 'HEAD']) != current['head']:
        raise SystemExit('the build clone is not at the manifest commit; run prepare again')
    # The delta depends only on source, so it can be reviewed before the images exist.
    added, changed, unproven = [], [], []
    patches = []
    for entry in current['files']:
        path = entry['path']
        after = (clone / path).read_bytes()
        if path not in deployed_hashes:
            added.append(path)
            before = b''
        elif deployed_hashes[path] in _forms(after):
            continue
        else:
            changed.append(path)
            before = _deployed_text(path, deployed_hashes[path])
            if before is None:
                unproven.append(path)
                before = b''
        patches.extend(difflib.unified_diff(
            before.replace(b'\r\n', b'\n').decode('utf-8', 'replace').splitlines(keepends=True),
            after.replace(b'\r\n', b'\n').decode('utf-8', 'replace').splitlines(keepends=True),
            fromfile=f'deployed/{path}', tofile=f'candidate/{path}'))
    removed = sorted(set(deployed_hashes) - {f['path'] for f in current['files']})
    (p['evidence'] / 'release-delta.diff').write_text(''.join(patches), encoding='utf-8', newline='\n')
    record = {'generated_at': now(), 'deployed_source_id': deployed['source_id'], 'candidate_source_id': current['source_id'],
              'added': added, 'changed': changed, 'removed': removed,
              'changed_without_proven_deployed_content': unproven,
              'diff_sha256': sha256_file(p['evidence'] / 'release-delta.diff')}
    write_json(p['evidence'] / 'release-delta.json', record)
    print(json.dumps({k: (len(v) if isinstance(v, list) else v) for k, v in record.items()}, indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('step', choices=['prepare', 'image', 'receipt', 'identity', 'archive', 'delta'])
    parser.add_argument('release')
    parser.add_argument('service', nargs='?', choices=['backend', 'frontend'])
    args = parser.parse_args()
    if args.step == 'image':
        if not args.service:
            raise SystemExit('image needs backend or frontend')
        image(args.release, args.service)
        return
    {'prepare': prepare, 'receipt': receipt, 'identity': identity, 'archive': archive, 'delta': delta}[args.step](args.release)


if __name__ == '__main__':
    main()
