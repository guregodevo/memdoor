"""Offline regression tests for the request filter embedded in metrics.sh."""
import pathlib
import subprocess
import unittest


class MetricsRequestsTest(unittest.TestCase):
    def filter(self, rows, first="2026-09-27", last="2026-10-10"):
        script = pathlib.Path(__file__).with_name("metrics.sh").read_text()
        function = script.split("requests() {", 1)[1].split('\necho "==>', 1)[0]
        shell = (
            'catlog() { cat; }\n'
            f"first={first}\nlast={last}\n"
            "requests() {" + function + "\nrequests\n"
        )
        result = subprocess.run(
            ["sh", "-c", shell], input="\n".join(rows) + "\n",
            text=True, capture_output=True, check=True,
        )
        return result.stdout.splitlines()

    def row(self, day, path="/", status=200, agent="Mozilla/5.0", method="GET"):
        return (
            f'192.0.2.1 - - [{day}:12:00:00 +0000] '
            f'"{method} {path} HTTP/1.1" {status} 123 "-" "{agent}"'
        )

    def test_calendar_boundaries(self):
        rows = [self.row(day) for day in [
            "26/Sep/2026", "27/Sep/2026", "30/Sep/2026",
            "01/Oct/2026", "10/Oct/2026", "11/Oct/2026",
        ]]
        self.assertEqual([line.split()[0] for line in self.filter(rows)], [
            "2026-09-27", "2026-09-30", "2026-10-01", "2026-10-10",
        ])
        rows = [self.row(day) for day in ["31/Dec/2025", "01/Jan/2026"]]
        self.assertEqual(len(self.filter(rows, "2025-12-25", "2026-01-07")), 2)

    def test_status_ci_and_exact_paths(self):
        rows = [self.row("10/Oct/2026", path, status) for path, status in [
            ("/install.sh?ci=1", 200), ("/install.sh?x=2&ci=1&y=3", 200),
            ("/install.sh", 404), ("/install.sh", 500),
            ("/install.sh?ci=10", 200), ("/dl/memdoor-darwin-arm64", 206),
            ("/dl/VERSION", 304),
        ]]
        rows.append(self.row("10/Oct/2026", method="POST"))
        self.assertEqual([line.split()[2] for line in self.filter(rows)], [
            "/install.sh", "/dl/memdoor-darwin-arm64", "/dl/VERSION",
        ])

    def test_bot_heuristic_is_case_insensitive(self):
        rows = [self.row("10/Oct/2026", agent=agent) for agent in [
            "GoogleBot", "CURL/8", "Mozilla/5.0",
        ]]
        self.assertEqual([line.split()[3] for line in self.filter(rows)], ["1", "1", "0"])


if __name__ == "__main__":
    unittest.main()
