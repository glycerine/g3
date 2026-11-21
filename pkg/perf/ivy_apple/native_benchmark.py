"""Link a standalone benchmark to the configured in-tree native solver."""

from pathlib import Path
import re
import shlex
import subprocess


ROOT = Path(__file__).resolve().parents[3]


def make_variable(text, key):
    match = re.search(rf"^{key}\s*=\s*(.*)$", text, re.MULTILINE)
    if not match:
        raise ValueError(f"missing {key} in the configured Makefile build")
    return shlex.split(match[1])


def build_native(build, output, source):
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    includes = re.search(r"\$\((includes_\d+)\).*\.\./src/test/smt_scope_restoration\.cpp", makefile)[1]
    cc = make_variable(config, "CXX")
    obj = output.with_suffix(".o")
    subprocess.run([*cc, *make_variable(config, "CXXFLAGS"), *make_variable(makefile, includes),
                    "-o", obj, source], cwd=build, check=True)
    subprocess.run([*cc, "-o", output, obj, build / "libz3.a", *make_variable(config, "LINK_EXTRA_FLAGS")],
                   cwd=build, check=True)
