import copy
import json
from pathlib import Path
import sys
import unittest

SCRIPT_DIR = Path(__file__).parent
sys.path.insert(0, str(SCRIPT_DIR))
from render_report import validate  # noqa: E402

FIXTURE = SCRIPT_DIR / "fixtures" / "minimal.json"


class ReportModelTest(unittest.TestCase):
    def model(self):
        return json.loads(FIXTURE.read_text(encoding="utf-8"))

    def test_minimal_fixture_is_valid(self):
        validate(self.model())

    def test_requires_all_fields_and_rejects_unknown_fields(self):
        model = self.model()
        del model["state"]
        with self.assertRaisesRegex(ValueError, "state must be a list"):
            validate(model)
        model = self.model()
        model["surprise"] = True
        with self.assertRaisesRegex(ValueError, "unknown top-level keys"):
            validate(model)

    def test_requires_string_bullets_state_knowledge_and_items(self):
        mutations = [
            ("turns", lambda m: m["turns"][0]["bullets"].append(7)),
            ("timeline", lambda m: m["timeline"][0]["advice"].append(None)),
            ("knowledge", lambda m: m["knowledge"][0].update(text=3)),
            ("state", lambda m: m["state"].append(False)),
            ("items", lambda m: m["file_groups"][0]["items"][0].update(purpose=None)),
        ]
        for name, mutate in mutations:
            with self.subTest(name=name):
                model = copy.deepcopy(self.model())
                mutate(model)
                with self.assertRaises(ValueError):
                    validate(model)

    def test_churn_requires_nonempty_prevention_advice(self):
        for advice in ([], [""]):
            model = self.model()
            model["timeline"][1]["advice"] = advice
            with self.assertRaisesRegex(ValueError, "requires prevention advice"):
                validate(model)

    def test_turn_numbers_are_unique(self):
        model = self.model()
        model["turns"].append(copy.deepcopy(model["turns"][0]))
        with self.assertRaisesRegex(ValueError, "duplicate number"):
            validate(model)


if __name__ == "__main__":
    unittest.main()
