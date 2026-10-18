"""Task-data helper: prove a named Go test actually ran and passed.

`go test` exits 0 when a -run pattern matches nothing, and a test that calls
t.Skip also counts as ok. Either would let a task "pass" without its test ever
executing. This reads `go test -json` and requires at least one matching test
to report pass, none to fail, and none to skip.

    python verify_go_test.py <package> <test-name-prefix>
"""
import json
import subprocess
import sys


def main() -> int:
    package, prefix = sys.argv[1], sys.argv[2]
    result = subprocess.run(
        ['go', 'test', '-count=1', '-json', '-run', f'^{prefix}', package],
        capture_output=True, text=True, encoding='utf-8', errors='replace',
    )
    passed, failed, skipped = set(), set(), set()
    for line in result.stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        name = event.get('Test')
        if not name or not name.startswith(prefix):
            continue
        action = event.get('Action')
        if action == 'pass':
            passed.add(name)
        elif action == 'fail':
            failed.add(name)
        elif action == 'skip':
            skipped.add(name)
    print(f'passed={sorted(passed)} failed={sorted(failed)} skipped={sorted(skipped)}')
    if result.returncode != 0 and not failed:
        # Compilation errors never produce per-test events.
        print(result.stdout[-3000:])
        print(result.stderr[-3000:])
        return 1
    if failed or skipped or not passed:
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
