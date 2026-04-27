import sys
from subprocess import run

DEFAULT = "e.yv"


def main():
    file = sys.argv[1] if len(sys.argv) > 1 else DEFAULT
    if file == "test":
        run(f"py ./scripts/test.py")
    elif file == "bench":
        run(f"py ./scripts/bench.py")
    else:
        run(f"go run ./cmd/yeva run {file}")


if __name__ == "__main__":
    main()
