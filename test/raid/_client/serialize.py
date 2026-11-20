"""Deterministic synthetic SavedVariables with Lua 5.1-compatible UTF-8 strings.
Table layout is a fixture convention, not native-client formatting evidence.
"""

# Fixed-width decimals cannot swallow a following digit. Keep Unicode raw:
# JSON's \uXXXX escapes are not Unicode escapes in Lua 5.1.
_STRING_ESCAPES = {n: f"\\{n:03d}" for n in range(32)}
_STRING_ESCAPES.update({127: r"\127", ord("\\"): r"\\", ord('"'): r'\"',
                        8: r"\b", 9: r"\t", 10: r"\n", 12: r"\f", 13: r"\r"})

def literal(value):
    if value is None:
        return "nil"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    if isinstance(value, str):
        return '"' + value.translate(_STRING_ESCAPES) + '"'
    rows = sorted(value.items(), key=lambda pair: (type(pair[0]).__name__, str(pair[0])))
    return "{" + ",".join("[" + literal(k) + "]=" + literal(v) for k,v in rows) + "}"
