"""Provider-free tests for the implementation lane. No provider, no credentials."""
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from datetime import timedelta
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import lane  # noqa: E402


class VerdictTests(unittest.TestCase):
    def test_accepts_the_exact_contract(self):
        parsed = lane.parse_verdict('{"summary":"did the thing","verdict":"pass","findings":[]}')
        self.assertEqual(parsed['verdict'], 'pass')

    def test_accepts_a_fenced_verdict_with_surrounding_prose(self):
        parsed = lane.parse_verdict(
            'Here you go:\n```json\n{"summary":"nope","verdict":"fail","findings":["x"]}\n```\n')
        self.assertEqual(parsed['findings'], ['x'])

    def test_rejects_a_pass_that_carries_findings(self):
        with self.assertRaisesRegex(RuntimeError, 'zero findings'):
            lane.parse_verdict('{"summary":"s","verdict":"pass","findings":["still broken"]}')

    def test_rejects_extra_or_missing_keys(self):
        for payload in ('{"summary":"s","verdict":"pass"}',
                        '{"summary":"s","verdict":"pass","findings":[],"extra":1}'):
            with self.assertRaisesRegex(RuntimeError, 'verdict keys'):
                lane.parse_verdict(payload)

    def test_rejects_an_unknown_verdict_word(self):
        with self.assertRaisesRegex(RuntimeError, 'pass or fail'):
            lane.parse_verdict('{"summary":"s","verdict":"maybe","findings":[]}')

    def test_rejects_unparsable_output(self):
        with self.assertRaisesRegex(RuntimeError, 'parsable verdict'):
            lane.parse_verdict('I finished the task successfully.')


class AuthorityTests(unittest.TestCase):
    def setUp(self):
        self.run = lane.Run.__new__(lane.Run)
        self.run.events = []
        self.run.authority = None
        self.run.run_id = 'test'
        self.run.task_id = 'test'
        self.run.worktree = Path('C:/tmp/test-worktree')
        self.run.save = lambda *a, **k: None

    def _mint(self):
        self.run.mint_authority({'base_ref': 'abc', 'allowed_paths': ['x/'], 'max_turns': 3})

    def test_a_live_authority_permits_a_call(self):
        self._mint()
        self.run.require_live_authority('executor')

    def test_a_revoked_authority_denies_every_further_call(self):
        self._mint()
        self.run.revoke_authority('cleanup')
        with self.assertRaisesRegex(RuntimeError, 'revoked'):
            self.run.require_live_authority('executor')

    def test_an_expired_authority_denies_every_further_call(self):
        self._mint()
        expired = lane.utc() - timedelta(minutes=1)
        self.run.authority['expires_at'] = expired.isoformat()
        with self.assertRaisesRegex(RuntimeError, 'expired'):
            self.run.require_live_authority('executor')

    def test_authority_grants_no_standing_permission(self):
        self._mint()
        a = self.run.authority
        for field in ('production_access', 'deployment', 'outbound_communication',
                      'phone_farm', 'other_repositories', 'permanent_selection'):
            self.assertFalse(a[field], field)
        self.assertEqual(a['maximum_corrections'], 1)
        self.assertNotIn('Task', a['executor_tools'])

    def test_every_run_mints_a_distinct_authority(self):
        self._mint()
        first = self.run.authority['nonce']
        self._mint()
        self.assertNotEqual(first, self.run.authority['nonce'])


class ToolPolicyTests(unittest.TestCase):
    def test_the_reviewer_cannot_edit_or_run_anything(self):
        for forbidden in ('Edit', 'Write', 'Bash'):
            self.assertNotIn(forbidden, lane.REVIEWER_TOOLS)

    def test_dangerous_commands_are_explicitly_disallowed(self):
        joined = ' '.join(lane.EXECUTOR_DISALLOWED)
        for pattern in ('git push', 'git commit', 'docker', 'curl', 'ssh', 'WebFetch'):
            self.assertIn(pattern, joined)

    def test_the_forbidden_capability_set_matches_the_accepted_vocabulary(self):
        self.assertEqual(sorted(lane.FORBIDDEN_CAPABILITIES), sorted([
            'browser', 'git_push', 'deployment', 'external_messaging',
            'credential_access', 'phone_farm', 'host_files_outside_workspace']))


class ContainmentTests(unittest.TestCase):
    def test_paths_outside_the_allowed_scope_are_reported(self):
        recorded = {}
        fake = type('R', (), {
            'save': lambda self, n, v: recorded.update({n: v}),
            'event': lambda self, k, **f: None,
        })()
        with patch.object(lane, 'capture_diff', return_value={
                'diff': '', 'changed_paths': ['packages/views/locales/ja/chat.json'],
                'untracked_paths': ['server/secret.go']}), \
             patch.object(lane, 'run', return_value=type('P', (), {'stdout': 'abc'})()):
            report = lane.containment_check(Path('.'), ['packages/views/locales/'], fake)
        self.assertFalse(report['contained'])
        self.assertEqual(report['paths_outside_allowed_scope'], ['server/secret.go'])

    def test_an_in_scope_change_is_contained(self):
        fake = type('R', (), {'save': lambda self, n, v: None, 'event': lambda self, k, **f: None})()
        with patch.object(lane, 'capture_diff', return_value={
                'diff': '', 'changed_paths': ['packages/views/locales/ja/chat.json'],
                'untracked_paths': []}), \
             patch.object(lane, 'run', return_value=type('P', (), {'stdout': 'abc'})()):
            report = lane.containment_check(Path('.'), ['packages/views/locales/'], fake)
        self.assertTrue(report['contained'])


