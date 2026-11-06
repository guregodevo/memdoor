import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


class ReportUsageTest(unittest.TestCase):
    def test_legacy_and_step_sessions_count_only_their_run(self):
        for step_sessions in (False, True):
            with self.subTest(step_sessions=step_sessions), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                shutil.copyfile(Path(__file__).with_name("report.py"), root / "report.py")
                (root / "results").mkdir()
                (root / "logs" / "team-llm-lcb").mkdir(parents=True)
                (root / "results" / "team-llm-lcb.tsv").write_text(
                    "one\tdone\t1/1\t1/1\t2\ntwo\tdone\t1/1\t1/1\t3\n"
                )
                for problem, stamp in (("one", "20261106-090000"), ("two", "20261106-090000-2")):
                    (root / "logs" / "team-llm-lcb" / f"{problem}.log").write_text(
                        f"started as team-llm-lcb-{stamp}\n"
                    )
                sessions = [
                    "workflow:team-llm-lcb-20261106-090000",
                    "workflow:team-llm-lcb-20261106-090000-2",
                ]
                if step_sessions:
                    sessions = [
                        "workflow:team-llm-lcb:same-day@20261106-090000:plan",
                        "workflow:team-llm-lcb:same-day@20261106-090000:judge",
                        "workflow:team-llm-lcb:same-day@20261106-090000-2:plan",
                    ]
                unrelated = [
                    "workflow:team-llm-lcb:same-day@20261106-090000-20:plan",
                    "workflow:other:same-day@20261106-090000:plan",
                    "workflow:team-llm-lcb-20261106-090001",
                    "workflow:team-llm-lcb:same-day:plan",
                ]
                meter = root / "meter.jsonl"
                meter.write_text("\n".join(json.dumps({
                    "session": session, "input_tokens": 12, "output_tokens": 7,
                    "cost_usd": 0.125,
                }) for session in sessions + unrelated) + "\n")
                output = subprocess.check_output(
                    [sys.executable, str(root / "report.py"), "team-llm-lcb", str(meter), "--md"],
                    text=True,
                )
                calls = 2 if step_sessions else 1
                self.assertIn(f"| one | - | {calls} | {12*calls} / {7*calls} | ${0.125*calls:.3f} |", output)
                self.assertIn("| two | - | 1 | 12 / 7 | $0.125 |", output)
                self.assertIn(f"· {len(sessions)} calls ·", output)


if __name__ == "__main__":
    unittest.main()
