# Chat template sources

## DeepSeek-V4.1-Flash

`DeepSeek-V4.1-Flash.jinja` is an unmodified copy of the actual model-repository
template (not llama.cpp's minimal stand-in):

- Repository: https://huggingface.co/deepseek-ai/DeepSeek-V4.1-Flash
- Revision: `314113f8a7dadb388215396ba5fbee2c610a04d6`
- File: https://huggingface.co/deepseek-ai/DeepSeek-V4.1-Flash/blob/314113f8a7dadb388215396ba5fbee2c610a04d6/chat_template.jinja

The upstream template describes itself as a port of that repository's
`encoding/encoding.py`. It differs substantially from the existing V4 fixture:
spaced DSML `calls`/`invoke`/`parameter` tags, a system token, numeric reasoning
budgets, and tool-result merging/reordering, among other changes.

The synthetic `deepseek-v4.1-*.json` requests exercise Go/Python Jinja2 parity,
not model execution or full model/encoder compatibility. `chat_template_kwargs`
is keyed by template name and overrides that template's neutral render defaults
in both engines (other templates still run against the requests with their
existing defaults). Efforts
cover low/medium/high/max and integer endpoints 1/100; chat mode and both thinking
control spellings are covered. The tool-history request includes JSON-encoded
arguments (normalized to a mapping as in Kronk), object arguments with string and
non-string parameters, retained reasoning, and out-of-order tool results.
