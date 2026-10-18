"""Smallest useful code-changing lane: isolated executor, independent reviewer.

The accepted publication workflow proved that two independent provider sessions
can verify and publish a frozen document. It cannot change code: its provider
child runs with no tools and no repository access. This lane is the smallest
extension that lets an agent actually edit code while keeping every safeguard
the publication workflow established.

Shape of one run:

  isolated task worktree -> executor session -> captured diff and focused tests
  -> independent reviewer session (read-only) -> at most one bounded correction
  -> cleanup

What is deliberately NOT here: production access, deployment, outbound
communication, Phone Farm, destructive data work, other repositories, git push,
and any permanent role, model or seat selection. The executor is confined to its
own worktree by the provider CLI's restricted mode, by the tool allowlist taken
from the run's authority record, and by an after-the-fact containment check.

Authority is minted fresh per run, expires, and is revoked at cleanup. Nothing
is reused between runs.
"""
import ctypes
import hashlib
import json
import os
import re
import secrets
import shutil
import subprocess
import sys
import time
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
LANE = Path(__file__).resolve().parent
RUNS = ROOT / '.qualification-evidence' / 'implementation-lane'
WORKTREE_PARENT = Path('C:/Users/a9sho/.codex/worktrees/implementation-lane')
PROVIDER = Path('C:/Users/a9sho/AppData/Roaming/npm/node_modules/@anthropic-ai/claude-code/bin/claude.exe')

MODEL = 'claude-sonnet-4-6'
EFFORT = 'medium'
AUTHORITY_MINUTES = 90

# The reviewer runs on a different provider from the executor. Every miss of the
# same-subscription Claude reviewer was an unchecked assumption about the real
# system; a second vendor's model is an independent reader. Model and effort are
# named explicitly and recorded in every receipt, never inherited.
REVIEWER_PROVIDER = 'codex'
CODEX_MODEL = 'gpt-5.6-luna'
CODEX_EFFORT = 'high'
CODEX_AUTH_SOURCE = Path.home() / '.codex' / 'auth.json'
# A fresh, minimal Codex home per review: no plugins, MCP servers, web search or
# personal instructions from the operator's own Codex setup. Commands may only
# read, because the sandbox is read-only and nothing asks for approval.
CODEX_REVIEWER_CONFIG = '\n'.join([
    f'model = "{CODEX_MODEL}"',
    f'model_reasoning_effort = "{CODEX_EFFORT}"',
    'approval_policy = "never"',
    'sandbox_mode = "read-only"',
    'web_search = "disabled"',
    '',
    '[windows]',
    'sandbox = "unelevated"',
    '',
    '[tools]',
    'web_search = false',
    '',
])
# Anything a reviewer does beyond reading files and answering.
CODEX_FORBIDDEN_ITEMS = ('web_search', 'mcp_tool_call', 'file_change', 'patch_apply')

# The executor may read, search, edit and run focused checks. It may not run
# arbitrary commands: Bash is granted only for the exact check commands this
# repository already uses. The reviewer gets no editing tool at all.
EXECUTOR_TOOLS = ['Read', 'Grep', 'Glob', 'Edit', 'Write', 'Bash']
EXECUTOR_ALLOWED = [
    'Bash(pnpm vitest:*)',
    'Bash(npx vitest:*)',
    'Bash(pnpm typecheck:*)',
    'Bash(node:*)',
    'Bash(git diff:*)',
    'Bash(git status:*)',
]
EXECUTOR_DISALLOWED = [
    'Bash(git push:*)', 'Bash(git commit:*)', 'Bash(git remote:*)', 'Bash(git config:*)',
    'Bash(docker:*)', 'Bash(curl:*)', 'Bash(wget:*)', 'Bash(ssh:*)', 'Bash(scp:*)',
    'Bash(rm:*)', 'Bash(del:*)', 'Bash(format:*)', 'Bash(psql:*)',
    'WebFetch', 'WebSearch', 'Task',
]
# Go tasks get the three go commands a Go change needs to be checked by its
# author. Before this the executor wrote Go it could not compile, and every Go
# change needed verifying by hand.
GO_ALLOWED = ['Bash(go build:*)', 'Bash(go vet:*)', 'Bash(go test:*)']
REVIEWER_TOOLS = ['Read', 'Grep', 'Glob']

# Environment for every command run inside a task worktree, by an agent or by
# the harness. A native PostgreSQL listens on localhost:5432 on this host and
# the server's DB-backed tests default to exactly that address, so without this
# a `go test` could write fixtures into a real database. Port 9 is the discard
# port; nothing listens there. The Go settings keep module resolution offline
# and restrain build parallelism, which exhausted host memory once before.
SANDBOX_ENV = {
    'DATABASE_URL': 'postgres://lane-no-database@127.0.0.1:9/none?sslmode=disable&connect_timeout=2',
    # No GOTOOLCHAIN=local: the host's go 1.26.1 must switch to the 1.26.6 the
    # module requires, which it does from the local cache with the proxy off.
    'GOPROXY': 'off',
    'GOFLAGS': '-mod=readonly -p=2',
}

# The host runs permanently near 90% memory (a kernel pool leak only a reboot
# clears, and the reboot is ruled out), so the lane waits for room before each
# heavy step instead of refusing the run. The floors sit above the 1.7 GB at
# which setup was once killed; the lane's own peak is lowered separately by the
# warm dependency install and single-worker tests.
MIN_AVAILABLE_PHYSICAL_GB = 2.5
MIN_AVAILABLE_COMMIT_GB = 3.0
MEMORY_WAIT_MAX_SECONDS = 1800
WARM_PARENT = Path('C:/Users/a9sho/.codex/worktrees/implementation-lane-warm')
FORBIDDEN_CAPABILITIES = [
    'browser', 'git_push', 'deployment', 'external_messaging',
    'credential_access', 'phone_farm', 'host_files_outside_workspace',
]

VERDICT_KEYS = {'summary', 'verdict', 'findings'}
# The reviewer must also list every change the task did not ask for, even a
# harmless one. In the first trial a reviewer passed an unrequested change
# without mentioning it.
REVIEWER_KEYS = VERDICT_KEYS | {'out_of_scope_changes', 'contract_warning_resolutions'}
# Six turns left no room to open the server code a claim depends on.
REVIEWER_MAX_TURNS = 25
TEST_SUFFIXES = ('_test.go', '.test.ts', '.test.tsx', '.spec.ts', '.test.mjs', '_test.py')
# Python test modules are named by prefix (pytest's default `test_*.py`), which
# a suffix list cannot express. Without this a task in a Python repository such
# as the CRM could never pass the fails-before check.
PYTHON_TEST_PREFIX = 'test_'


