"""Offline recovery wiring checks; no Docker, credentials, provider or production."""
import hashlib,io,json,shutil,subprocess,sys,tarfile,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
class CompletionRecoveryTests(unittest.TestCase):
    def test_successor_verifier_accepts_exact_archive_and_rejects_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);(root/'scripts').mkdir();identity_dir=root/'deploy/silver-mediums/completion-20260915';identity_dir.mkdir(parents=True)
            shutil.copyfile(ROOT/'scripts/verify-completion-images.py',root/'scripts/verify-completion-images.py')
            blobs={}
            def blob(value):
                raw=json.dumps(value,sort_keys=True).encode();digest='sha256:'+hashlib.sha256(raw).hexdigest();blobs['blobs/sha256/'+digest[7:]]=raw;return digest
            ids=[]
            for number in range(5):
                config=blob({'fixture':number});layer=blob({'layer':number});manifest=blob({'config':{'digest':config},'layers':[{'digest':layer}]})
                ids.append(blob({'manifests':[{'platform':{'os':'linux','architecture':'amd64'},'digest':manifest}]}))
            identity={'images':dict(zip(['backend','frontend','postgres'],ids[:3])),'rollback_images':dict(zip(['backend','frontend'],ids[3:]))}
            (identity_dir/'release-identity.json').write_text(json.dumps(identity),encoding='utf-8')
            archive=root/'images.tar';receipt=root/'receipt.json'
            def save(index_ids):
                with tarfile.open(archive,'w') as tar:
                    entries={'index.json':json.dumps({'manifests':[{'digest':v} for v in index_ids]}).encode(),**blobs}
                    for name,raw in entries.items():
                        info=tarfile.TarInfo(name);info.size=len(raw);tar.addfile(info,io.BytesIO(raw))
            command=[sys.executable,str(root/'scripts/verify-completion-images.py'),str(archive),'--receipt',str(receipt)]
            save(ids);result=subprocess.run(command,capture_output=True,text=True);self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(json.loads(receipt.read_text())['images'],sorted(ids))
            save(ids[:-1]);result=subprocess.run(command,capture_output=True,text=True);self.assertNotEqual(result.returncode,0)
    def test_release_and_rehearsal_use_same_successor_paths(self):
        release=(ROOT/'scripts/completion-release.ps1').read_text(encoding='utf-8');rehearsal=(ROOT/'scripts/completion-recovery-rehearsal.py').read_text(encoding='utf-8')
        self.assertNotIn('production-go-live-20260915',release);self.assertNotIn('production-go-live-20260915',rehearsal)
        self.assertIn("$evidenceRoot = Join-Path $releaseRoot '.qualification-evidence/production-completion-20260915'",release)
        self.assertIn("EVIDENCE = ROOT / '.qualification-evidence/production-completion-20260915'",rehearsal)
        self.assertIn('verify-completion-images.py',rehearsal);self.assertNotIn('verify-production-images.py',rehearsal)
        self.assertIn('deploy/silver-mediums/completion-20260915/release-identity.json',release)
        self.assertIn('deploy/silver-mediums/completion-20260915/release-identity.json',rehearsal)
if __name__=='__main__':unittest.main()
