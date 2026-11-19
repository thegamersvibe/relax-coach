"""Mock game client for Relax Raid Lua modules (PLAN.md 3.4). Small on purpose: it models only what RelaxRaid
touches. R1 is the first user; a call R1 needs and this lacks is added by R1, not guessed here.

Lua runs in-process through lupa (Lua 5.1), the library test/harness.py uses, pinned in
.github/requirements-ci.txt. This file does not import or extend test/harness.py.

Modelled, and nothing else:
  - LoggingCombat(on) and its read: the read lags by `log_lag` seconds; a start is ignored for
    `start_ignored_after_zone` seconds after a zone change.
  - The advancedCombatLogging setting through C_CVar.GetCVar/SetCVar, with `refuse_cvar_write` making the
    write fail and leave the value unchanged.
  - The addon restriction state (`restricted`, set by the test): C_ChatInfo.SendAddonMessage while restricted
    is dropped, counted in `dropped`, and returns false.
  - Instance type ("raid", "party", "none") and map id: IsInInstance(), GetInstanceInfo().
  - A clock the test moves: GetTime(), C_Timer.After(); timers fire in time order when advance() passes them.
  - Secret values: secret() returns a Lua value that raises on arithmetic, comparison and concatenation.
API names are the client's own; each is confirmed in the client listing by the module that first uses it.
"""
try:
    import lupa.lua51 as lupa
except ImportError:  # older lupa layouts
    import lupa

SECRET_LUA = """
local function boom() error("secret value used", 2) end
local mt = {__add=boom, __sub=boom, __mul=boom, __div=boom, __mod=boom, __pow=boom, __unm=boom,
            __lt=boom, __le=boom, __eq=boom, __concat=boom, __len=boom}
return function() return setmetatable({}, mt) end
"""


class Client:
    def __init__(self, log_lag=0.0, start_ignored_after_zone=0.0):
        self.lua = lupa.LuaRuntime(unpack_returned_tuples=True)
        self.now = 0.0
        self.log_lag = log_lag
        self.start_ignored_after_zone = start_ignored_after_zone
        self.zone_changed_at = None
        self._log = [(float("-inf"), False)]  # (time the value took effect, value)
        self.ignored_starts = 0
        self.cvars = {"advancedCombatLogging": "0"}
        self.refuse_cvar_write = False
        self.restricted = False
        self.sent = []
        self.dropped = 0
        self.instance_type = "none"
        self.map_id = 0
        self._timers = []
        self._seq = 0
        self._install()

    # --- test controls -------------------------------------------------------------------------------
    def advance(self, seconds):
        """Move the clock; timers fire in time order (ties in scheduling order) as the clock passes them."""
        end = self.now + seconds
        while True:
            due = [t for t in self._timers if t[0] <= end]
            if not due:
                break
            t = min(due)
            self._timers.remove(t)
            self.now = t[0]
            t[2]()
        self.now = end

    def zone_change(self, instance_type="none", map_id=0):
        self.instance_type, self.map_id = instance_type, map_id
        self.zone_changed_at = self.now

    def run(self, code):
        return self.lua.execute(code)

    # --- client API ------------------------------------------------------------------------------------
    def _logging_combat(self, on=None):
        if on is None:
            seen = self.now - self.log_lag
            value = False
            for at, v in self._log:
                if at <= seen:
                    value = v
            return value
        on = bool(on)
        if on and self.zone_changed_at is not None and self.now - self.zone_changed_at < self.start_ignored_after_zone:
            self.ignored_starts += 1
            return self._logging_combat()
        self._log.append((self.now, on))
        return on

    def _get_cvar(self, name):
        return self.cvars.get(name)

    def _set_cvar(self, name, value):
        if self.refuse_cvar_write:
            return False
        self.cvars[name] = str(value)
        return True

    def _send(self, prefix, text, channel, target=None):
        if self.restricted:
            self.dropped += 1
            return False
        self.sent.append((prefix, text, channel, target))
        return True

    def _in_instance(self):
        inside = self.instance_type != "none"
        return inside, self.instance_type

    def _instance_info(self):
        # name, instanceType, difficultyID, difficultyName, maxPlayers, dynamicDifficulty, isDynamic, instanceID
        return "Instance", self.instance_type, 0, "", 0, 0, False, self.map_id

    def _after(self, delay, fn):
        self._seq += 1
        self._timers.append((self.now + max(0.0, float(delay)), self._seq, fn))

    def _install(self):
        g = self.lua.globals()
        self.secret = self.lua.execute(SECRET_LUA)
        g.LoggingCombat = self._logging_combat
        g.GetTime = lambda: self.now
        g.IsInInstance = self._in_instance
        g.GetInstanceInfo = self._instance_info
        g.C_CVar = self.lua.table_from({"GetCVar": self._get_cvar, "SetCVar": self._set_cvar})
        g.C_ChatInfo = self.lua.table_from({"SendAddonMessage": self._send})
        g.C_Timer = self.lua.table_from({"After": self._after})
        g.secret = self.secret
