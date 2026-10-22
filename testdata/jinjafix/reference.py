"""Recompute Python results for the exact inputs used by the Go unit test."""
import json
import pathlib
import platform
import sys

import jinja2


def evaluate(source):
    env = jinja2.Environment(trim_blocks=True, lstrip_blocks=True,
                             keep_trailing_newline=True)
    # fromjson is a repository extension; tojson remains stock Jinja2.
    env.filters['fromjson'] = json.loads
    try:
        template = env.from_string(source)
    except Exception as error:
        return dict(stage='compile_error', error=str(error), exception=type(error).__name__)
    try:
        return dict(stage='render', output=template.render())
    except Exception as error:
        return dict(stage='render_error', error=str(error), exception=type(error).__name__)


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] == '--worker':
        print(json.dumps(evaluate(sys.stdin.read()), ensure_ascii=False))
    else:
        import subprocess
        here = pathlib.Path(__file__).resolve().parent
        cases = json.loads((here / 'cases.json').read_text())
        results = []
        for case in cases:
            child = subprocess.run([sys.executable, __file__, '--worker'],
                                   input=case['source'], capture_output=True,
                                   text=True, timeout=5, check=True)
            results.append(dict(id=case['id'], **json.loads(child.stdout)))
        print(json.dumps(dict(python=platform.python_version(), jinja2=jinja2.__version__,
                              settings=dict(trim_blocks=True, lstrip_blocks=True,
                                            keep_trailing_newline=True),
                              extensions=['fromjson=json.loads'], cases=results),
                         ensure_ascii=False, indent=2))
