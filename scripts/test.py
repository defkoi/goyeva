from lib import *

import pathlib
import re

TEST_DIR = "./tests/language"
EXTS = ["yv", "yeva"]


def read_skip(file: pathlib.Path) -> bool:
    with open(file, "r", encoding="utf-8") as f:
        for line in f.readlines():
            if re.search(r"#\s*skip!", line):
                return True
    return False


def read_out(file: pathlib.Path) -> list[str]:
    out: list[str] = []
    with open(file, "r", encoding="utf-8") as f:
        for line in f.readlines():
            if match := re.search(r"#\s*out:(.+)", line):
                out.append(match.group(1).strip())
    return out


def read_err(file: pathlib.Path) -> str:
    with open(file, "r", encoding="utf-8") as f:
        for line in f.readlines():
            if match := re.search(r"#\s*err:(.+)", line):
                return match.group(1).strip()
    return ""


def check_out(out: str, expect: list[str]) -> str | None:
    outs = out.splitlines()
    if outs != expect:
        if len(outs) != len(expect):
            return f"expected {len(expect)} lines, got {len(outs)}"
        for i, line in enumerate(outs):
            if line != expect[i]:
                return f"expected '{expect[i]}', got '{line}'"
    return None


def check_err(err: str, expect: str) -> str | None:
    out = err.splitlines()[0]
    if expect != "..." and expect not in out:
        return f"expected '{expect}', got '{out}'"
    return None


@runner("__test_build.exe")
def main(run: Run):
    print(cover("tests"))
    errors = 0
    for file in read_files(TEST_DIR, EXTS):
        if read_skip(file):
            print(file, "-> skip")
            continue

        expect_out, expect_err = read_out(file), read_err(file)
        out, err = run("run", str(file), timeout=2)

        result: str | None
        if err:
            if not expect_err:
                result = f"unexpected error '{short(err, 120)}'"
            else:
                result = check_err(err, expect_err)
        elif expect_err:
            result = f"expected error '{short(expect_err, 120)}'"
        else:
            result = check_out(out, expect_out)

        if not result:
            print(file, "-> ok")
        else:
            errors += 1
            print(file, "-> error:", short(result, 120))

    print(cover("result"))
    print("ok" if not errors else f"error [{errors}]")


if __name__ == "__main__":
    main()