def utc():
    return datetime.now(timezone.utc)


def stamp():
    return utc().strftime('%Y%m%dT%H%M%SZ')


def sha(path: Path) -> str:
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def source_sha(path: Path) -> str:
    """Hash of a source file with line endings normalised. Git on Windows checks
    these files out with CRLF, so a raw-byte hash would break the freeze in
    every fresh checkout without any real change."""
    return hashlib.sha256(Path(path).read_bytes().replace(b'\r\n', b'\n')).hexdigest()


def resolve_executable(name: str) -> str:
    """Resolve a command to a real path. On Windows pnpm/npx are .CMD shims that
    subprocess cannot find without a shell, and running through a shell would
    widen what the child can do."""
    found = shutil.which(name)
    if found:
        return found
    raise RuntimeError(f'required executable is not available: {name}')


# Variables never passed to anything the lane starts. The operator's shell holds
# connection strings for other applications' databases and this session's own
# messaging token; an executor allowed to run node could read either. Matching
# is on the name, so a new credential variable is dropped without a code change.
WITHHELD_ENV_NAME = re.compile(
    r'(DATABASE_URL|DSN|TOKEN|SECRET|PASSWORD|PASSWD|API_KEY|PRIVATE_KEY|CREDENTIAL'
    r'|^CLAUDE_CODE_MESSAGING_)', re.I)


def sandbox_env():
    kept = {name: value for name, value in os.environ.items()
            if not WITHHELD_ENV_NAME.search(name)}
    return {**kept, **SANDBOX_ENV}


def withheld_env_names():
    return sorted(name for name in os.environ if WITHHELD_ENV_NAME.search(name))


def run(args, cwd=None, timeout=900, check=True):
    args = [resolve_executable(args[0]), *args[1:]]
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True,
                            encoding='utf-8', errors='replace', timeout=timeout,
                            env=sandbox_env())
    if check and result.returncode:
        raise RuntimeError(f'command failed ({result.returncode}): {args[0]}\n{result.stderr[-2000:]}')
    return result


def executor_allowed(task) -> list:
    allowed = list(EXECUTOR_ALLOWED)
    if 'go' in task.get('toolchains', []):
        allowed += GO_ALLOWED
    return allowed


class _MemoryStatus(ctypes.Structure):
    _fields_ = [('length', ctypes.c_ulong), ('load', ctypes.c_ulong),
                ('total_physical', ctypes.c_ulonglong), ('available_physical', ctypes.c_ulonglong),
                ('total_commit', ctypes.c_ulonglong), ('available_commit', ctypes.c_ulonglong),
                ('total_virtual', ctypes.c_ulonglong), ('available_virtual', ctypes.c_ulonglong),
                ('available_extended', ctypes.c_ulonglong)]


def memory_status() -> dict:
    status = _MemoryStatus()
    status.length = ctypes.sizeof(_MemoryStatus)
    if not ctypes.windll.kernel32.GlobalMemoryStatusEx(ctypes.byref(status)):
        raise RuntimeError('host memory status is unavailable')
    gb = 1024 ** 3
    return {
        'observed_at': utc().isoformat(),
        'memory_load_percent': status.load,
        'available_physical_gb': round(status.available_physical / gb, 2),
        'total_physical_gb': round(status.total_physical / gb, 2),
        'available_commit_gb': round(status.available_commit / gb, 2),
        'total_commit_gb': round(status.total_commit / gb, 2),
    }


def memory_refusals(status: dict) -> list:
    """Reasons the host cannot take a run now; empty when it can."""
    reasons = []
    if status['available_physical_gb'] < MIN_AVAILABLE_PHYSICAL_GB:
        reasons.append(f"available physical memory {status['available_physical_gb']} GB "
                       f"is below {MIN_AVAILABLE_PHYSICAL_GB} GB")
    if status['available_commit_gb'] < MIN_AVAILABLE_COMMIT_GB:
        reasons.append(f"available commit {status['available_commit_gb']} GB "
                       f"is below {MIN_AVAILABLE_COMMIT_GB} GB")
    return reasons


def wait_for_memory(phase: str, max_wait: float = None, sleep=time.sleep, status_fn=None) -> dict:
    """Wait with backoff until the host has room for the next heavy step.

    Returns a record of the wait. Only a wait longer than max_wait fails, and
    that failure is a host result, not a task result."""
    status_fn = status_fn or memory_status
    max_wait = MEMORY_WAIT_MAX_SECONDS if max_wait is None else max_wait
    started, delay, checks = time.monotonic(), 15.0, 0
    while True:
        status = status_fn()
        checks += 1
        reasons = memory_refusals(status)
        waited = round(time.monotonic() - started, 1)
        if not reasons:
            return {'phase': phase, 'admitted': True, 'waited_seconds': waited,
                    'checks': checks, 'status': status}
        if waited >= max_wait:
            return {'phase': phase, 'admitted': False, 'waited_seconds': waited,
                    'checks': checks, 'status': status, 'reasons': reasons}
        sleep(min(delay, max(1.0, max_wait - waited)))
        delay = min(delay * 2, 120.0)


class _ProcessEntry(ctypes.Structure):
    _fields_ = [('size', ctypes.c_ulong), ('usage', ctypes.c_ulong), ('pid', ctypes.c_ulong),
                ('heap', ctypes.c_size_t), ('module', ctypes.c_ulong), ('threads', ctypes.c_ulong),
                ('parent', ctypes.c_ulong), ('priority', ctypes.c_long), ('flags', ctypes.c_ulong),
                ('exe', ctypes.c_char * 260)]


class _MemoryCounters(ctypes.Structure):
    _fields_ = [('cb', ctypes.c_ulong), ('faults', ctypes.c_ulong),
                ('peak_working_set', ctypes.c_size_t), ('working_set', ctypes.c_size_t),
                ('a', ctypes.c_size_t), ('b', ctypes.c_size_t), ('c', ctypes.c_size_t),
                ('d', ctypes.c_size_t), ('pagefile', ctypes.c_size_t), ('peak_pagefile', ctypes.c_size_t)]


