import importlib.util
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("access", ROOT / "app" / "access.py")
access = importlib.util.module_from_spec(spec)
spec.loader.exec_module(access)


class ReleaseContractTest(unittest.TestCase):
    def test_critical_employee_pages_are_safe_default(self):
        self.assertEqual(["posting", "schedule"], access.employee_pages())

    def test_manager_retains_review(self):
        self.assertEqual(["posting", "schedule", "review"], access.manager_pages())


if __name__ == "__main__":
    unittest.main()
