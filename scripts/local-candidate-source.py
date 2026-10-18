"""Fingerprint Git-visible Docker build inputs without reading local profiles."""
import argparse
import concurrent.futures
import hashlib
import json
import subprocess
from datetime import datetime, timezone
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--check', action='store_true')
# A release names its own source root and manifest. The defaults keep the
# original local-integration behaviour for existing callers.
parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
parser.add_argument('--manifest', type=Path, default=None)
# Hash file contents with CRLF folded to LF. A Windows checkout with
# core.autocrlf=true and a clean LF build clone then fingerprint identically,
# so the reviewed build can be verified from the everyday checkout.
parser.add_argument('--normalize-eol', action='store_true')
# On --check, compare only the build-input content. HEAD may move with
# records-only commits that change no build input; the reviewed source_id is
# reported unchanged.
parser.add_argument('--content-only', action='store_true')
args = parser.parse_args()
root = args.root.resolve()
evidence = root / '.qualification-evidence/local-integration-20260915'
manifest = args.manifest or (evidence / 'candidate-source-manifest.json')
if not args.check:
    manifest.parent.mkdir(parents=True, exist_ok=True)
names = subprocess.check_output([
    'git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard',
]).decode().split('\0')
root_inputs = {'Dockerfile', 'Dockerfile.web', 'Dockerfile.local-candidate-gateway', '.dockerignore', '.npmrc', 'package.json',
               'pnpm-lock.yaml', 'pnpm-workspace.yaml', 'turbo.json', 'LICENSE', 'NOTICE',
               'docker-compose.local-candidate.yml'}
paths = sorted({name for name in names if name and
                (name in root_inputs or name.startswith(('server/', 'packages/', 'apps/web/', 'docker/')))})

def fingerprint(name):
    path = root / name
    if path.is_symlink():
        raise RuntimeError(f'Unexpected source symlink: {name}')
    data = path.read_bytes()
    if args.normalize_eol:
        data = data.replace(b'\r\n', b'\n')
    return {'path': name, 'sha256': hashlib.sha256(data).hexdigest()}

with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
    files = list(workers.map(fingerprint, paths))
canonical = json.dumps(files, separators=(',', ':'), ensure_ascii=False).encode()
source_hash = hashlib.sha256(canonical).hexdigest()
head = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD']).decode().strip()
source_id = f'{head[:12]}-local-{source_hash[:12]}'
record = {'snapshot': datetime.now(timezone.utc).isoformat(), 'head': head,
          'source_sha256': source_hash, 'source_id': source_id,
          'build_input_count': len(files), 'files': files}
if args.normalize_eol:
    record['eol_normalized'] = True
if args.check:
    expected = json.loads(manifest.read_text(encoding='utf-8'))
    if args.content_only:
        if expected['source_sha256'] != source_hash or expected['files'] != files:
            raise SystemExit('Build inputs changed since the reviewed candidate.')
        record['source_id'] = expected['source_id']
        record['head'] = expected['head']
    elif expected['source_id'] != source_id or expected['files'] != files:
        raise SystemExit('Build inputs changed. Run local-candidate.ps1 Build before Start.')
else:
    raw = json.dumps(record, indent=2) + '\n'
    manifest.write_text(raw, encoding='utf-8')
    (manifest.parent / f'candidate-source-{source_hash}.json').write_text(raw, encoding='utf-8')
print(json.dumps({k: v for k, v in record.items() if k != 'files'}))