def process_tree_working_set_mb(root_pid: int) -> float:
    """Working set of a process and all its descendants, in MB."""
    kernel32, psapi = ctypes.windll.kernel32, ctypes.windll.psapi
    kernel32.CreateToolhelp32Snapshot.restype = ctypes.c_void_p
    snapshot = kernel32.CreateToolhelp32Snapshot(0x2, 0)
    children = {}
    try:
        entry = _ProcessEntry()
        entry.size = ctypes.sizeof(_ProcessEntry)
        ok = kernel32.Process32First(ctypes.c_void_p(snapshot), ctypes.byref(entry))
        while ok:
            children.setdefault(entry.parent, []).append(entry.pid)
            ok = kernel32.Process32Next(ctypes.c_void_p(snapshot), ctypes.byref(entry))
    finally:
        kernel32.CloseHandle(ctypes.c_void_p(snapshot))
    tree, stack = set(), [root_pid]
    while stack:
        pid = stack.pop()
        if pid in tree:
            continue
        tree.add(pid)
        stack.extend(children.get(pid, []))
    total = 0
    kernel32.OpenProcess.restype = ctypes.c_void_p
    for pid in tree:
        handle = kernel32.OpenProcess(0x1000 | 0x0010, False, pid)
        if not handle:
            continue
        try:
            counters = _MemoryCounters()
            counters.cb = ctypes.sizeof(_MemoryCounters)
            if psapi.GetProcessMemoryInfo(ctypes.c_void_p(handle), ctypes.byref(counters), counters.cb):
                total += counters.working_set
        finally:
            kernel32.CloseHandle(ctypes.c_void_p(handle))
    return round(total / (1024 * 1024), 1)


RUN_LOCK = RUNS / 'lane-run.lock'
_STILL_ACTIVE = 259


def process_alive(pid: int) -> bool:
    """True when a process with this id is still running on this host."""
    if pid <= 0:
        return False
    kernel32 = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel32.OpenProcess.restype = ctypes.c_void_p
    handle = kernel32.OpenProcess(0x1000, False, pid)
    if not handle:
        return False
    try:
        code = ctypes.c_ulong()
        if not kernel32.GetExitCodeProcess(ctypes.c_void_p(handle), ctypes.byref(code)):
            return False
        return code.value == _STILL_ACTIVE
    finally:
        kernel32.CloseHandle(ctypes.c_void_p(handle))


def acquire_run_lock(run_id: str, lock: Path = None) -> dict:
    """Allow one lane run on this host at a time.

    Every run sweeps the shared worktree directory before it starts. A second
    run started while another is working would see the first run's worktree:
    a clean one of the same repository is removed from under it, and one of a
    different repository blocks admission with a misleading orphan reason. The
    owner's first issue proof hit the second case while a CRM run was active.
    A lock left by a process that is no longer running is taken over.
    """
    lock = lock or RUN_LOCK
    lock.parent.mkdir(parents=True, exist_ok=True)
    record = {'pid': os.getpid(), 'run_id': run_id, 'acquired_at': utc().isoformat()}
    for _attempt in range(2):
        try:
            with open(lock, 'x', encoding='utf-8') as handle:
                json.dump(record, handle)
            return {'acquired': True, **record}
        except FileExistsError:
            try:
                holder = json.loads(lock.read_text(encoding='utf-8'))
            except (OSError, ValueError):
                holder = {}
            if process_alive(int(holder.get('pid') or 0)):
                return {'acquired': False, 'holder': holder}
            lock.unlink(missing_ok=True)
    return {'acquired': False, 'holder': {}}


def release_run_lock(run_id: str, lock: Path = None) -> None:
    lock = lock or RUN_LOCK
    try:
        holder = json.loads(lock.read_text(encoding='utf-8'))
    except (OSError, ValueError):
        return
    if holder.get('run_id') == run_id and holder.get('pid') == os.getpid():
        lock.unlink(missing_ok=True)


class MemorySampler:
    """Samples the lane's process tree and the host every few seconds, so each
    run records what it really needs rather than what the host had free."""

    def __init__(self, interval: float = 3.0, root_pid: int = None):
        import threading
        self.interval, self.root_pid = interval, root_pid or os.getpid()
        self.phase, self.samples, self.by_phase = 'start', 0, {}
        self.peak_tree_mb, self.min_available_gb, self.max_load = 0.0, None, 0
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._loop, daemon=True)

    def _sample(self):
        tree = process_tree_working_set_mb(self.root_pid)
        host = memory_status()
        self.samples += 1
        self.peak_tree_mb = max(self.peak_tree_mb, tree)
        available = host['available_physical_gb']
        self.min_available_gb = available if self.min_available_gb is None else min(self.min_available_gb, available)
        self.max_load = max(self.max_load, host['memory_load_percent'])
        phase = self.by_phase.setdefault(self.phase, {'peak_tree_mb': 0.0, 'min_available_gb': available})
        phase['peak_tree_mb'] = max(phase['peak_tree_mb'], tree)
        phase['min_available_gb'] = min(phase['min_available_gb'], available)

    def _loop(self):
        while not self._stop.is_set():
            try:
                self._sample()
            except Exception:  # sampling must never break a run
                pass
            self._stop.wait(self.interval)

    def start(self):
        self._thread.start()
        return self

    def stop(self) -> dict:
        self._stop.set()
        self._thread.join(timeout=10)
        return {'samples': self.samples, 'interval_seconds': self.interval,
                'peak_lane_process_tree_mb': self.peak_tree_mb,
                'min_host_available_gb': self.min_available_gb,
                'max_host_load_percent': self.max_load, 'by_phase': self.by_phase}


def lockfile_key(base_ref: str, install_scope: str) -> str:
    lock = run(['git', '-C', str(ROOT), 'show', f'{base_ref}:pnpm-lock.yaml']).stdout
    return hashlib.sha256((lock + '\0' + (install_scope or 'all')).encode('utf-8')).hexdigest()[:16]


def ensure_warm_install(base_ref: str, install_scope: str) -> dict:
    """One offline dependency install per lockfile and scope, reused by runs.

    A fresh install per run was the largest and most memory-hungry step and was
    killed by the host more than once. The warm copy is a detached worktree of
    the base ref, used only as the target of links from task worktrees."""
    key = lockfile_key(base_ref, install_scope)
    warm = WARM_PARENT / key
    ready = warm / '.lane-warm-ready'
    if ready.is_file():
        return {'path': warm, 'key': key, 'reused': True, 'install_seconds': 0}
    if warm.exists():
        discard_warm_install(warm)
    WARM_PARENT.mkdir(parents=True, exist_ok=True)
    run(['git', '-C', str(ROOT), 'worktree', 'add', '--detach', str(warm), base_ref])
    started = time.time()
    command = ['pnpm', 'install', '--offline', '--frozen-lockfile', '--ignore-scripts',
               '--child-concurrency=1', '--network-concurrency=1']
    if install_scope:
        command += ['--filter', install_scope]
    run(command, cwd=str(warm), timeout=3600)
    ready.write_text(key + '\n', encoding='utf-8')
    return {'path': warm, 'key': key, 'reused': False,
            'install_seconds': round(time.time() - started, 3)}