class RevisionPolicyTests(unittest.TestCase):
    def test_go_commands_are_granted_only_to_go_tasks(self):
        self.assertFalse(set(lane.GO_ALLOWED) & set(lane.executor_allowed({})))
        go = lane.executor_allowed({'toolchains': ['go']})
        for command in ('go build', 'go vet', 'go test'):
            self.assertTrue(any(command in pattern for pattern in go), command)
        self.assertFalse(any('go run' in pattern or 'go get' in pattern for pattern in go))

    def test_commands_can_never_reach_the_local_database_or_module_proxy(self):
        env = lane.sandbox_env()
        self.assertIn('127.0.0.1:9/', env['DATABASE_URL'])
        self.assertNotIn('5432', env['DATABASE_URL'])
        self.assertEqual(env['GOPROXY'], 'off')

    def test_credentials_in_the_operator_shell_never_reach_a_task(self):
        planted = {'CRM_DATABASE_URL': 'postgres://planted', 'CLAUDE_CODE_MESSAGING_TOKEN': 'x',
                   'GH_TOKEN': 'x', 'SOME_API_KEY': 'x', 'STRIPE_SECRET': 'x',
                   'PATH': os.environ.get('PATH', ''), 'GOPATH': 'kept'}
        with patch.dict(os.environ, planted, clear=True):
            env = lane.sandbox_env()
            withheld = lane.withheld_env_names()
        for name in ('CRM_DATABASE_URL', 'CLAUDE_CODE_MESSAGING_TOKEN', 'GH_TOKEN',
                     'SOME_API_KEY', 'STRIPE_SECRET'):
            self.assertNotIn(name, env)
            self.assertIn(name, withheld)
        self.assertEqual(env['GOPATH'], 'kept')
        self.assertIn('PATH', env)
        # The lane's own unreachable address is set after filtering, so it survives.
        self.assertIn('127.0.0.1:9/', env['DATABASE_URL'])

    def test_memory_floor_refuses_a_starved_host_and_admits_a_healthy_one(self):
        starved = {'available_physical_gb': 1.7, 'available_commit_gb': 2.3}
        self.assertEqual(len(lane.memory_refusals(starved)), 2)
        healthy = {'available_physical_gb': 9.0, 'available_commit_gb': 12.0}
        self.assertEqual(lane.memory_refusals(healthy), [])

    def test_memory_status_reads_the_real_host(self):
        self.assertGreater(lane.memory_status()['total_physical_gb'], 0)

    def test_reviewer_must_list_out_of_scope_changes(self):
        with self.assertRaisesRegex(RuntimeError, 'verdict keys'):
            lane.parse_verdict('{"summary":"s","verdict":"pass","findings":[]}', lane.REVIEWER_KEYS)
        with self.assertRaisesRegex(RuntimeError, 'must be a list'):
            lane.parse_verdict('{"summary":"s","verdict":"pass","findings":[],'
                               '"out_of_scope_changes":"none","contract_warning_resolutions":[]}',
                               lane.REVIEWER_KEYS)
        with self.assertRaisesRegex(RuntimeError, 'contract_warning_resolutions must be a list'):
            lane.parse_verdict('{"summary":"s","verdict":"pass","findings":[],'
                               '"out_of_scope_changes":[],"contract_warning_resolutions":"n/a"}',
                               lane.REVIEWER_KEYS)
        parsed = lane.parse_verdict('{"summary":"s","verdict":"pass","findings":[],'
                                    '"out_of_scope_changes":["a.go: reordered lookup"],'
                                    '"contract_warning_resolutions":[]}',
                                    lane.REVIEWER_KEYS)
        self.assertEqual(parsed['out_of_scope_changes'], ['a.go: reordered lookup'])

    def test_contract_check_flags_only_required_fields_the_server_may_omit(self):
        root = Path(tempfile.mkdtemp(prefix='lane-contract-'))
        try:
            (root / 'server' / 'pkg' / 'protocol').mkdir(parents=True)
            (root / 'server' / 'pkg' / 'protocol' / 'p.go').write_text(
                'type M struct {\n'
                '\tTaskID string `json:"task_id"`\n'
                '\tIssueID string `json:"issue_id,omitempty"`\n'
                '\tTool string `json:"tool,omitempty"`\n}\n', encoding='utf-8')
            (root / 'server' / 'pkg' / 'protocol' / 'p_test.go').write_text(
                'type T struct {\n\tSeq int `json:"task_id,omitempty"`\n}\n', encoding='utf-8')
            # Outside the response packages: a name collision that must not count.
            (root / 'server' / 'pkg' / 'agent').mkdir(parents=True)
            (root / 'server' / 'pkg' / 'agent' / 'a.go').write_text(
                'type A struct {\n\tID string `json:"unknown_field,omitempty"`\n}\n', encoding='utf-8')
            patch_text = (
                'diff --git a/packages/core/api/schemas.ts b/packages/core/api/schemas.ts\n'
                '+++ b/packages/core/api/schemas.ts\n'
                '+  task_id: z.string(),\n'
                '+  issue_id: z.string(),\n'
                '+  tool: z.string().optional(),\n'
                '+  unknown_field: z.string(),\n'
                'diff --git a/packages/core/api/x.test.ts b/packages/core/api/x.test.ts\n'
                '+++ b/packages/core/api/x.test.ts\n'
                '+  issue_id: z.string(),\n')
            report = lane.server_contract_check(patch_text, root / 'server')
            self.assertEqual([w['field'] for w in report['required_fields_that_the_server_may_omit']],
                             ['issue_id'])
            warning = report['required_fields_that_the_server_may_omit'][0]
            self.assertEqual(warning['server_omitempty_at'], ['server/pkg/protocol/p.go:3'])

            moved = (
                'diff --git a/packages/core/api/schemas.ts b/packages/core/api/schemas.ts\n'
                '--- a/packages/core/api/schemas.ts\n'
                '+++ b/packages/core/api/schemas.ts\n'
                '-      issue_id: z.string(),\n'
                '+    issue_id: z.string(),\n')
            self.assertEqual(lane.server_contract_check(moved, root / 'server')['warning_count'], 0)
        finally:
            shutil.rmtree(root)

    def test_evidence_is_written_byte_for_byte(self):
        target = Path(tempfile.mkdtemp(prefix='lane-evidence-')) / 'final.patch'
        try:
            lane.write_evidence(target, 'line one\nline two\n')
            self.assertEqual(target.read_bytes(), b'line one\nline two\n')
        finally:
            shutil.rmtree(target.parent)

    def test_only_one_lane_run_holds_the_host_lock(self):
        root = Path(tempfile.mkdtemp(prefix='lane-lock-'))
        lock = root / 'lane-run.lock'
        try:
            first = lane.acquire_run_lock('run-a', lock)
            self.assertTrue(first['acquired'])
            second = lane.acquire_run_lock('run-b', lock)
            self.assertFalse(second['acquired'])
            self.assertEqual(second['holder']['run_id'], 'run-a')
            lane.release_run_lock('run-b', lock)
            self.assertTrue(lock.exists(), 'a run cannot release a lock it does not hold')
            lane.release_run_lock('run-a', lock)
            self.assertFalse(lock.exists())
        finally:
            shutil.rmtree(root)

    def test_a_lock_left_by_a_finished_process_is_taken_over(self):
        root = Path(tempfile.mkdtemp(prefix='lane-lock-'))
        lock = root / 'lane-run.lock'
        try:
            finished = subprocess.run([sys.executable, '-c', 'import os; print(os.getpid())'],
                                      capture_output=True, text=True, check=True)
            lock.write_text(json.dumps({'pid': int(finished.stdout.strip()), 'run_id': 'crashed'}),
                            encoding='utf-8')
            self.assertFalse(lane.process_alive(int(finished.stdout.strip())))
            taken = lane.acquire_run_lock('run-c', lock)
            self.assertTrue(taken['acquired'])
            self.assertTrue(lane.process_alive(os.getpid()))
        finally:
            shutil.rmtree(root)

    def test_test_paths_are_recognised(self):
        for path in ('server/x_test.go', 'packages/a.test.ts', 'b.test.tsx', 'e2e/c.spec.ts',
                     'tests/test_crm_pulse.py', 'test_root.py', 'pkg/thing_test.py',
                     'tests/crm_telemetry_collector.test.mjs'):
            self.assertTrue(lane.is_test_path(path), path)
        for path in ('server/x.go', 'packages/a.ts', 'testdata/fixture.json',
                     'services/crm_pulse.py', 'static/js/crm-telemetry.js', 'tests/conftest.py',
                     'services/test_helpers_data.json', 'latest_report.py'):
            self.assertFalse(lane.is_test_path(path), path)


