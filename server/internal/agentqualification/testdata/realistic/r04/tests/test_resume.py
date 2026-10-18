import json
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]


class ResumeTest(unittest.TestCase):
    def test_exactly_once_state(self):
        counts = json.loads((ROOT / "state" / "step_counts.json").read_text())
        self.assertEqual({"collect": 1, "normalize": 1, "verify": 1, "publish_local_output": 1}, counts)
        output = json.loads((ROOT / "FINAL-OUTPUT.json").read_text())
        self.assertEqual("resume-fixture-17", output["run_family_id"])
        self.assertTrue(output["verified"])


if __name__ == "__main__":
    unittest.main()
