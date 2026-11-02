"""
render_snippets.py renders a list of template sources with the Jinja2 setup in
render_python.py and writes the results to stdout as JSON. The sources are read
from stdin as a JSON array of strings.

Output shape:

    {
      "results": [{"out": "<rendered text>"} or {"error": "<exception>"}, ...],
      "whitespace": [<every code point Python treats as whitespace>, ...]
    }

The Go test in whitespace_parity_test.go invokes this script through uv and
compares this engine's output to Jinja2's byte for byte. An entry in "results"
holds an error when Jinja2 rejects the template, and the engine must reject it too.
"""

from __future__ import annotations

import json
import re
import sys

from render_python import make_env

# Must stay in sync with whitespaceData() in whitespace_test.go.
CONTEXT = {
    "x": "Max",
    "y": "",
    "n": 3,
    "yes": True,
    "no": False,
    "items": ["a", "b"],
    "ml": "l1\nl2",
}


def render(env, source: str) -> dict:
    try:
        return {"out": env.from_string(source).render(**CONTEXT)}
    except Exception as e:
        return {"error": f"{type(e).__name__}: {e}"}


def whitespace() -> list[int]:
    """Code points in str.isspace and in the lexer's \\s, which must agree."""
    points = [c for c in range(0x110000) if chr(c).isspace()]
    regex = [c for c in range(0x110000) if re.fullmatch(r"\s", chr(c))]
    stripped = [c for c in range(0x110000) if not chr(c).rstrip()]
    if points != regex or points != stripped:
        raise SystemExit("str.isspace, str.rstrip and \\s disagree")
    return points


def main() -> int:
    env = make_env()
    sources = json.load(sys.stdin)
    json.dump(
        {
            "results": [render(env, source) for source in sources],
            "whitespace": whitespace(),
        },
        sys.stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