def git(repo, *args):
    return subprocess.run(['git', '-C', str(repo), *args], capture_output=True, text=True,
                          encoding='utf-8', check=True).stdout


class FreezeTests(unittest.TestCase):
    def test_a_changed_lane_file_breaks_the_freeze(self):
        root = Path(tempfile.mkdtemp(prefix='lane-freeze-'))
        try:
            lane_dir = root / 'scripts' / 'implementation-lane'
            lane_dir.mkdir(parents=True)
            source = lane_dir / 'lane.py'
            source.write_text('frozen\n', encoding='utf-8')
            freeze = lane_dir / 'FROZEN.json'
            freeze.write_text(json.dumps({'freeze_version': 5, 'source_hashes': {
                'scripts/implementation-lane/lane.py': lane.source_sha(source)}}), encoding='utf-8')
            self.assertTrue(lane.freeze_check(freeze)['intact'])
            # A CRLF checkout of the same source is not a change.
            source.write_bytes(b'frozen\r\n')
            self.assertTrue(lane.freeze_check(freeze)['intact'])
            source.write_text('edited mid-trial\n', encoding='utf-8')
            result = lane.freeze_check(freeze)
            self.assertFalse(result['intact'])
            self.assertEqual(result['changed_files'], ['scripts/implementation-lane/lane.py'])
        finally:
            shutil.rmtree(root)


