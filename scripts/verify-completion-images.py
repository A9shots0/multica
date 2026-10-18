"""Verify the exact local release/rollback OCI archive without exposing image config."""
import argparse
import hashlib
import json
import tarfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
INDEX_MEDIA_TYPE = 'application/vnd.oci.image.index.v1+json'
parser = argparse.ArgumentParser()
parser.add_argument('archive', type=Path)
parser.add_argument('--receipt', type=Path, required=True)
# The identity file names the exact release. It defaults to the release this
# script was written for so existing callers are unchanged.
parser.add_argument('--identity', type=Path,
                    default=ROOT/'deploy/silver-mediums/completion-20260915/release-identity.json')
args = parser.parse_args()
IDENTITY = json.loads(args.identity.read_text(encoding='utf-8'))
EXPECTED = set(IDENTITY['images'].values()) | set(IDENTITY['rollback_images'].values())

def blob_path(digest):
    algorithm, value = digest.split(':', 1)
    if algorithm != 'sha256' or len(value) != 64:
        raise ValueError('Unsupported image digest')
    return f'blobs/sha256/{value}'

with tarfile.open(args.archive) as archive:
    index = json.load(archive.extractfile('index.json'))
    # An entry that does not declare a media type is treated as an image, so a
    # missing key can never demote something into the ignored branch below.
    image_entries = [item for item in index['manifests'] if item.get('mediaType', INDEX_MEDIA_TYPE) == INDEX_MEDIA_TYPE]
    assert {item['digest'] for item in image_entries} == EXPECTED, 'Image set differs'
    # The local daemon also writes orphaned attestation manifests left over from
    # earlier builds into the archive. They carry no image for this release, so
    # each one must name a subject and that subject must not be one of ours --
    # nothing can hide an image here.
    attestations = [item for item in index['manifests'] if item.get('mediaType', INDEX_MEDIA_TYPE) != INDEX_MEDIA_TYPE]
    for item in attestations:
        subject = (item.get('annotations') or {}).get('io.containerd.manifest.subject')
        assert subject and subject not in EXPECTED, 'Unexpected manifest in image archive'
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
          'platform': 'linux/amd64', 'images': sorted(EXPECTED), 'verified_blob_count': len(verified),
          'unrelated_attestation_manifests': sorted(item['digest'] for item in attestations)}
args.receipt.write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
print(json.dumps(record))