def discard_warm_install(warm: Path) -> None:
    run(['git', '-C', str(ROOT), 'worktree', 'remove', '--force', str(warm)], check=False)
    if warm.exists():
        shutil.rmtree('\\\\?\\' + str(warm).replace('/', '\\'), ignore_errors=True)
    run(['git', '-C', str(ROOT), 'worktree', 'prune'], check=False)


def dependency_dirs(warm: Path) -> list:
    """Relative paths of node_modules directories that belong to the workspace
    itself (root and each package), not ones nested inside node_modules."""
    found = []
    for current, dirs, _ in os.walk(warm):
        rel = Path(current).relative_to(warm)
        if 'node_modules' in rel.parts or '.git' in rel.parts or len(rel.parts) > 3:
            dirs[:] = []
            continue
        if 'node_modules' in dirs:
            found.append((rel / 'node_modules').as_posix())
        dirs[:] = [d for d in dirs if d not in ('node_modules', '.git')]
    return sorted(found)


def _inside(path: str, root: Path) -> bool:
    try:
        Path(path).relative_to(root)
        return True
    except ValueError:
        return False


def link_dependencies(worktree: Path, warm: Path) -> dict:
    """Give the task worktree the warm install's dependencies without copying
    them.

    Each workspace node_modules becomes a real directory whose entries are
    junctions. Third-party entries point into the warm install. Entries that
    pnpm links to a workspace package's own source (for example
    node_modules/@multica/core) point at that package inside the task worktree
    instead, so tests never run against the warm copy's stale source. Small
    loose files pnpm keeps there are copied. Everything created is returned so
    it can be removed before the worktree."""
    import _winapi
    created = {'dirs': [], 'junctions': [], 'files': [], 'redirected_to_task': []}
    warm_resolved = Path(os.path.realpath(warm))
    for rel in dependency_dirs(warm):
        source_dir, target_dir = warm / rel, worktree / rel
        if not target_dir.parent.is_dir() or os.path.lexists(target_dir):
            continue
        target_dir.mkdir()
        created['dirs'].append(rel)
        entries = []
        for name in os.listdir(source_dir):
            if name.startswith('@') and os.path.isdir(source_dir / name) \
                    and not os.path.isjunction(source_dir / name):
                (target_dir / name).mkdir()
                created['dirs'].append(f'{rel}/{name}')
                entries += [f'{name}/{inner}' for inner in os.listdir(source_dir / name)]
            else:
                entries.append(name)
        for entry in entries:
            source, target = source_dir / entry, target_dir / entry
            entry_rel = f'{rel}/{entry}'
            real = os.path.realpath(source)
            if os.path.isfile(source) and not os.path.islink(source):
                shutil.copy2(source, target)
                created['files'].append(entry_rel)
                continue
            if _inside(real, warm_resolved) and 'node_modules' not in Path(real).relative_to(warm_resolved).parts:
                # A workspace package's own source: use the task worktree's copy.
                real = str(worktree / Path(real).relative_to(warm_resolved))
                created['redirected_to_task'].append(entry_rel)
            _winapi.CreateJunction(real, str(target))
            created['junctions'].append(entry_rel)
    return created


def unlink_dependencies(worktree: Path, created: dict) -> dict:
    """Remove only what link_dependencies made. Junctions are removed with
    rmdir, which never touches their target; a path that is no longer a
    junction is left alone and reported."""
    left = []
    for rel in created.get('junctions', []):
        path = worktree / rel
        if os.path.isjunction(path):
            os.rmdir(path)
        elif os.path.lexists(path):
            left.append(rel)
    for rel in created.get('files', []):
        path = worktree / rel
        if os.path.isfile(path) and not os.path.islink(path):
            os.remove(path)
    for rel in sorted(created.get('dirs', []), key=lambda r: -r.count('/')):
        path = worktree / rel
        if not path.is_dir() or os.path.isjunction(path):
            continue
        # Test tools write caches (such as .vite) into these directories. They
        # belong to the task worktree, and every junction inside has already
        # been removed, so clearing them cannot reach the warm install.
        remove_junctions_under(path)
        shutil.rmtree(path, ignore_errors=True)
        if path.exists():
            left.append(rel)
    return {'not_removed': left}


def remove_junctions_under(root: Path, max_depth: int = 5) -> int:
    """Remove every junction under root without following any. Used before
    deleting a worktree whose link record was lost, such as after a kill."""
    removed = 0
    for current, dirs, _ in os.walk(root):
        depth = len(Path(current).relative_to(root).parts)
        keep = []
        for name in dirs:
            path = os.path.join(current, name)
            if os.path.isjunction(path):
                os.rmdir(path)
                removed += 1
            elif depth < max_depth:
                keep.append(name)
        dirs[:] = keep
    return removed


# Tool caches written inside node_modules during tests; not dependency content.
FINGERPRINT_SKIP_DIRS = {'.vite', '.vitest', '.cache'}


def dependency_fingerprint(warm: Path) -> dict:
    """Size and mtime of every regular file in the warm install's own
    node_modules trees. pnpm's internal links are not followed, so each file is
    counted once where it really lives."""
    digest, files, started = hashlib.sha256(), 0, time.time()
    for rel in dependency_dirs(warm):
        top = warm / rel
        for current, dirs, names in os.walk(top):
            dirs[:] = sorted(d for d in dirs if d not in FINGERPRINT_SKIP_DIRS
                             and not os.path.isjunction(os.path.join(current, d))
                             and not os.path.islink(os.path.join(current, d)))
            for name in sorted(names):
                path = os.path.join(current, name)
                try:
                    info = os.lstat(path)
                except OSError:
                    continue
                digest.update(f'{os.path.relpath(path, warm)}|{info.st_size}|{info.st_mtime_ns}\n'.encode('utf-8'))
                files += 1
    return {'sha256': digest.hexdigest(), 'files': files, 'seconds': round(time.time() - started, 2)}


