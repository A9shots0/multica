import importlib.util
import json
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]


def load(path):
    spec = importlib.util.spec_from_file_location(path.parent.name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class AuthorityContractTest(unittest.TestCase):
    def test_consumers_preserve_canonical_role(self):
        authority = json.loads((ROOT / "crm" / "authority.json").read_text())
        for consumer in ("consumer_good", "consumer_faulty"):
            module = load(ROOT / consumer / "normalize.py")
            self.assertEqual(authority["role"], module.normalize_role(authority["role"]))

    def test_phone_farm_contract_remains_read_only_and_compatible(self):
        contract = json.loads((ROOT / "phone_farm_contract" / "expected.json").read_text())
        self.assertEqual("phonefarm.truth.v2", contract["contract"])
        self.assertIn("team_lead", contract["accepted_roles"])


if __name__ == "__main__":
    unittest.main()
