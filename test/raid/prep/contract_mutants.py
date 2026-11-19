"""Run the app-1.8.0 gate against isolated malformed copies; all six must fail."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
for key in ("extra-save", "extra-sv-key", "texture", "dependencies", "requireddeps", "dep-alias"):
    with tempfile.TemporaryDirectory() as tmp:
        folder = Path(tmp) / "RelaxRaid"
        shutil.copytree(ROOT / "addon/RelaxRaid", folder)
        toc = folder / "RelaxRaid.toc"
        text = toc.read_text()
        if key == "extra-save":
            text = text.replace("## SavedVariables: RelaxRaidDB", "## SavedVariables: RelaxRaidDB, RelaxRaidPrepDB")
        elif key == "extra-sv-key":
            text += "\n## SavedVariablesFuture: OtherDB\n"
        elif key == "texture":
            (folder / "unexpected.tga").write_bytes(b"synthetic invalid texture")
        else:
            meta = {"dependencies": "Dependencies", "requireddeps": "RequiredDeps", "dep-alias": "DepFuture"}[key]
            text += "\n## " + meta + ": ForeverCoach\n"
        toc.write_text(text)
        env = dict(os.environ, RAID_CONTRACT_FOLDER=str(folder))
        p = subprocess.run([sys.executable, "-I", "-B", str(ROOT / "test/raid/prep/test_prep.py"),
                            "-v", "-k", "test_app_180_contract"], env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        print(key, "EXPECTED RED", p.returncode)
        print(p.stdout)
        assert p.returncode != 0 and "FAIL: test_app_180_contract" in p.stdout
print("RESULT: 6/6 app-1.8.0 contract mutations rejected")
