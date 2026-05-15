"""
render_python.py renders every chat template in testdata/templates against every
captured request body in testdata/requests using the **same** Jinja2 setup that
HuggingFace's `transformers.PreTrainedTokenizerBase.apply_chat_template` uses,
and writes the results to stdout as JSON.

Output shape:

    {
      "<template_name>/<request_name>": "<rendered text, or RENDER ERROR: ...>",
      ...
    }

The Go test in requests_test.go invokes this script once per `go test` run via
`uv run --with "jinja2>=3.1" python scripts/render_python.py` and uses the
returned map as the canonical reference for byte-for-byte comparison. There are
no checked-in pre-rendered "golden" files — adding a new template or request is
just a matter of dropping the file in testdata/, and the next test run picks it
up automatically.
"""

from __future__ import annotations

import json
import sys
from datetime import datetime
from pathlib import Path

import jinja2
import jinja2.ext  # noqa: F401  (registers loopcontrols extension)
from jinja2.exceptions import TemplateError
from jinja2.sandbox import ImmutableSandboxedEnvironment


class GenerationExtension(jinja2.ext.Extension):
    """No-op {% generation %}...{% endgeneration %} block.

    HuggingFace tokenizers register an `AssistantTracker` extension that adds
    these tags to mark which spans count as model-generated for loss masking.
    For pure prompt rendering the tags are pass-through — the inner content
    is emitted verbatim. We register an equivalent no-op here so the rnj-1
    chat template (and any future template using the convention) renders
    instead of raising "Encountered unknown tag 'generation'".
    """

    tags = {"generation"}

    def parse(self, parser):
        lineno = next(parser.stream).lineno
        body = parser.parse_statements(("name:endgeneration",), drop_needle=True)
        return jinja2.nodes.Scope(body).set_lineno(lineno)


REPO_ROOT = Path(__file__).resolve().parent.parent
TEMPLATES_DIR = REPO_ROOT / "testdata" / "templates"
REQUESTS_DIR = REPO_ROOT / "testdata" / "requests"


# ---- env setup: copied from transformers/tokenization_utils_base.py ----

def _raise_exception(message):
    raise TemplateError(message)


def _tojson(x, ensure_ascii=False, indent=None, separators=None, sort_keys=False):
    return json.dumps(
        x,
        ensure_ascii=ensure_ascii,
        indent=indent,
        separators=separators,
        sort_keys=sort_keys,
        default=str,
    )


def _strftime_now(format):
    # Match the Go engine's strftime_now builtin (which calls time.Now()).
    return datetime.now().strftime(format)


def make_env() -> ImmutableSandboxedEnvironment:
    env = ImmutableSandboxedEnvironment(
        trim_blocks=True,
        lstrip_blocks=True,
        extensions=[jinja2.ext.loopcontrols, GenerationExtension],
    )
    env.filters["tojson"] = _tojson
    env.globals["raise_exception"] = _raise_exception
    env.globals["strftime_now"] = _strftime_now
    return env


# ---- render context: must match buildRenderData() in requests_test.go ----

def build_context(req: dict) -> dict:
    return {
        "messages": req.get("messages"),
        "tools": req.get("tools"),
        "add_generation_prompt": True,
        "bos_token": "<s>",
        "eos_token": "</s>",
        "enable_thinking": False,
        "thinking": False,
        "reasoning_effort": "medium",
        "model_identity": "You are a helpful assistant.",
        "builtin_tools": [],
        "date": "2024-01-01",
        "date_string": "2024-01-01",
        "tools_in_user_message": False,
    }


def render_one(env, template_src: str, ctx: dict) -> str:
    try:
        compiled = env.from_string(template_src)
        return compiled.render(**ctx)
    except Exception as e:  # TemplateError, UndefinedError, anything raised by the template
        return f"RENDER ERROR: {e}\n"


def main() -> int:
    env = make_env()

    templates = sorted(TEMPLATES_DIR.glob("*.jinja"))
    requests = sorted(REQUESTS_DIR.glob("*.json"))

    if not templates:
        print(f"no templates found under {TEMPLATES_DIR}", file=sys.stderr)
        return 1
    if not requests:
        print(f"no requests found under {REQUESTS_DIR}", file=sys.stderr)
        return 1

    # Pre-parse requests once, since we render every template against every one.
    parsed_requests = {p.stem: json.loads(p.read_text()) for p in requests}

    out: dict[str, str] = {}
    for tpl_path in templates:
        tpl_name = tpl_path.stem
        tpl_src = tpl_path.read_text()
        for req_name, req in parsed_requests.items():
            ctx = build_context(req)
            out[f"{tpl_name}/{req_name}"] = render_one(env, tpl_src, ctx)

    json.dump(out, sys.stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