def worktree_status_paths(worktree: Path) -> set:
    """Every changed or untracked path git status reports, ignored files aside."""
    out = run(['git', '-C', str(worktree), 'status', '--porcelain=v1', '-z',
               '--untracked-files=all'], check=False).stdout
    paths, entries, index = set(), out.split('\0'), 0
    while index < len(entries):
        entry = entries[index]
        index += 1
        if len(entry) < 4:
            continue
        paths.add(entry[3:])
        if entry[0] in 'RC':  # a rename or copy carries its source as the next entry
            index += 1
    return paths


def ignored_entries(worktree: Path) -> set:
    out = run(['git', '-C', str(worktree), 'status', '--porcelain=v1', '-z',
               '--ignored=matching', '--untracked-files=normal'], check=False).stdout
    return {entry[3:] for entry in out.split('\0') if entry.startswith('!! ')}


def patch_paths(patch: str) -> set:
    paths = set()
    for line in patch.splitlines():
        match = re.match(r'^diff --git a/(.+?) b/(.+)$', line)
        if match:
            paths.update({match.group(1), match.group(2)})
    return paths


def is_test_path(path: str) -> bool:
    name = path.rsplit('/', 1)[-1]
    return path.endswith(TEST_SUFFIXES) or (name.startswith(PYTHON_TEST_PREFIX) and name.endswith('.py'))


# Caches the repository's own check commands write, such as `pnpm typecheck`
# through turbo. They are rebuilt from source, so losing them loses no work.
# Every other ignored entry that appears during a run blocks deletion.
REGENERABLE_CACHE_DIRS = ('node_modules', '.turbo', '.next', '__pycache__', '.pytest_cache')


def is_regenerable_cache(entry: str) -> bool:
    parts = entry.rstrip('/').split('/')
    return any(part in REGENERABLE_CACHE_DIRS for part in parts) or entry.endswith('.tsbuildinfo')


def sweep_orphans() -> dict:
    """Clear what an interrupted earlier run left behind, before starting.

    A leftover worktree with no changes is removed. One holding changes, or a
    leftover directory git no longer knows, is never deleted: the run refuses
    to start so a person can look first.
    """
    run(['git', '-C', str(ROOT), 'worktree', 'prune'], check=False)
    listing = run(['git', '-C', str(ROOT), 'worktree', 'list', '--porcelain'], check=False).stdout
    parent = str(WORKTREE_PARENT).replace('\\', '/').lower()
    # Keyed case-insensitively for comparison, but git is always given the path
    # exactly as it reported it.
    registered = {}
    for line in listing.splitlines():
        if line.startswith('worktree '):
            path = line[len('worktree '):].replace('\\', '/')
            registered[path.lower()] = path
    removed, blocking = [], []
    for key in sorted(k for k in registered if k.startswith(parent + '/')):
        path = registered[key]
        dirty = worktree_status_paths(Path(path))
        if dirty:
            blocking.append({'path': path, 'reason': 'registered worktree holds changes',
                             'changed_path_count': len(dirty)})
            continue
        remove_junctions_under(Path(path))
        run(['git', '-C', str(ROOT), 'worktree', 'remove', '--force', path], check=False)
        if Path(path).exists():
            blocking.append({'path': path, 'reason': 'clean worktree could not be removed'})
        else:
            removed.append(path)
    if WORKTREE_PARENT.exists():
        for child in WORKTREE_PARENT.iterdir():
            key = str(child).replace('\\', '/').lower()
            if key not in registered:
                blocking.append({'path': key, 'reason': 'directory is not a registered worktree'})
    return {'removed': removed, 'blocking': blocking}


def write_evidence(path: Path, text: str) -> None:
    """Write a patch or diff exactly as git produced it. Text mode on Windows
    turned every newline into CRLF, so saved patches did not apply."""
    Path(path).write_bytes(text.encode('utf-8'))


# A zod field is optional if its own line says so; anything else is required.
ZOD_OPTIONAL_MARKERS = ('.optional(', '.nullable(', '.nullish(', '.default(', '.catch(')
ZOD_FIELD = re.compile(r'^\+\s*"?([A-Za-z_][A-Za-z0-9_]*)"?\s*:\s*z\s*\.')
GO_JSON_TAG = re.compile(r'`[^`]*json:"([A-Za-z_][A-Za-z0-9_]*)((?:,[a-z]+)*)"')
# Only packages that shape HTTP and realtime responses. In trial 3 every warning
# drawn from agent adapters, third-party clients and projections was a name
# collision; the one real defect (trial 2) came from pkg/protocol.
RESPONSE_PACKAGES = ('internal/handler', 'pkg/protocol')


def go_json_tags(server_root: Path) -> dict:
    """Every JSON field name the server's response packages serialise, with
    where it is omitempty and where it is always present. Test files are
    ignored."""
    tags = {}
    base = Path(server_root).parent
    paths = [p for package in RESPONSE_PACKAGES
             for p in (Path(server_root) / package).rglob('*.go')]
    for path in paths:
        if path.name.endswith('_test.go'):
            continue
        try:
            lines = path.read_text(encoding='utf-8', errors='replace').splitlines()
        except OSError:
            continue
        for number, line in enumerate(lines, 1):
            for match in GO_JSON_TAG.finditer(line):
                name, options = match.group(1), match.group(2)
                where = f'{path.relative_to(base).as_posix()}:{number}'
                entry = tags.setdefault(name, {'omitempty': [], 'always': []})
                entry['omitempty' if 'omitempty' in options else 'always'].append(where)
    return tags


def server_contract_check(patch: str, server_root: Path) -> dict:
    """Mechanical check of client response schemas against the real server.

    Both trials' reviewer misses were claims about the real system that nobody
    checked. Trial 2's was a zod schema that required issue_id, which the Go
    struct marks omitempty and leaves out for chat tasks. For every field a
    patch adds to a TypeScript zod schema as required, this looks the name up
    in the server's JSON tags; a name that is omitempty anywhere becomes a
    warning the reviewer must resolve against the actual endpoint.
    """
    warnings, current_file, tags = [], None, None
    # A field line removed and re-added in the same file was moved, not added;
    # trial 3's schema extraction produced six warnings that way.
    removed, file_for_removed = set(), None
    for line in patch.splitlines():
        if line.startswith('--- '):
            continue
        if line.startswith('+++ '):
            file_for_removed = line[6:] if line.startswith('+++ b/') else None
        elif line.startswith('-') and file_for_removed:
            removed.add((file_for_removed, line[1:].strip().rstrip(',')))
    for line in patch.splitlines():
        if line.startswith('+++ '):
            current_file = line[6:] if line.startswith('+++ b/') else None
            continue
        if not current_file or not current_file.endswith(('.ts', '.tsx')) or is_test_path(current_file):
            continue
        match = ZOD_FIELD.match(line)
        if not match or any(marker in line for marker in ZOD_OPTIONAL_MARKERS):
            continue
        if (current_file, line[1:].strip().rstrip(',')) in removed:
            continue
        if tags is None:
            tags = go_json_tags(server_root)
        field = match.group(1)
        entry = tags.get(field)
        if entry and entry['omitempty']:
            warnings.append({
                'client_file': current_file,
                'field': field,
                'client_line': line[1:].strip()[:160],
                'server_omitempty_at': entry['omitempty'][:8],
                'server_always_present_at': entry['always'][:8],
            })
    return {'required_fields_that_the_server_may_omit': warnings, 'warning_count': len(warnings)}


