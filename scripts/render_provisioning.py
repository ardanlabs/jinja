"""Render provisioning cases from stdin, independently of the chat request corpus."""

import json
import sys

from render_python import make_env


def main():
    env = make_env()
    results = []
    for case in json.load(sys.stdin):
        try:
            output = env.from_string(case["source"]).render(**case["data"])
            results.append({"output": output})
        except Exception as exc:
            results.append({"error": str(exc)})
    json.dump(results, sys.stdout)


if __name__ == "__main__":
    main()
