"""Durable, manifest-backed archive of the gitignored qualification evidence.

The accepted Item 14 and adoption evidence lives under `.qualification-evidence/`,
which `.gitignore` excludes on purpose: it is 1.7 GB of build archives, receipts
and logs that do not belong in normal Git history. That leaves it existing only
inside one temporary worktree. This creates a verified copy outside that
worktree, alongside but never touching the existing encrypted recovery backups.

`--create` writes an uncompressed tar plus a manifest listing every member with
its size and SHA-256. `--verify` re-reads every member out of the archive and
recomputes its digest, so a pass proves the archive itself is readable and
intact rather than proving the source tree still exists.
"""
import argparse
import hashlib
import json
import os
import subprocess
import tarfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / '.qualification-evidence'
DEFAULT_DESTINATION = Path('A:/Multica-Recovery/Evidence-Archive')
CHUNK = 1 << 20


def digest_stream(stream) -> tuple[str, int]:
    sha = hashlib.sha256()
    total = 0
    while True:
        block = stream.read(CHUNK)
        if not block:
            break
        sha.update(block)
        total += len(block)
    return sha.hexdigest(), total


def digest_file(path: Path) -> tuple[str, int]:
    with path.open('rb') as stream:
        return digest_stream(stream)


def source_members() -> list[Path]:
    # Sorted so the manifest and the archive are reproducible.
    return sorted(p for p in EVIDENCE.rglob('*') if p.is_file() and not p.is_symlink())


def git_head() -> str:
    try:
        return subprocess.check_output(
            ['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True
        ).strip()
    except (OSError, subprocess.SubprocessError):
        return 'unknown'


def create(destination: Path) -> Path:
    if not EVIDENCE.is_dir():
        raise SystemExit(f'No evidence directory at {EVIDENCE}')
    destination.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    base = f'multica-qualification-evidence-{stamp}'
    archive_path = destination / f'{base}.tar'
    manifest_path = destination / f'{base}.manifest.json'
    if archive_path.exists() or manifest_path.exists():
        raise SystemExit('An archive with this timestamp already exists; refusing to overwrite')

    members = source_members()
    entries = []
    # Write to a temporary name so an interrupted run never leaves a file that
    # looks like a complete archive.
    temporary = destination / f'{base}.tar.partial'
    try:
        with tarfile.open(temporary, 'w', format=tarfile.PAX_FORMAT) as tar:
            for path in members:
                relative = path.relative_to(ROOT).as_posix()
                sha, size = digest_file(path)
                entries.append({'path': relative, 'bytes': size, 'sha256': sha})
                tar.add(path, arcname=relative, recursive=False)
        os.replace(temporary, archive_path)
    finally:
        if temporary.exists():
            temporary.unlink()

    archive_sha, archive_bytes = digest_file(archive_path)
    manifest = {
        'manifest_version': 1,
        'kind': 'multica_qualification_evidence_archive',
        'created_at': datetime.now(timezone.utc).isoformat(),
        'source_worktree': str(ROOT).replace('\\', '/'),
        'source_directory': '.qualification-evidence',
        'source_is_gitignored': True,
        'git_head': git_head(),
        'file_count': len(entries),
        'total_bytes': sum(e['bytes'] for e in entries),
        'archive_name': archive_path.name,
        'archive_sha256': archive_sha,
        'archive_bytes': archive_bytes,
        'retains_existing_encrypted_backups': True,
        'files': entries,
    }
    manifest_path.write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({k: manifest[k] for k in (
        'archive_name', 'file_count', 'total_bytes', 'archive_sha256', 'git_head')}))
    return manifest_path


def verify(manifest_path: Path) -> None:
    manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
    archive_path = manifest_path.parent / manifest['archive_name']
    archive_sha, archive_bytes = digest_file(archive_path)
    problems = []
    if archive_sha != manifest['archive_sha256']:
        problems.append('archive sha256 differs from the manifest')
    if archive_bytes != manifest['archive_bytes']:
        problems.append('archive size differs from the manifest')

    expected = {e['path']: e for e in manifest['files']}
    seen = set()
    # Read every member back out of the archive. Hashing the source tree again
    # would only prove the source still exists, not that the copy is good.
    with tarfile.open(archive_path, 'r') as tar:
        for member in tar:
            if not member.isfile():
                continue
            record = expected.get(member.name)
            if record is None:
                problems.append(f'unexpected member: {member.name}')
                continue
            stream = tar.extractfile(member)
            if stream is None:
                problems.append(f'unreadable member: {member.name}')
                continue
            sha, size = digest_stream(stream)
            if sha != record['sha256'] or size != record['bytes']:
                problems.append(f'content mismatch: {member.name}')
            seen.add(member.name)
    for missing in sorted(set(expected) - seen):
        problems.append(f'missing member: {missing}')

    receipt = {
        'receipt_version': 1,
        'kind': 'multica_qualification_evidence_archive_verification',
        'verified_at': datetime.now(timezone.utc).isoformat(),
        'manifest': manifest_path.name,
        'archive_name': manifest['archive_name'],
        'archive_sha256': archive_sha,
        'expected_file_count': len(expected),
        'verified_file_count': len(seen),
        'status': 'verified' if not problems else 'failed',
        'problems': problems[:50],
    }
    receipt_path = manifest_path.with_suffix('')
    receipt_path = receipt_path.parent / (receipt_path.stem + '.verification.json')
    receipt_path.write_text(json.dumps(receipt, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({k: receipt[k] for k in (
        'status', 'expected_file_count', 'verified_file_count', 'archive_sha256')}))
    if problems:
        raise SystemExit('Archive verification failed; see ' + str(receipt_path))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--destination', type=Path, default=DEFAULT_DESTINATION)
    parser.add_argument('--create', action='store_true')
    parser.add_argument('--verify', type=Path, help='manifest path to verify')
    args = parser.parse_args()
    if args.create:
        manifest_path = create(args.destination)
        verify(manifest_path)
    elif args.verify:
        verify(args.verify)
    else:
        raise SystemExit('Use --create or --verify <manifest>')


if __name__ == '__main__':
    main()
