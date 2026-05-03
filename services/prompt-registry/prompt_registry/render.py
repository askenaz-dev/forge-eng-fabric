from __future__ import annotations

import re
from typing import Any

import jsonschema

from prompt_registry.models import PromptTemplate

VARIABLE_RE = re.compile(r"{{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*}}")


def render_template(template: PromptTemplate, variables: dict[str, Any]) -> str:
    jsonschema.validate(variables, template.variables_schema)
    rendered = template.template
    for name in VARIABLE_RE.findall(template.template):
        rendered = rendered.replace(f"{{{{{name}}}}}", str(variables.get(name, "")))
        rendered = rendered.replace(f"{{{{ {name} }}}}", str(variables.get(name, "")))
    max_tokens = int(template.guardrails.get("max_tokens", 0) or 0)
    if max_tokens and len(rendered.split()) > max_tokens:
        raise ValueError("rendered prompt exceeds max_tokens guardrail")
    return rendered
