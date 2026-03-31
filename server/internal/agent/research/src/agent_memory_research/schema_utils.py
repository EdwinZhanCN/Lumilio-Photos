from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from jsonschema import Draft202012Validator

AGENT_ROOT = Path(__file__).resolve().parents[3]
SCHEMA_DIR = AGENT_ROOT / "schemas"


def load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as file:
        return json.load(file)


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as file:
        json.dump(payload, file, ensure_ascii=False, indent=2)
        file.write("\n")


def load_schema(name: str) -> dict[str, Any]:
    path = SCHEMA_DIR / name
    if not path.exists():
        raise FileNotFoundError(f"schema not found: {path}")
    schema = load_json(path)
    if not isinstance(schema, dict):
        raise TypeError(f"schema must be an object: {path}")
    return schema


def validate_with_schema(payload: Any, schema_name: str) -> None:
    schema = load_schema(schema_name)
    validator = Draft202012Validator(schema)
    errors = sorted(
        validator.iter_errors(payload), key=lambda err: list(err.absolute_path)
    )
    if not errors:
        return

    formatted = []
    for error in errors:
        location = ".".join(str(part) for part in error.absolute_path) or "<root>"
        formatted.append(f"{location}: {error.message}")
    raise ValueError("schema validation failed:\n" + "\n".join(formatted))