def freeze_check(freeze_file: Path = None) -> dict:
    """Compare the lane's files with the hashes recorded when it was frozen.

    A trial measures one unchanged lane. If any frozen file differs, a run
    refuses to start rather than silently measuring something else."""
    freeze_file = freeze_file or LANE / 'FROZEN.json'
    record = json.loads(freeze_file.read_text(encoding='utf-8'))
    base = freeze_file.resolve().parents[2]
    changed = sorted(relative for relative, expected in record['source_hashes'].items()
                     if not (base / relative).is_file()
                     or source_sha(base / relative) != expected)
    return {'freeze_version': record['freeze_version'], 'changed_files': changed,
            'intact': not changed}


def cleanup_guard(worktree: Path, captured_patch: str, ignored_before) -> dict:
    """Decide whether a worktree may be deleted without losing agent work.

    The capture uses git diff; this check uses git status, a different view of
    the same tree, so a gap in the capture method is caught rather than
    repeated. In the first trial a run passed every check and its two new files
    were deleted with the worktree. Anything present in the tree but absent
    from the captured patch, or any new ignored entry, keeps the worktree and
    fails the run.
    """
    present = worktree_status_paths(worktree)
    captured = patch_paths(captured_patch)
    recapture = capture_diff(worktree)['diff']
    # None means no agent session ran, so every ignored entry is harness setup.
    appeared = ([] if ignored_before is None
                else sorted(ignored_entries(worktree) - set(ignored_before)))
    caches = [entry for entry in appeared if is_regenerable_cache(entry)]
    new_ignored = [entry for entry in appeared if not is_regenerable_cache(entry)]
    report = {
        'paths_present_but_not_captured': sorted(present - captured),
        'capture_differs_from_tree': recapture != captured_patch,
        'new_ignored_entries': new_ignored,
        'regenerable_cache_entries': caches,
    }
    report['safe_to_delete'] = not (report['paths_present_but_not_captured']
                                    or report['capture_differs_from_tree']
                                    or new_ignored)
    return report


