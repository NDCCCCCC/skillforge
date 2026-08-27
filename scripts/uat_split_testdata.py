"""Split data/staging/testdata.duckdb into per-period dataset files + registry rows.

Phase 17 live-UAT fixture step: the KPI engine ATTACHes one .duckdb per
(dataset, period) composite (D14-D2 / D15-C2); the staging file holds all
periods in one table named `testdata`, but the controlled KPI SQL (D16-A2
staging contract) queries table `data` inside each composite file.
"""
import duckdb, hashlib, json, os

SRC = "data/staging/testdata.duckdb"
OUT_DIR = "data/datasets"

src = duckdb.connect(SRC, read_only=True)
periods = [r[0] for r in src.execute(
    "SELECT DISTINCT period FROM testdata ORDER BY period").fetchall()]
print("periods:", periods)

rows = []
for i, p in enumerate(periods):
    out_path = f"{OUT_DIR}/testdata__{p}.duckdb"
    if os.path.exists(out_path):
        os.remove(out_path)
    con = duckdb.connect(out_path)
    con.execute(f"ATTACH '{SRC}' AS src (READ_ONLY)")
    con.execute(f"CREATE TABLE data AS SELECT * FROM src.testdata WHERE period = '{p}'")
    n = con.execute("SELECT COUNT(*) FROM data").fetchone()[0]
    con.close()
    sha = hashlib.sha256(open(out_path, "rb").read()).hexdigest()
    # stagger created_at so the latest period carries max(created_at)
    # (executor as_of picks the newest row's period, D16-08-F)
    ts = f"2026-08-27T10:{i:02d}:00Z"
    rows.append({
        "name": f"testdata__{p}", "dataset": "testdata", "period": p,
        "file_path": out_path, "content_sha256": sha, "created_at": ts, "rows": n,
    })
    print(p, n, sha[:12], ts)

json.dump(rows, open(f"{OUT_DIR}/_registry_rows.json", "w"), indent=1)
print("DONE", len(rows))
