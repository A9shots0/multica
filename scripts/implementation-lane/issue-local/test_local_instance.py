"""Provider-free checks for the local issue-proof instance script."""
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import local_instance  # noqa: E402


class DaemonLaunchDirectoryTest(unittest.TestCase):
    def test_daemon_does_not_start_inside_its_workspaces_root(self):
        work = local_instance.STATE / 'daemon-workspaces'
        launch = local_instance.daemon_launch_dir(work)
        self.assertNotEqual(launch, work.resolve())
        self.assertNotIn(work.resolve(), launch.parents)

    def test_a_launch_directory_below_the_workspaces_root_is_refused(self):
        root = Path(tempfile.mkdtemp(prefix='lane-daemon-root-'))
        original = local_instance.STATE
        try:
            local_instance.STATE = root / 'workspaces' / 'nested'
            with self.assertRaises(RuntimeError):
                local_instance.daemon_launch_dir(root / 'workspaces')
        finally:
            local_instance.STATE = original


class SignalReadingTest(unittest.TestCase):
    def test_owner_summary_signals_are_matched_by_code(self):
        picture = {'revision': 2, 'owner_summary': {'current_picture_revision': 2, 'stage': 'complete',
                                                    'signals': [{'code': 'verified_live', 'references': []}]}}
        self.assertEqual(local_instance.lane_for({'current_picture': picture}), 'verified_live')


class ReviewGrantTest(unittest.TestCase):
    def setUp(self):
        self.original_secret = local_instance.secret
        local_instance.secret = lambda name, factory: '99999999-9999-4999-8999-999999999999'

    def tearDown(self):
        local_instance.secret = self.original_secret

    def test_the_grant_contract_binds_the_reviewed_broker_and_one_reviewer(self):
        from datetime import datetime, timezone
        state = {'squad_id': 'sq', 'runtime_id': 'rt', 'roles': {'leader': 'l', 'implementer': 'i', 'reviewer': 'r'}}
        issued = datetime(2026, 9, 17, 9, 0, tzinfo=timezone.utc)
        contract = local_instance.review_grant_contract(state, 'ws', 'owner', 'issue', 'a' * 64, 'b' * 64, issued)
        self.assertEqual(contract['reviewer_id'], 'r')
        self.assertEqual(contract['implementer_id'], 'i')
        self.assertNotEqual(contract['reviewer_id'], contract['implementer_id'])
        self.assertEqual(contract['evidence_receipt_id'], 'implementation-lane-review-broker-sha256.' + 'a' * 64)
        self.assertEqual(contract['reviewer_name'], local_instance.REVIEWER)
        self.assertEqual(contract['expires_at'], '2026-09-17T12:00:00Z')
        for key in ('broker_path', 'provider_path', 'provider_profile_path'):
            self.assertNotIn('/', contract[key], key)
            self.assertTrue(Path(contract[key]).is_absolute(), key)

    def test_provider_profile_cleanup_stays_inside_the_instance(self):
        root = Path(tempfile.mkdtemp(prefix='lane-provider-home-'))
        original_state, original_home = local_instance.STATE, local_instance.PROVIDER_HOME
        try:
            local_instance.STATE = root
            local_instance.PROVIDER_HOME = root / 'provider-home'
            (root / 'provider-home' / '.claude').mkdir(parents=True)
            local_instance.remove_provider_profile()
            self.assertFalse((root / 'provider-home').exists())
            local_instance.remove_provider_profile()  # absent is fine
            outside = Path(tempfile.mkdtemp(prefix='lane-outside-'))
            local_instance.PROVIDER_HOME = outside
            with self.assertRaises(RuntimeError):
                local_instance.remove_provider_profile()
            self.assertTrue(outside.exists())
        finally:
            local_instance.STATE, local_instance.PROVIDER_HOME = original_state, original_home


class DaemonIdentityTest(unittest.TestCase):
    def test_only_the_hash_of_a_scoped_daemon_token_is_stored(self):
        import hashlib
        token = 'mdt_' + 'ab' * 20
        record = local_instance.daemon_token_record(token, '22222222-2222-4222-8222-222222222222', '2026-09-17T12:00:00+00:00')
        self.assertEqual(record['token_hash'], hashlib.sha256(token.encode()).hexdigest())
        self.assertEqual(record['daemon_id'], local_instance.DAEMON_ID)
        self.assertNotIn(token, repr(record))

    def test_a_personal_access_token_is_not_a_daemon_identity(self):
        for token in ('mul_' + 'ab' * 20, 'mdt_short', ''):
            with self.assertRaises(RuntimeError):
                local_instance.daemon_token_record(token, '22222222-2222-4222-8222-222222222222', '2026-09-17T12:00:00+00:00')

    def test_a_non_canonical_workspace_is_refused(self):
        with self.assertRaises(ValueError):
            local_instance.daemon_token_record('mdt_' + 'ab' * 20, "x'; DROP TABLE daemon_token; --", '2026-09-17T12:00:00+00:00')


if __name__ == '__main__':
    unittest.main()