class Run:
    def __init__(self, task_id: str):
        self.task_id = task_id
        self.run_id = f'{stamp()}-{secrets.token_hex(4)}'
        self.directory = RUNS / f'{task_id}-{self.run_id}'
        self.directory.mkdir(parents=True, exist_ok=False)
        self.worktree = WORKTREE_PARENT / f'{task_id}-{self.run_id}'
        self.events = []
        self.authority = None

    def save(self, name, value):
        target = self.directory / name
        target.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')
        return target

    def event(self, kind, **fields):
        record = {'at': utc().isoformat(), 'event': kind, **fields}
        self.events.append(record)
        print(json.dumps(record), flush=True)

    # -- authority -------------------------------------------------------
    def mint_authority(self, task):
        issued = utc()
        self.authority = {
            'authority_version': 1,
            'run_id': self.run_id,
            'task_id': self.task_id,
            'nonce': secrets.token_hex(16),
            'issued_at': issued.isoformat(),
            'expires_at': (issued + timedelta(minutes=AUTHORITY_MINUTES)).isoformat(),
            'revoked': False,
            'revoked_at': None,
            'worktree': str(self.worktree).replace('\\', '/'),
            'base_ref': task['base_ref'],
            'allowed_paths': task['allowed_paths'],
            'executor_tools': EXECUTOR_TOOLS,
            'executor_allowed_tool_patterns': EXECUTOR_ALLOWED,
            'executor_disallowed_tool_patterns': EXECUTOR_DISALLOWED,
            'reviewer_tools': REVIEWER_TOOLS,
            'forbidden_capabilities': FORBIDDEN_CAPABILITIES,
            'maximum_corrections': 1,
            'maximum_executor_turns': task.get('max_turns', 30),
            'model': MODEL,
            'effort': EFFORT,
            'production_access': False,
            'deployment': False,
            'outbound_communication': False,
            'phone_farm': False,
            'other_repositories': False,
            'permanent_selection': False,
        }
        self.save('authority.json', self.authority)
        self.event('authority_issued', expires_at=self.authority['expires_at'])

    def require_live_authority(self, purpose):
        a = self.authority
        if a is None or a['revoked']:
            raise RuntimeError('authority is revoked or absent')
        if utc() >= datetime.fromisoformat(a['expires_at']):
            raise RuntimeError('authority expired')
        self.event('authority_checked', purpose=purpose)

    def revoke_authority(self, reason):
        if self.authority and not self.authority['revoked']:
            self.authority['revoked'] = True
            self.authority['revoked_at'] = utc().isoformat()
            self.authority['revocation_reason'] = reason
            self.save('authority.json', self.authority)
            self.event('authority_revoked', reason=reason)

    # -- worktree --------------------------------------------------------
    def create_worktree(self, base_ref):
        WORKTREE_PARENT.mkdir(parents=True, exist_ok=True)
        if self.worktree.exists():
            raise RuntimeError('worktree path already exists')
        run(['git', '-C', str(ROOT), 'worktree', 'add', '--detach', str(self.worktree), base_ref])
        head = run(['git', '-C', str(self.worktree), 'rev-parse', 'HEAD']).stdout.strip()
        self.event('worktree_created', path=str(self.worktree).replace('\\', '/'), head=head)
        return head

    def prepare_worktree(self, install_scope=None):
        """Harness setup, not agent action: a fresh worktree has no node_modules,
        so the executor could not run the repository's own tests. The install is
        offline and lockfile-pinned, so it reaches no network and resolves
        exactly what the committed lockfile names."""
        started = time.time()
        # The host killed this step twice for low memory before any provider
        # session started, so it is deliberately restrained: serialised, and
        # optionally narrowed to the one workspace the task's verification
        # needs. This changes how much memory and time setup costs, not what the
        # agent is allowed to do.
        command = ['pnpm', 'install', '--offline', '--frozen-lockfile', '--ignore-scripts',
                   '--child-concurrency=1', '--network-concurrency=1']
        if install_scope:
            command += ['--filter', install_scope]
        run(command, cwd=str(self.worktree), timeout=3600)
        elapsed = round(time.time() - started, 3)
        self.event('worktree_dependencies_installed', elapsed_seconds=elapsed,
                   offline=True, scope=install_scope or 'all workspaces')
        return elapsed

    def remove_worktree(self):
        if not self.worktree.exists():
            return
        # Never let a recursive delete reach the shared warm install.
        remove_junctions_under(self.worktree)
        run(['git', '-C', str(ROOT), 'worktree', 'remove', '--force', str(self.worktree)], check=False)
        if self.worktree.exists():
            # The extended-length prefix reaches node_modules paths past 260
            # characters; without it a manual cleanup found the folder left
            # behind after git had already unregistered it.
            shutil.rmtree('\\\\?\\' + str(self.worktree).replace('/', '\\'), ignore_errors=True)
        run(['git', '-C', str(ROOT), 'worktree', 'prune'], check=False)
        self.event('worktree_removed', still_present=self.worktree.exists())

    # -- provider --------------------------------------------------------
    def call_provider(self, slot, prompt, tools, allowed, disallowed, max_turns, cwd):
        """One bounded provider session. Returns the parsed structured verdict."""
        self.require_live_authority(slot)
        args = [
            str(PROVIDER), '-p', '--restricted',
            '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
            # acceptEdits auto-approves edits inside the working directory and
            # nothing else. Without an explicit mode the CLI's default left the
            # session with "no approval surface available" and silently denied
            # every Edit and Write, which is how a correction session once
            # produced a byte-identical diff while reporting it was blocked.
            '--no-session-persistence', '--permission-mode', 'acceptEdits',
            '--permission-prompts', 'none',
            '--disable-slash-commands', '--no-chrome',
            '--output-format', 'stream-json', '--input-format', 'stream-json',
            '--verbose', '--max-turns', str(max_turns),
            '--model', MODEL, '--effort', EFFORT,
            '--tools', *tools,
        ]
        if allowed:
            args += ['--allowedTools', *allowed]
        if disallowed:
            args += ['--disallowedTools', *disallowed]

        payload = json.dumps({
            'type': 'user',
            'message': {'role': 'user', 'content': [{'type': 'text', 'text': prompt}]},
        }) + '\n'
        memory = memory_status()
        started = time.time()
        proc = subprocess.run(
            args, cwd=str(cwd), input=payload, capture_output=True, text=True,
            encoding='utf-8', errors='replace', timeout=1800, env=sandbox_env(),
        )
        elapsed = round(time.time() - started, 3)

        frames, models, result_line, assistant, rate_limits = [], set(), None, [], []
        for line in proc.stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                frame = json.loads(line)
            except json.JSONDecodeError:
                continue
            frames.append(frame.get('type'))
            if 'rate_limit' in str(frame.get('type')) or 'rate_limit' in str(frame.get('subtype')):
                rate_limits.append(frame)
            if frame.get('type') == 'assistant':
                message = frame.get('message', {})
                if message.get('model'):
                    models.add(message['model'])
                for block in message.get('content', []) or []:
                    if block.get('type') == 'text':
                        assistant.append(block['text'])
            if frame.get('type') == 'result':
                result_line = frame

        usage = {}
        if result_line:
            usage = {
                'duration_ms': result_line.get('duration_ms'),
                'num_turns': result_line.get('num_turns'),
                'total_cost_usd': result_line.get('total_cost_usd'),
                'usage': result_line.get('usage'),
                'is_error': result_line.get('is_error'),
                'subtype': result_line.get('subtype'),
                'api_error_status': result_line.get('api_error_status'),
                # Denials are what hid the permission defect in the first
                # trial; they are kept so a silent denial is visible afterwards.
                'permission_denials': result_line.get('permission_denials'),
            }
        receipt = {
            'slot': slot, 'run_id': self.run_id, 'recorded_at': utc().isoformat(),
            'elapsed_seconds': elapsed, 'exit_code': proc.returncode,
            'frame_types': sorted(set(f for f in frames if f)), 'models': sorted(models),
            'prompt_sha256': hashlib.sha256(prompt.encode('utf-8')).hexdigest(),
            'provider_result_seen': result_line is not None, 'usage': usage,
            'rate_limit_frames': rate_limits,
            'host_memory_before_session': memory,
            'stderr_tail': proc.stderr[-2000:],
            'sandbox_env_keys': sorted(SANDBOX_ENV),
            'withheld_env_names': withheld_env_names(),
        }
        self.save(f'{slot}.provider-receipt.json', receipt)
        self.event('provider_session_complete', slot=slot, elapsed_seconds=elapsed,
                   models=receipt['models'], exit_code=proc.returncode)

        keys = REVIEWER_KEYS if slot.startswith(('02-', '04-')) else VERDICT_KEYS
        verdict = parse_verdict('\n'.join(assistant), keys)
        self.save(f'{slot}.verdict.json', verdict)
        return verdict, receipt

    def call_codex_reviewer(self, slot, prompt, cwd, runner=None):
        """One read-only Codex review session in a task-scoped Codex home.

        The operator's Codex sign-in file is copied opaquely into a fresh home
        next to (never inside) the task worktree and deleted afterwards. The
        credential bytes are never parsed, printed or hashed."""
        self.require_live_authority(slot)
        try:
            codex = resolve_executable('codex')
        except RuntimeError as error:
            raise CodexUnavailable(str(error))
        if not CODEX_AUTH_SOURCE.is_file():
            raise CodexUnavailable('no Codex sign-in is available on this host')
        home = WORKTREE_PARENT / f'{self.task_id}-{self.run_id}-codex-home-{slot}'
        home.mkdir(parents=True, exist_ok=False)
        last = home / 'last-message.txt'
        started = time.time()
        memory = memory_status()
        try:
            shutil.copyfile(CODEX_AUTH_SOURCE, home / 'auth.json')
            (home / 'config.toml').write_text(CODEX_REVIEWER_CONFIG, encoding='utf-8')
            args = [codex, 'exec', '-m', CODEX_MODEL, '-c', f'model_reasoning_effort={CODEX_EFFORT}',
                    '-s', 'read-only', '--ephemeral', '--skip-git-repo-check', '-C', str(cwd),
                    '--json', '--output-last-message', str(last), '-']
            env = dict(sandbox_env(), CODEX_HOME=str(home))
            proc = (runner or subprocess.run)(args, input=prompt, capture_output=True, text=True,
                                               encoding='utf-8', errors='replace', timeout=1800, env=env)
            final_text = last.read_text(encoding='utf-8') if last.is_file() else ''
        finally:
            shutil.rmtree(home, ignore_errors=True)
        events = parse_codex_events(proc.stdout)
        receipt = {
            'slot': slot, 'run_id': self.run_id, 'recorded_at': utc().isoformat(),
            'provider': 'codex', 'model': CODEX_MODEL, 'effort': CODEX_EFFORT,
            'elapsed_seconds': round(time.time() - started, 3), 'exit_code': proc.returncode,
            'prompt_sha256': hashlib.sha256(prompt.encode('utf-8')).hexdigest(),
            'read_commands': events['commands'], 'forbidden_items': events['forbidden'],
            'item_types': events['item_types'], 'usage': events['usage'],
            'host_memory_before_session': memory,
            'stderr_tail': re.sub(r'\x1b\[[0-9;]*m', '', proc.stderr or '')[-2000:],
            'codex_home_removed': not home.exists(),
            'withheld_env_names': withheld_env_names(),
        }
        self.save(f'{slot}.provider-receipt.json', receipt)
        self.event('provider_session_complete', slot=slot, elapsed_seconds=receipt['elapsed_seconds'],
                   models=[CODEX_MODEL], provider='codex', exit_code=proc.returncode)
        if proc.returncode != 0 or not final_text.strip():
            raise CodexUnavailable(f'codex exited {proc.returncode} without a final answer')
        if events['forbidden']:
            raise RuntimeError(f"codex reviewer used forbidden tools: {sorted(set(events['forbidden']))}")
        verdict = parse_verdict(final_text, REVIEWER_KEYS)
        self.save(f'{slot}.verdict.json', verdict)
        return verdict, receipt


