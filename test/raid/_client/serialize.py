"""Canonical Lua-literal fixture writer for synthetic SavedVariables (not game serialization)."""
import json

def literal(value):
    if value is None:
        return "nil"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=True)
    rows = sorted(value.items(), key=lambda pair: (type(pair[0]).__name__, str(pair[0])))
    return "{" + ",".join("[" + literal(k) + "]=" + literal(v) for k,v in rows) + "}"