class GitRepoCase(unittest.TestCase):
    """Real git on throwaway repositories. No provider is involved."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix='lane-test-'))
        self.repo = self.tmp / 'repo'
        self.repo.mkdir()
        git(self.repo, 'init', '-q')
        git(self.repo, 'config', 'user.email', 'lane-test@example.invalid')
        git(self.repo, 'config', 'user.name', 'lane test')
        (self.repo / '.gitignore').write_text('build/\n', encoding='utf-8')
        (self.repo / 'app.txt').write_text('broken\n', encoding='utf-8')
        git(self.repo, 'add', '.gitignore', 'app.txt')
        git(self.repo, 'commit', '-q', '-m', 'base')

    def tearDown(self):
        def unlock(func, path, _):
            os.chmod(path, stat.S_IWRITE)
            func(path)
        shutil.rmtree(self.tmp, onerror=unlock)

    def fake_run(self, evidence):
        saved = {}
        fake = type('R', (), {
            'worktree': self.repo, 'directory': evidence,
            'save': lambda _self, n, v: saved.update({n: v}),
            'event': lambda _self, k, **f: None,
        })()
        return fake, saved

class RealGitTests(GitRepoCase):
    def test_the_guard_allows_deletion_only_when_every_file_is_in_the_patch(self):
        (self.repo / 'app.txt').write_text('fixed\n', encoding='utf-8')
        (self.repo / 'new_module.txt').write_text('created by the agent\n', encoding='utf-8')
        captured = lane.capture_diff(self.repo)['diff']
        self.assertIn('new_module.txt', captured)
        self.assertTrue(lane.cleanup_guard(self.repo, captured, [])['safe_to_delete'])

    def test_the_guard_refuses_when_a_created_file_is_missing_from_the_patch(self):
        (self.repo / 'app.txt').write_text('fixed\n', encoding='utf-8')
        # The first trial's defect: a capture that covered tracked files only.
        tracked_only = git(self.repo, 'diff', '--binary')
        (self.repo / 'lost.test.ts').write_text('the test the agent wrote\n', encoding='utf-8')
        report = lane.cleanup_guard(self.repo, tracked_only, [])
        self.assertFalse(report['safe_to_delete'])
        self.assertIn('lost.test.ts', report['paths_present_but_not_captured'])

    def test_the_guard_refuses_when_the_agent_created_an_ignored_file(self):
        before = sorted(lane.ignored_entries(self.repo))
        (self.repo / 'build').mkdir()
        (self.repo / 'build' / 'output.bin').write_bytes(bytes([0, 1]))
        report = lane.cleanup_guard(self.repo, lane.capture_diff(self.repo)['diff'], before)
        self.assertFalse(report['safe_to_delete'])
        self.assertEqual(report['new_ignored_entries'], ['build/'])

    def test_check_command_caches_do_not_block_deletion_but_are_recorded(self):
        (self.repo / '.gitignore').write_text('build/\n.turbo\n*.tsbuildinfo\n', encoding='utf-8')
        git(self.repo, 'add', '.gitignore')
        git(self.repo, 'commit', '-q', '-m', 'ignore caches')
        before = sorted(lane.ignored_entries(self.repo))
        (self.repo / '.turbo').mkdir()
        (self.repo / '.turbo' / 'cache.json').write_text('{}', encoding='utf-8')
        (self.repo / 'tsconfig.tsbuildinfo').write_text('{}', encoding='utf-8')
        report = lane.cleanup_guard(self.repo, lane.capture_diff(self.repo)['diff'], before)
        self.assertTrue(report['safe_to_delete'], report)
        self.assertEqual(sorted(report['regenerable_cache_entries']),
                         ['.turbo/', 'tsconfig.tsbuildinfo'])

    def test_binary_content_is_captured_not_elided(self):
        (self.repo / 'icon.bin').write_bytes(bytes(range(256)))
        captured = lane.capture_diff(self.repo)['diff']
        self.assertIn('GIT binary patch', captured)
        self.assertTrue(lane.cleanup_guard(self.repo, captured, [])['safe_to_delete'])

    def _fails_before(self, test_content):
        import run_task
        (self.repo / 'app.txt').write_text('fixed\n', encoding='utf-8')
        (self.repo / 'helper.txt').write_text('new product file\n', encoding='utf-8')
        (self.repo / 'app.test.ts').write_text(test_content, encoding='utf-8')
        final = lane.capture_diff(self.repo)['diff']
        evidence = self.tmp / 'evidence'
        evidence.mkdir()
        fake, _ = self.fake_run(evidence)
        # Stand-in test: a "strict" test passes only on the fixed product code;
        # a "lenient" one passes regardless.
        checker = self.tmp / 'check.py'
        checker.write_text(
            'import pathlib, sys\n'
            'fixed = pathlib.Path("app.txt").read_text().strip() == "fixed" '
            'and pathlib.Path("helper.txt").exists()\n'
            'strict = "strict" in pathlib.Path("app.test.ts").read_text()\n'
            'sys.exit(0 if (fixed or not strict) else 1)\n', encoding='utf-8')
        task = {'verification_command': [sys.executable, str(checker)], 'verification_cwd': '.'}
        return run_task.fails_before_check(task, fake, final), final

    def test_a_test_that_detects_the_defect_is_accepted_and_the_change_restored(self):
        record, final = self._fails_before('strict\n')
        self.assertTrue(record['detected_defect'])
        self.assertTrue(record['restored_exactly'])
        self.assertEqual(lane.capture_diff(self.repo)['diff'], final)
        self.assertEqual((self.repo / 'app.txt').read_text(encoding='utf-8'), 'fixed\n')
        self.assertTrue((self.repo / 'helper.txt').exists())

    def test_a_test_that_passes_without_the_change_is_rejected(self):
        record, final = self._fails_before('lenient\n')
        self.assertFalse(record['detected_defect'])
        self.assertEqual(lane.capture_diff(self.repo)['diff'], final)

    def test_a_change_without_a_test_is_rejected(self):
        import run_task
        (self.repo / 'app.txt').write_text('fixed\n', encoding='utf-8')
        fake, _ = self.fake_run(self.tmp)
        record = run_task.fails_before_check({}, fake, lane.capture_diff(self.repo)['diff'])
        self.assertFalse(record['detected_defect'])

    def test_the_sweep_removes_clean_orphans_and_blocks_on_anything_holding_work(self):
        parent = self.tmp / 'lane-worktrees'
        parent.mkdir()
        git(self.repo, 'worktree', 'add', '-q', '--detach', str(parent / 'clean-orphan'), 'HEAD')
        git(self.repo, 'worktree', 'add', '-q', '--detach', str(parent / 'dirty-orphan'), 'HEAD')
        (parent / 'dirty-orphan' / 'app.txt').write_text('unsaved agent work\n', encoding='utf-8')
        (parent / 'stray').mkdir()
        with patch.object(lane, 'ROOT', self.repo), patch.object(lane, 'WORKTREE_PARENT', parent):
            result = lane.sweep_orphans()
        self.assertEqual(len(result['removed']), 1)
        self.assertFalse((parent / 'clean-orphan').exists())
        reasons = {Path(b['path']).name: b['reason'] for b in result['blocking']}
        self.assertEqual(reasons['dirty-orphan'], 'registered worktree holds changes')
        self.assertEqual(reasons['stray'], 'directory is not a registered worktree')
        self.assertEqual((parent / 'dirty-orphan' / 'app.txt').read_text(encoding='utf-8'),
                         'unsaved agent work\n')


class EndToEndWiringTests(GitRepoCase):
    """The whole runner against a throwaway repository with a stand-in provider.

    This proves the wiring between phases, not agent quality. The stand-in
    edits files the way an executor would and returns fixed verdicts.
    """

    def _run(self, executor_writes, memory=None, resolutions=None, extra_task=None):
        import run_task
        resolutions = [] if resolutions is None else resolutions
        checker = self.tmp / 'check.py'
        checker.write_text(
            'import pathlib, sys\n'
            'ok = pathlib.Path("app.txt").read_text().strip() == "fixed" '
            'and pathlib.Path("app.test.ts").exists()\n'
            'sys.exit(0 if ok else 1)\n', encoding='utf-8')
        task_file = self.tmp / 'task.json'
        task_file.write_text(json.dumps({
            'task_id': 'wiring', 'title': 'wiring', 'base_ref': 'HEAD',
            'allowed_paths': ['app', 'build/', 'schema.ts'], 'dependencies': 'none', 'toolchains': ['go'],
            'verification_command': [sys.executable, str(checker)], 'verification_cwd': '.',
            'objective': 'fix app.txt', 'max_turns': 3, **(extra_task or {})}), encoding='utf-8')

        def provider(run_obj, slot, prompt, tools, allowed, disallowed, max_turns, cwd):
            if slot == '01-executor':
                executor_writes(Path(cwd))
                return {'summary': 'done', 'verdict': 'pass', 'findings': []}, {}
            reviewed_prompts.append(prompt)
            return {'summary': 'ok', 'verdict': 'pass', 'findings': [],
                    'out_of_scope_changes': [], 'contract_warning_resolutions': resolutions}, {}

        reviewed_prompts = []
        self.reviewed_prompts = reviewed_prompts

        def codex_reviewer(run_obj, slot, prompt, cwd):
            return provider(run_obj, slot, prompt, None, None, None, None, cwd)

        healthy = dict({'memory_load_percent': 50, 'total_physical_gb': 32.0, 'total_commit_gb': 64.0,
                        'observed_at': 'test'},
                       **(memory or {'available_physical_gb': 9.0, 'available_commit_gb': 9.0}))
        with patch.object(lane, 'ROOT', self.repo), patch.object(run_task, 'ROOT', self.repo), \
                patch.object(lane, 'RUNS', self.tmp / 'runs'), \
                patch.object(lane, 'WORKTREE_PARENT', self.tmp / 'lane-worktrees'), \
                patch.object(lane, 'PROVIDER', Path(sys.executable)), \
                patch.object(run_task, 'PROVIDER', Path(sys.executable)), \
                patch.object(lane, 'memory_status', return_value=healthy), \
                patch.object(lane, 'MEMORY_WAIT_MAX_SECONDS', 0), \
                patch.object(run_task, 'freeze_check', return_value={
                    'freeze_version': 5, 'changed_files': [], 'intact': True}), \
                patch.object(lane.Run, 'call_provider', provider), \
                patch.object(lane.Run, 'call_codex_reviewer', codex_reviewer), \
                patch.object(sys, 'argv', ['run_task.py', str(task_file)]):
            with self.assertRaises(SystemExit) as exit_:
                run_task.main()
        outcome_path = next((self.tmp / 'runs').glob('wiring-*/outcome.json'))
        return exit_.exception.code, json.loads(outcome_path.read_text(encoding='utf-8'))

    def test_a_clean_run_passes_every_gate_and_leaves_nothing_behind(self):
        def good(tree):
            (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
            (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
        code, outcome = self._run(good)
        self.assertEqual(code, 0, outcome.get('failure'))
        self.assertTrue(outcome['succeeded'])
        self.assertTrue(outcome['test_fails_before_change'])
        self.assertEqual(outcome['out_of_scope_changes'], [])
        self.assertTrue(outcome['worktree_removed'])
        self.assertTrue(outcome['authority_revoked'])
        self.assertEqual(outcome['dependency_install_seconds'], 0)
        patch_bytes = next((self.tmp / 'runs').glob('wiring-*/final.patch')).read_bytes()
        self.assertIn(b'app.test.ts', patch_bytes)
        # Saved exactly as git produced it: applies as-is, no CRLF conversion.
        self.assertNotIn(b'\r\n', patch_bytes)
        self.assertEqual(outcome['server_contract_warnings'], 0)
        authority = json.loads(next((self.tmp / 'runs').glob('wiring-*/authority.json'))
                               .read_text(encoding='utf-8'))
        self.assertIn('Bash(go test:*)', authority['executor_allowed_tool_patterns'])

    def test_a_run_that_leaves_uncaptured_work_keeps_its_worktree_and_fails(self):
        def leaves_ignored_output(tree):
            (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
            (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
            (tree / 'build').mkdir()
            (tree / 'build' / 'generated.txt').write_text('agent output\n', encoding='utf-8')
        code, outcome = self._run(leaves_ignored_output)
        self.assertEqual(code, 1)
        self.assertFalse(outcome['succeeded'])
        self.assertTrue(outcome['worktree_kept_for_inspection'])
        self.assertIn('worktree kept', outcome['failure'])
        self.assertTrue(outcome['authority_revoked'])

    def _commit_server_struct(self):
        (self.repo / 'server' / 'pkg' / 'protocol').mkdir(parents=True)
        (self.repo / 'server' / 'pkg' / 'protocol' / 'protocol.go').write_text(
            'package protocol\n\ntype TaskMessage struct {\n'
            '\tTaskID  string `json:"task_id"`\n'
            '\tIssueID string `json:"issue_id,omitempty"`\n}\n', encoding='utf-8')
        git(self.repo, 'add', 'server/pkg/protocol/protocol.go')
        git(self.repo, 'commit', '-q', '-m', 'server struct')

    @staticmethod
    def _adds_a_required_omitempty_field(tree):
        (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
        (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
        (tree / 'schema.ts').write_text(
            'export const S = z.object({\n  task_id: z.string(),\n  issue_id: z.string(),\n});\n',
            encoding='utf-8')

    def test_a_contract_warning_the_reviewer_does_not_resolve_fails_the_run(self):
        self._commit_server_struct()
        code, outcome = self._run(self._adds_a_required_omitempty_field, resolutions=[])
        self.assertEqual(code, 1)
        self.assertEqual(outcome['server_contract_warnings'], 1)
        self.assertIn('resolved 0 of 1 server-contract warnings', outcome['failure'])
        self.assertIn('issue_id', self.reviewed_prompts[0])
        self.assertIn('server/pkg/protocol/protocol.go:5', self.reviewed_prompts[0])
        self.assertTrue(outcome['worktree_removed'])

    def test_a_failing_regression_check_fails_an_otherwise_passing_run(self):
        breaks = self.tmp / 'existing_suite.py'
        breaks.write_text('import sys\nprint("existing test broke")\nsys.exit(1)\n', encoding='utf-8')
        def good(tree):
            (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
            (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
        code, outcome = self._run(good, extra_task={
            'regression_commands': [{'command': [sys.executable, str(breaks)], 'cwd': '.'}]})
        self.assertEqual(code, 1)
        self.assertFalse(outcome['regression_passed'])
        self.assertEqual(outcome['failure'], 'a regression check fails with the change applied')
        self.assertIn('existing test broke', self.reviewed_prompts[0])
        self.assertTrue(outcome['worktree_removed'])

    def test_a_passing_regression_check_is_reported_to_the_reviewer(self):
        fine = self.tmp / 'existing_suite.py'
        fine.write_text('print("all existing tests pass")\n', encoding='utf-8')
        def good(tree):
            (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
            (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
        code, outcome = self._run(good, extra_task={
            'regression_commands': [{'command': [sys.executable, str(fine)], 'cwd': '.'}]})
        self.assertEqual(code, 0, outcome.get('failure'))
        self.assertTrue(outcome['regression_passed'])
        self.assertIn('all existing tests pass', self.reviewed_prompts[0])

    def test_a_resolved_contract_warning_lets_the_reviewer_decide(self):
        self._commit_server_struct()
        code, outcome = self._run(self._adds_a_required_omitempty_field,
                                  resolutions=['issue_id: server/protocol.go TaskMessage -> may be absent'])
        self.assertEqual(code, 0, outcome.get('failure'))
        self.assertEqual(outcome['contract_warning_resolutions'],
                         ['issue_id: server/protocol.go TaskMessage -> may be absent'])

    def test_a_starved_host_refuses_before_any_authority_or_worktree(self):
        code, outcome = self._run(lambda tree: None,
                                  memory={'available_physical_gb': 1.7, 'available_commit_gb': 2.3})
        self.assertEqual(code, 2)
        self.assertTrue(outcome['host_blocked'])
        self.assertFalse(list((self.tmp / 'runs').glob('wiring-*/authority.json')))
        self.assertFalse((self.tmp / 'lane-worktrees').exists())

    def test_a_clean_run_records_its_memory_profile(self):
        def good(tree):
            (tree / 'app.txt').write_text('fixed\n', encoding='utf-8')
            (tree / 'app.test.ts').write_text('strict\n', encoding='utf-8')
        code, outcome = self._run(good)
        self.assertEqual(code, 0, outcome.get('failure'))
        self.assertIn('peak_lane_process_tree_mb', outcome['memory'])
        self.assertGreaterEqual(outcome['memory']['samples'], 1)


class CodexReviewerTests(unittest.TestCase):
    def _run_obj(self, tmp):
        run = lane.Run.__new__(lane.Run)
        run.events, run.run_id, run.task_id = [], 'r1', 't1'
        run.directory, run.worktree = tmp / 'evidence', tmp / 'worktree'
        run.directory.mkdir()
        run.worktree.mkdir()
        run.authority = None
        run.mint_authority({'base_ref': 'abc', 'allowed_paths': ['x/'], 'max_turns': 3})
        return run

    def test_events_count_reads_usage_and_forbidden_tools(self):
        stdout = '\n'.join(json.dumps(e) for e in [
            {'type': 'item.completed', 'item': {'type': 'command_execution', 'command': 'Get-Content a.go', 'exit_code': 0}},
            {'type': 'item.completed', 'item': {'type': 'web_search'}},
            {'type': 'item.completed', 'item': {'type': 'agent_message', 'text': '{}'}},
            {'type': 'turn.completed', 'usage': {'input_tokens': 10, 'output_tokens': 2}},
        ])
        events = lane.parse_codex_events(stdout + '\nnot json')
        self.assertEqual(len(events['commands']), 1)
        self.assertEqual(events['forbidden'], ['web_search'])
        self.assertEqual(events['usage'], {'input_tokens': 10, 'output_tokens': 2})

    def test_a_review_uses_a_fresh_codex_home_that_is_removed_and_names_model_and_effort(self):
        tmp = Path(tempfile.mkdtemp(prefix='lane-codex-'))
        try:
            auth = tmp / 'auth.json'
            auth.write_text('{"opaque": true}', encoding='utf-8')
            seen = {}

            def fake_codex(args, **kwargs):
                home = Path(kwargs['env']['CODEX_HOME'])
                seen['home'] = home
                seen['args'] = args
                seen['auth_copied'] = (home / 'auth.json').read_bytes() == auth.read_bytes()
                seen['config'] = (home / 'config.toml').read_text(encoding='utf-8')
                last = Path(args[args.index('--output-last-message') + 1])
                last.write_text('{"summary":"server struct checked","verdict":"pass","findings":[],'
                                '"out_of_scope_changes":[],"contract_warning_resolutions":[]}', encoding='utf-8')
                return subprocess.CompletedProcess(args, 0, stdout=json.dumps(
                    {'type': 'item.completed', 'item': {'type': 'command_execution', 'command': 'type a.go', 'exit_code': 0}}),
                    stderr='')
            run = self._run_obj(tmp)
            with patch.object(lane, 'WORKTREE_PARENT', tmp), patch.object(lane, 'CODEX_AUTH_SOURCE', auth), \
                    patch.object(lane, 'resolve_executable', return_value='codex.exe'), \
                    patch.object(lane, 'save_noop', create=True):
                run.save = lambda name, value: None
                verdict, receipt = run.call_codex_reviewer('02-reviewer', 'review this', run.worktree, runner=fake_codex)
            self.assertEqual(verdict['verdict'], 'pass')
            self.assertTrue(seen['auth_copied'])
            self.assertIn('approval_policy = "never"', seen['config'])
            self.assertIn('sandbox_mode = "read-only"', seen['config'])
            self.assertEqual(seen['args'][seen['args'].index('-m') + 1], lane.CODEX_MODEL)
            self.assertIn(f'model_reasoning_effort={lane.CODEX_EFFORT}', seen['args'])
            self.assertFalse(seen['home'].exists())
            self.assertEqual((receipt['provider'], receipt['model'], receipt['effort']),
                             ('codex', lane.CODEX_MODEL, lane.CODEX_EFFORT))
            self.assertEqual(len(receipt['read_commands']), 1)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_forbidden_reviewer_tools_fail_the_review(self):
        tmp = Path(tempfile.mkdtemp(prefix='lane-codex-'))
        try:
            auth = tmp / 'auth.json'
            auth.write_text('{}', encoding='utf-8')

            def searching_codex(args, **kwargs):
                Path(args[args.index('--output-last-message') + 1]).write_text(
                    '{"summary":"guessed from the web","verdict":"pass","findings":[],'
                    '"out_of_scope_changes":[],"contract_warning_resolutions":[]}', encoding='utf-8')
                return subprocess.CompletedProcess(args, 0, stdout=json.dumps(
                    {'type': 'item.completed', 'item': {'type': 'web_search'}}), stderr='')
            run = self._run_obj(tmp)
            run.save = lambda name, value: None
            with patch.object(lane, 'WORKTREE_PARENT', tmp), patch.object(lane, 'CODEX_AUTH_SOURCE', auth), \
                    patch.object(lane, 'resolve_executable', return_value='codex.exe'):
                with self.assertRaisesRegex(RuntimeError, 'forbidden tools'):
                    run.call_codex_reviewer('02-reviewer', 'review', run.worktree, runner=searching_codex)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_missing_codex_is_reported_as_unavailable_for_fallback(self):
        tmp = Path(tempfile.mkdtemp(prefix='lane-codex-'))
        try:
            run = self._run_obj(tmp)
            with patch.object(lane, 'resolve_executable', side_effect=RuntimeError('required executable is not available: codex')):
                with self.assertRaises(lane.CodexUnavailable):
                    run.call_codex_reviewer('02-reviewer', 'review', run.worktree)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class MemoryWaitTests(unittest.TestCase):
    def test_waits_with_backoff_until_the_host_recovers(self):
        low = {'available_physical_gb': 1.0, 'available_commit_gb': 1.0}
        ok = {'available_physical_gb': 5.0, 'available_commit_gb': 5.0}
        statuses, sleeps = iter([low, low, ok]), []
        record = lane.wait_for_memory('executor', max_wait=10_000, sleep=sleeps.append,
                                      status_fn=lambda: next(statuses))
        self.assertTrue(record['admitted'])
        self.assertEqual(record['checks'], 3)
        self.assertEqual(sleeps, [15.0, 30.0])

    def test_gives_up_only_after_the_maximum_wait(self):
        low = {'available_physical_gb': 1.0, 'available_commit_gb': 1.0}
        record = lane.wait_for_memory('executor', max_wait=0, sleep=lambda s: None,
                                      status_fn=lambda: low)
        self.assertFalse(record['admitted'])
        self.assertEqual(len(record['reasons']), 2)

    def test_the_sampler_measures_its_own_process_tree(self):
        self.assertGreater(lane.process_tree_working_set_mb(os.getpid()), 1)
        sampler = lane.MemorySampler(interval=0.05).start()
        sampler.phase = 'probe'
        time.sleep(0.3)
        report = sampler.stop()
        self.assertGreater(report['peak_lane_process_tree_mb'], 1)
        self.assertIn('probe', report['by_phase'])


class WarmInstallTests(unittest.TestCase):
    """Real junctions in throwaway directories; no pnpm, no provider."""

    def setUp(self):
        import _winapi
        self.junction = _winapi.CreateJunction
        self.tmp = Path(tempfile.mkdtemp(prefix='lane-warm-'))
        self.warm = self.tmp / 'warm'
        real_pkg = self.warm / 'node_modules' / '.pnpm' / 'dep@1.0.0' / 'node_modules' / 'dep'
        real_pkg.mkdir(parents=True)
        (real_pkg / 'index.js').write_text('module.exports = 1\n', encoding='utf-8')
        self.junction(str(real_pkg), str(self.warm / 'node_modules' / 'dep'))
        (self.warm / 'node_modules' / '.modules.yaml').write_text('layout: 5\n', encoding='utf-8')
        (self.warm / 'packages' / 'core').mkdir(parents=True)
        (self.warm / 'packages' / 'core' / 'stale.ts').write_text('old\n', encoding='utf-8')
        (self.warm / 'node_modules' / '@multica').mkdir()
        self.junction(str(self.warm / 'packages' / 'core'), str(self.warm / 'node_modules' / '@multica' / 'core'))
        (self.warm / 'packages' / 'core' / 'node_modules').mkdir()
        self.junction(str(real_pkg), str(self.warm / 'packages' / 'core' / 'node_modules' / 'dep'))
        self.task = self.tmp / 'task'
        (self.task / 'packages' / 'core').mkdir(parents=True)
        (self.task / 'packages' / 'core' / 'current.ts').write_text('new\n', encoding='utf-8')

    def tearDown(self):
        lane.remove_junctions_under(self.tmp, max_depth=8)
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_links_reach_dependencies_and_the_task_own_workspace_source(self):
        created = lane.link_dependencies(self.task, self.warm)
        self.assertTrue((self.task / 'node_modules' / 'dep' / 'index.js').is_file())
        self.assertTrue((self.task / 'packages' / 'core' / 'node_modules' / 'dep' / 'index.js').is_file())
        # The workspace package resolves to the task's source, never the warm copy.
        self.assertTrue((self.task / 'node_modules' / '@multica' / 'core' / 'current.ts').is_file())
        self.assertFalse((self.task / 'node_modules' / '@multica' / 'core' / 'stale.ts').exists())
        self.assertEqual(created['redirected_to_task'], ['node_modules/@multica/core'])
        self.assertTrue((self.task / 'node_modules' / '.modules.yaml').is_file())

    def test_unlinking_and_deleting_the_task_never_touches_the_warm_install(self):
        before = lane.dependency_fingerprint(self.warm)
        created = lane.link_dependencies(self.task, self.warm)
        # A test tool's cache written inside a linked package directory.
        (self.task / 'packages' / 'core' / 'node_modules' / '.vite').mkdir()
        (self.task / 'packages' / 'core' / 'node_modules' / '.vite' / 'results.json').write_text('{}', encoding='utf-8')
        self.assertEqual(lane.unlink_dependencies(self.task, created), {'not_removed': []})
        self.assertFalse((self.task / 'node_modules').exists())
        lane.link_dependencies(self.task, self.warm)
        lane.remove_junctions_under(self.task)
        shutil.rmtree(self.task)
        self.assertTrue((self.warm / 'node_modules' / '.pnpm' / 'dep@1.0.0' / 'node_modules' / 'dep' / 'index.js').is_file())
        self.assertEqual(lane.dependency_fingerprint(self.warm)['sha256'], before['sha256'])

    def test_a_dependency_edited_through_a_link_changes_the_fingerprint(self):
        before = lane.dependency_fingerprint(self.warm)
        lane.link_dependencies(self.task, self.warm)
        (self.task / 'node_modules' / 'dep' / 'index.js').write_text('module.exports = 2 // patched\n',
                                                                    encoding='utf-8')
        self.assertNotEqual(lane.dependency_fingerprint(self.warm)['sha256'], before['sha256'])

    def test_test_tool_caches_do_not_change_the_fingerprint(self):
        before = lane.dependency_fingerprint(self.warm)
        (self.warm / 'packages' / 'core' / 'node_modules' / '.vite').mkdir()
        (self.warm / 'packages' / 'core' / 'node_modules' / '.vite' / 'results.json').write_text('{}', encoding='utf-8')
        self.assertEqual(lane.dependency_fingerprint(self.warm)['sha256'], before['sha256'])


if __name__ == '__main__':
    unittest.main()