class CodexUnavailable(RuntimeError):
    """Codex could not produce a review at all, as opposed to producing a bad one."""


def parse_codex_events(stdout: str) -> dict:
    commands, forbidden, types, usage = [], [], {}, {}
    for line in (stdout or '').splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get('type') == 'turn.completed':
            for key, value in (event.get('usage') or {}).items():
                usage[key] = usage.get(key, 0) + (value or 0)
        if event.get('type') != 'item.completed':
            continue
        item = event.get('item') or {}
        kind = item.get('type') or 'unknown'
        types[kind] = types.get(kind, 0) + 1
        if kind == 'command_execution':
            commands.append({'command': str(item.get('command') or '')[:300], 'exit_code': item.get('exit_code')})
        if kind in CODEX_FORBIDDEN_ITEMS:
            forbidden.append(kind)
    return {'commands': commands, 'forbidden': forbidden, 'item_types': types, 'usage': usage}


def parse_verdict(text: str, keys=VERDICT_KEYS) -> dict:
    """Accept only the exact structured verdict, optionally inside a fence."""
    candidate = text.strip()
    fence = re.search(r'```(?:json)?\s*(\{.*?\})\s*```', candidate, re.S)
    if fence:
        candidate = fence.group(1)
    else:
        brace = candidate.find('{')
        if brace >= 0:
            candidate = candidate[brace:candidate.rfind('}') + 1]
    try:
        parsed = json.loads(candidate)
    except json.JSONDecodeError as error:
        raise RuntimeError(f'provider did not return a parsable verdict: {error}')
    if not isinstance(parsed, dict) or set(parsed) != set(keys):
        got = sorted(parsed) if isinstance(parsed, dict) else type(parsed).__name__
        raise RuntimeError(f'verdict keys must be exactly {sorted(keys)}, got {got}')
    if parsed['verdict'] not in {'pass', 'fail'}:
        raise RuntimeError('verdict must be pass or fail')
    if not isinstance(parsed['findings'], list):
        raise RuntimeError('findings must be a list')
    if parsed['verdict'] == 'pass' and parsed['findings']:
        raise RuntimeError('a pass must carry zero findings')
    for key in ('out_of_scope_changes', 'contract_warning_resolutions'):
        if key in keys and not isinstance(parsed[key], list):
            raise RuntimeError(f'{key} must be a list')
    return parsed


def capture_diff(worktree: Path) -> dict:
    """Capture the whole change, including files the agent created.

    `git diff` alone covers tracked files only, so a new module or a new test
    would be referenced by the patch and absent from it — and then destroyed
    with the worktree. Registering untracked paths with --intent-to-add makes
    their contents part of the diff, so the patch is complete and applyable.
    Paths are passed explicitly; this never stages with -A or a bare dot.
    """
    untracked = run(['git', '-C', str(worktree), 'ls-files', '--others', '--exclude-standard'],
                    check=False).stdout.split()
    if untracked:
        run(['git', '-C', str(worktree), 'add', '--intent-to-add', '--', *untracked], check=False)
    tracked = run(['git', '-C', str(worktree), 'diff', '--binary'], check=False).stdout
    names = run(['git', '-C', str(worktree), 'diff', '--name-only'], check=False).stdout.split()
    return {'diff': tracked, 'changed_paths': names, 'untracked_paths': untracked}


def containment_check(worktree: Path, allowed_paths, run_obj) -> dict:
    """After the fact: prove the executor stayed inside its own worktree."""
    changed = capture_diff(worktree)
    touched = changed['changed_paths'] + changed['untracked_paths']
    outside = [p for p in touched if not any(p.startswith(a) for a in allowed_paths)]
    head_moved = run(['git', '-C', str(worktree), 'rev-parse', 'HEAD'], check=False).stdout.strip()
    commits = run(['git', '-C', str(worktree), 'log', '--oneline', '-1'], check=False).stdout.strip()
    report = {
        'touched_path_count': len(touched),
        'paths_outside_allowed_scope': outside,
        'worktree_head': head_moved,
        'worktree_head_commit_line': commits,
        'contained': not outside,
    }
    run_obj.save('containment.json', report)
    run_obj.event('containment_checked', contained=report['contained'],
                  outside=len(outside), touched=len(touched))
    return report
