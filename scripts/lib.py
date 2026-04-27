import subprocess
import pathlib
import os

from itertools import chain
from typing import Iterable, Callable, Protocol
from functools import wraps

WD = "."
MAIN = "./cmd/yeva"


class TODO(Exception):
    pass


class Run(Protocol):
    def __call__(
        self, *args: str, timeout: int | None = None
    ) -> tuple[str, str]: ...


def runner(name: str):
    def deco(func: Callable[[Run], None]):
        @wraps(func)
        def inner():
            cwd = os.getcwd()
            os.chdir(WD)
            os.system(f"go build -o {name} {MAIN}")
            try:

                def run(*args: str, timeout: int | None = None):
                    return _run_script(name, *args, timeout=timeout)

                func(run)
            finally:
                os.unlink(name)
                os.chdir(cwd)

        return inner

    return deco


def short(string: str, length: int = 32) -> str:
    if len(string) <= length:
        return string
    return string[:length]


def cover(string: str, width: int = 24, char: str = "=") -> str:
    to_cover = width - (len(string) + 2)
    if to_cover < 0:
        return string
    left = to_cover // 2
    right = left + to_cover % 2
    return f"{left * char} {string} {right * char}"


def read_files(folder: str, exts: list[str]) -> Iterable[pathlib.Path]:
    return chain(*[pathlib.Path(folder).rglob(f"*.{ext}") for ext in exts])


def _run_script(
    executable: str, *args: str, timeout: int | None = None
) -> tuple[str, str]:
    result = subprocess.run(
        [executable, *args],
        capture_output=True,
        text=True,
        encoding="utf-8",
        timeout=timeout,
    )
    return result.stdout, result.stderr
