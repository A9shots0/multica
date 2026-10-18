import json
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent
counts_path = ROOT / "state" / "step_counts.json"
checkpoint = json.loads((ROOT / "state" / "checkpoint.json").read_text())
counts = json.loads(counts_path.read_text())
if checkpoint["terminal_state"] != "interrupted" or checkpoint["next_step"] != "verify":
    raise SystemExit("invalid durable checkpoint")
if counts["collect"] != 1 or counts["normalize"] != 1:
    raise SystemExit("completed work must not be repeated")
counts["verify"] += 1
counts["publish_local_output"] += 1
counts_path.write_text(json.dumps(counts, sort_keys=True) + "\n")
(ROOT / "FINAL-OUTPUT.json").write_text(json.dumps({"run_family_id": checkpoint["run_family_id"], "verified": True}, sort_keys=True) + "\n")
