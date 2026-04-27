from lib import *

import os
import time

from datetime import datetime

BENCH_DIR = "./tests/benchmarks"
RESULT_DIR = "./bench"
RESULT_FILE = "benchmark"
FORMAT = "{}\ntime:\n{}\n\nstdout:\n{}"
EXTS = ["yv", "yeva"]


@runner("__bench_build.exe")
def main(run: Run):
    os.makedirs(RESULT_DIR, exist_ok=True)
    file_name = (
        f"{RESULT_DIR}/{RESULT_FILE}"
        + "_"
        + f"{datetime.now().strftime('%d-%m-%Y_%H-%M')}"
    )
    with open(file_name + ".txt", "w", encoding="utf-8") as file:
        for script in read_files(BENCH_DIR, EXTS):
            print(script, "-> ...", end="", flush=True)
            start = time.perf_counter()
            out, err = run("run", str(script))
            perf = time.perf_counter() - start

            if err:
                print(f"\b\b\berror at {script}: {short(err, 32)}")
                continue

            result = FORMAT.format(cover(str(script), 80), perf, out)
            file.write(result)
            print("\b\b\bdone!")

    print("all done!")


if __name__ == "__main__":
    main()
