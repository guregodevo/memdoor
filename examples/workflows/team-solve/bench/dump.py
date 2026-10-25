import json, os, sys
sys.path.insert(0, os.getcwd())
from lcb_runner.benchmarks.code_generation import load_code_generation_dataset
ids = json.load(open(sys.argv[1]))
want = set(ids)
ds = load_code_generation_dataset(release_version=os.environ.get("LCB_RELEASE", "release_v6"))
out = []
for p in ds:
    if p.question_id in want:
        out.append({
            "id": p.question_id, "title": p.question_title, "platform": str(p.platform), "difficulty": str(p.difficulty),
            "date": str(p.contest_date), "content": p.question_content, "starter_code": p.starter_code,
            "public": [t.__dict__ if hasattr(t, "__dict__") else t for t in p.public_test_cases],
            "private": [t.__dict__ if hasattr(t, "__dict__") else t for t in p.private_test_cases],
            "metadata": p.metadata,
        })
json.dump(out, open(sys.argv[2], "w"), default=str)
print("dumped", len(out), "of", len(want))
