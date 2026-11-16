"""First-party literal parser adapted from the read-only Keep or Toss audit.
No donor code is imported or executed; sparse tables and duplicate keys matter.
"""
class ParseError(ValueError):
    pass


def tokenize(text):
    toks, i, n, line = [], 0, len(text), 1
    while i < n:
        c = text[i]
        if c == '\n':
            line += 1; i += 1
        elif c in ' \t\r':
            i += 1
        elif text.startswith('--', i):
            if text.startswith('--[[', i) or text.startswith('--[=', i):
                raise ParseError(f'line {line}: block comment not supported')
            j = text.find('\n', i)
            i = n if j < 0 else j
        elif c == '"':
            j, buf = i + 1, []
            while True:
                if j >= n or text[j] == '\n':
                    raise ParseError(f'line {line}: unterminated string')
                d = text[j]
                if d == '\\':
                    e = text[j + 1]
                    if e in '"\\\'':
                        buf.append(e)
                    elif e == 'n':
                        buf.append('\n')
                    else:
                        raise ParseError(f'line {line}: escape \\{e} not supported')
                    j += 2
                elif d == '"':
                    break
                else:
                    buf.append(d); j += 1
            toks.append(('str', ''.join(buf), line)); i = j + 1
        elif c.isdigit() or (c == '-' and i + 1 < n and text[i + 1].isdigit()):
            j = i + 1
            while j < n and (text[j].isdigit() or text[j] == '.'):
                j += 1
            raw = text[i:j]
            if j < n and (text[j].isalpha() or text[j] == '_'):
                raise ParseError(f'line {line}: bad number {text[i:j+4]!r}')
            toks.append(('num', float(raw) if '.' in raw else int(raw), line)); i = j
        elif c.isalpha() or c == '_':
            j = i + 1
            while j < n and (text[j].isalnum() or text[j] == '_'):
                j += 1
            toks.append(('name', text[i:j], line)); i = j
        elif text.startswith('...', i):
            toks.append(('op', '...', line)); i += 3
        elif c in '{}[]=,.;':
            toks.append(('op', c, line)); i += 1
        else:
            raise ParseError(f'line {line}: unexpected character {c!r}')
    toks.append(('eof', None, line))
    return toks


class Parser:
    def __init__(self, toks, fname):
        self.t, self.p, self.fname = toks, 0, fname
        self.locals, self.tables, self.has_L = {}, {}, False

    def peek(self, k=0):
        return self.t[self.p + k]

    def fail(self, msg):
        kind, val, line = self.peek()
        raise ParseError(f'{self.fname}:{line}: {msg} (at {kind} {val!r})')

    def eat(self, kind, val=None):
        k, v, _ = self.peek()
        if k != kind or (val is not None and v != val):
            self.fail(f'expected {kind} {val!r}')
        self.p += 1
        return v

    def at(self, kind, val=None, k=0):
        a, b, _ = self.peek(k)
        return a == kind and (val is None or b == val)

    def value(self):
        kind, val, _ = self.peek()
        if kind in ('num', 'str'):
            self.p += 1
            return val
        if kind == 'op' and val == '{':
            return self.table()
        if kind == 'name':
            if val in ('true', 'false', 'nil'):
                self.p += 1
                return {'true': True, 'false': False, 'nil': None}[val]
            if val == 'L' and self.has_L and self.at('op', '[', 1):
                self.p += 2
                s = self.eat('str')
                self.eat('op', ']')
                return s
            if val in self.locals:
                self.p += 1
                return self.locals[val]
            self.fail('unknown identifier in value position')
        self.fail('value expected')

    def table(self):
        self.eat('op', '{')
        out, index = {}, 1
        while not self.at('op', '}'):
            if self.at('op', '['):
                self.p += 1
                key = self.value()
                self.eat('op', ']'); self.eat('op', '=')
                if not isinstance(key, (int, str)) or isinstance(key, bool):
                    self.fail('key must be an integer or a string')
            elif self.at('name') and self.at('op', '=', 1):
                key = self.eat('name'); self.eat('op', '=')
            else:
                key = index; index += 1
            if key in out:
                self.fail(f'duplicate key {key!r}')
            out[key] = self.value()
            if self.at('op', ',') or self.at('op', ';'):
                self.p += 1
            elif not self.at('op', '}'):
                self.fail('expected , or }')
        self.eat('op', '}')
        return out

    def run(self):
        while not self.at('eof'):
            if self.at('name', 'local'):
                self.p += 1
                name = self.eat('name')
                if name == '_':
                    self.eat('op', ','); self.eat('name', 'ns'); self.eat('op', '='); self.eat('op', '...')
                    continue
                self.eat('op', '=')
                if name == 'L':
                    self.eat('name', 'ns'); self.eat('op', '.'); self.eat('name', 'L')
                    self.has_L = True
                    continue
                if name in self.locals:
                    self.fail(f'local {name} defined twice')
                v = self.value()
                if isinstance(v, dict):
                    self.fail('local table not supported')
                self.locals[name] = v
            elif self.at('name', 'ns'):
                self.p += 1
                self.eat('op', '.')
                name = self.eat('name')
                self.eat('op', '=')
                if name in self.tables:
                    self.fail(f'ns.{name} assigned twice')
                self.tables[name] = self.value()
            else:
                self.fail('statement not in the literal grammar')
        return self.tables
