"""Verify the exact local release/rollback OCI archive without exposing image config."""
import argparse
import hashlib
import json
import tarfile
from datetime import datetime, timezone
from pathlib import Path

EXPECTED = {
    'sha256:126d388925dec5a4a4b672e88b826c3cc9d59a8691baed9a11623d4b6efb8483',
    'sha256:de3c18a6faccbec926def3241521c484a63c25f67d29f2bd442bf1628db2b548',
    'sha256:cf134a767f474095eeba57e0117be8e568e011a63f33fbf252f14c9b760f8e6f',
    'sha256:86f1a85c653eaeaee899a60c30f6a8de1f5d6e179fe5197b2820630207b98798',
    'sha256:c1325230449adb7f4e7a27311f8b0caea9f9881e9d1f0a51431d78b95fe80387',
}
parser = argparse.ArgumentParser()
parser.add_argument('archive', type=Path)
parser.add_argument('--receipt', type=Path, required=True)
args = parser.parse_args()

def blob_path(digest):
    algorithm, value = digest.split(':', 1)
    if algorithm != 'sha256' or len(value) != 64:
        raise ValueError('Unsupported image digest')
    return f'blobs/sha256/{value}'

with tarfile.open(args.archive) as archive:
    index = json.load(archive.extractfile('index.json'))
    assert {item['digest'] for item in index['manifests']} == EXPECTED, 'Image set differs'
    verified = set()
    for member in archive:
        if member.isfile() and member.name.startswith('blobs/sha256/'):
            with archive.extractfile(member) as stream:
                actual = hashlib.file_digest(stream, 'sha256').hexdigest()
            assert member.name == f'blobs/sha256/{actual}', 'Image blob checksum mismatch'
            verified.add('sha256:' + actual)
    for image_id in EXPECTED:
        assert image_id in verified, 'Image index missing'
        child_index = json.load(archive.extractfile(blob_path(image_id)))
        manifests = [entry for entry in child_index['manifests']
                     if entry.get('platform', {}).get('os') == 'linux'
                     and entry.get('platform', {}).get('architecture') == 'amd64']
        assert len(manifests) == 1, 'Expected exactly one Linux amd64 image'
        manifest_id = manifests[0]['digest']
        assert manifest_id in verified, 'Platform manifest missing'
        manifest = json.load(archive.extractfile(blob_path(manifest_id)))
        assert manifest['config']['digest'] in verified, 'Image configuration missing'
        assert all(layer['digest'] in verified for layer in manifest['layers']), 'Image layer missing'
with args.archive.open('rb') as stream:
    archive_hash = hashlib.file_digest(stream, 'sha256').hexdigest()
record = {'status': 'verified', 'verified_at': datetime.now(timezone.utc).isoformat(),
          'archive_sha256': archive_hash, 'bytes': args.archive.stat().st_size,
          'platform': 'linux/amd64', 'images': sorted(EXPECTED), 'verified_blob_count': len(verified)}
args.receipt.write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
print(json.dumps(record))
