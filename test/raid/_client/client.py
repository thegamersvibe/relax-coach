"""Mock game client for Relax Raid Lua modules (PLAN.md 3.4). Small on purpose: it models only what RelaxRaid
touches. R1 is the first user; a call R1 needs and this lacks is added by R1, not guessed here.

Lua runs in-process through lupa (Lua 5.1), the library test/harness.py uses, pinned in
.github/requirements-ci.txt. This file does not import or extend test/harness.py.

Modelled, and nothing else:
  - LoggingCombat(on) and its read: the read lags by `log_lag` seconds; a start is ignored for
    `start_ignored_after_zone` seconds after a zone change (answering the read's value, or true with
    `ignored_start_says_on`, added by R1: the client can take a start without error and ignore it).
  - The advancedCombatLogging setting through C_CVar.GetCVar/SetCVar, with `refuse_cvar_write` making the
    write fail and leave the value unchanged.
  - The addon restriction state (`restricted`, set by the test): C_ChatInfo.SendAddonMessage while restricted
    is dropped, counted in `dropped`, and returns false.
  - Instance type ("raid", "party", "none") and map id: IsInInstance(), GetInstanceInfo().
  - A clock the test moves: GetTime(), C_Timer.After(); timers fire in time order when advance() passes them.
  - Secret values: secret() returns a Lua value that raises on arithmetic, comparison and concatenation;
    issecretvalue() tells them apart.
Added by R1 (each confirmed in the client listing at Gethe/wow-ui-source@eac30af, see Log.lua's header):
  - Frames and events: CreateFrame("Frame") with RegisterEvent/UnregisterEvent/SetScript("OnEvent"); fire() delivers
    an event to every frame registered for it, in creation order. enter() = zone_change() + PLAYER_ENTERING_WORLD.
  - Guild: IsInGuild(); GetGuildInfo("player") -> name, rank name, rank index. `guild` None is "not in a guild";
    `guild_loaded` False makes GetGuildInfo return nil while IsInGuild() is true (guild data not loaded yet).
  - Chat output: DEFAULT_CHAT_FRAME:AddMessage() lines are kept in `chat`.
  - Slash commands: SlashCmdList; slash("/cmd args") calls the handler a SLASH_<KEY>n global names.
  - The Settings panel: Settings.RegisterVerticalLayoutCategory/RegisterProxySetting/CreateCheckbox/
    RegisterAddOnCategory, kept in `settings`; a test clicks a checkbox with click_setting().
  - Every LoggingCombat call is kept in `log_calls` (its argument; None for a read); every SetCVar in `cvar_writes`.
  - load_addon(): the toc's files in order with (name, namespace), then saved variables, then ADDON_LOADED.
API names are the client's own; each is confirmed in the client listing by the module that first uses it.
"""
import os

try:
    import lupa.lua51 as lupa
except ImportError:  # older lupa layouts
    import lupa

SECRET_LUA = """
local function boom() error("secret value used", 2) end
local mt = {__add=boom, __sub=boom, __mul=boom, __div=boom, __mod=boom, __pow=boom, __unm=boom,
            __lt=boom, __le=boom, __eq=boom, __concat=boom, __len=boom}
local made = setmetatable({}, {__mode = "k"})
return function() local v = setmetatable({}, mt); made[v] = true; return v end,
       function(v) return made[v] == true end
"""


class Client:
    def __init__(self, log_lag=0.0, start_ignored_after_zone=0.0, ignored_start_says_on=False):
        self.lua = lupa.LuaRuntime(unpack_returned_tuples=True)
        self.now = 0.0
        self.log_lag = log_lag
        self.start_ignored_after_zone = start_ignored_after_zone
        self.ignored_start_says_on = ignored_start_says_on
        self.zone_changed_at = None
        self._log = [(float("-inf"), False)]  # (time the value took effect, value)
        self.ignored_starts = 0
        self.cvars = {"advancedCombatLogging": "0"}
        self.refuse_cvar_write = False
        self.cvar_writes = []  # every SetCVar call, refused or not
        self.restricted = False
        self.sent = []
        self.dropped = 0
        self.instance_type = "none"
        self.map_id = 0
        self._timers = []
        self._seq = 0
        self.log_calls = []
        self.frames = []
        self.chat = []
        self.guild = None  # (guild name, rank name, rank index) or None when not in a guild
        self.guild_loaded = True
        self.settings = {}  # variable -> {"name", "get", "set", "category", "checkbox"}
        self.categories = []
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

    def load_addon(self, folder, saved=None, files=None):
        """Load an addon the way the client does: the toc's files in order, each called with (name, namespace),
        then the saved variables, then ADDON_LOADED. `saved` maps a saved-variable name to Lua source for its value;
        `files` replaces the toc's list (to load Core.lua alone, say)."""
        name = os.path.basename(os.path.normpath(folder))
        if files is None:
            with open(os.path.join(folder, name + ".toc"), encoding="utf-8") as f:
                files = [ln.strip() for ln in f if ln.strip() and not ln.startswith("#")]
        ns = self.lua.table()
        loader = self.lua.eval("function(src, file, name, ns) local f = assert(loadstring(src, '@' .. file)); "
                               "return f(name, ns) end")
        for file in files:
            with open(os.path.join(folder, file), encoding="utf-8") as f:
                loader(f.read(), file, name, ns)
        for var, value in (saved or {}).items():
            self.lua.execute("%s = %s" % (var, value))
        self.fire("ADDON_LOADED", name, False)
        return ns

    def fire(self, event, *args):
        for f in list(self.frames):
            if event in f["events"] and f["script"] is not None:
                f["script"](f["obj"], event, *args)

    def enter(self, instance_type="none", map_id=0, initial=False, reload=False):
        self.zone_change(instance_type, map_id)
        self.fire("PLAYER_ENTERING_WORLD", initial, reload)

    def slash(self, line):
        cmd, _, rest = line.partition(" ")
        g = self.lua.globals()
        for key, fn in g.SlashCmdList.items():
            n = 1
            while g["SLASH_%s%d" % (key, n)] is not None:
                if g["SLASH_%s%d" % (key, n)].lower() == cmd.lower():
                    return fn(rest)
                n += 1
        raise KeyError("no slash command " + cmd)

    def click_setting(self, variable, value):
        self.settings[variable]["set"](value)

    def setting_value(self, variable):
        return self.settings[variable]["get"]()

    # --- client API ------------------------------------------------------------------------------------
    def _logging_combat(self, on=None):
        self.log_calls.append(on)
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
            if self.ignored_start_says_on:  # taken without error, answered "on", and still ignored
                return True
            return self._logging_combat()
        self._log.append((self.now, on))
        return on

    def _get_cvar(self, name):
        return self.cvars.get(name)

    def _set_cvar(self, name, value):
        self.cvar_writes.append((name, None if value is None else str(value)))
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

    def _create_frame(self, kind="Frame", name=None, parent=None, template=None):
        rec = {"events": set(), "script": None}
        lua = self.lua

        def register(_self, event):
            rec["events"].add(event)

        def unregister(_self, event):
            rec["events"].discard(event)

        def set_script(_self, what, fn):
            if what != "OnEvent":
                raise ValueError("mock frame models only OnEvent, not " + str(what))
            rec["script"] = fn

        obj = lua.table_from({"RegisterEvent": register, "UnregisterEvent": unregister, "SetScript": set_script})
        rec["obj"] = obj
        self.frames.append(rec)
        return obj

    def _guild_info(self, unit):
        if unit != "player" or self.guild is None or not self.guild_loaded:
            return None
        return tuple(self.guild)

    def _add_message(self, _frame, text, *rest):
        self.chat.append(str(text))

    def _register_category(self, name):
        cat = self.lua.table_from({"name": name, "GetID": lambda _s: name})
        self.categories.append({"name": name, "obj": cat, "registered": False})
        return cat, self.lua.table_from({})

    def _register_addon_category(self, cat):
        for c in self.categories:
            if c["name"] == cat["name"]:
                c["registered"] = True

    def _proxy_setting(self, cat, variable, var_type, name, default, get, set_):
        if var_type != "boolean":
            raise ValueError("mock models boolean settings only")
        self.settings[variable] = {"name": name, "get": get, "set": set_, "category": cat, "checkbox": None,
                                   "default": default}
        return self.lua.table_from({"variable": variable})

    def _checkbox(self, cat, setting, tooltip=None):
        self.settings[setting["variable"]]["checkbox"] = tooltip or ""
        return self.lua.table_from({})

    def _after(self, delay, fn):
        self._seq += 1
        self._timers.append((self.now + max(0.0, float(delay)), self._seq, fn))

    def _install(self):
        g = self.lua.globals()
        self.secret, issecret = self.lua.execute(SECRET_LUA)
        g.LoggingCombat = self._logging_combat
        g.GetTime = lambda: self.now
        g.IsInInstance = self._in_instance
        g.GetInstanceInfo = self._instance_info
        g.C_CVar = self.lua.table_from({"GetCVar": self._get_cvar, "SetCVar": self._set_cvar})
        g.C_ChatInfo = self.lua.table_from({"SendAddonMessage": self._send})
        g.C_Timer = self.lua.table_from({"After": self._after})
        g.secret = self.secret
        g.issecretvalue = issecret
        g.CreateFrame = self._create_frame
        g.IsInGuild = lambda: self.guild is not None
        g.GetGuildInfo = self._guild_info
        g.DEFAULT_CHAT_FRAME = self.lua.table_from({"AddMessage": self._add_message})
        g.SlashCmdList = self.lua.table_from({})
        g.Settings = self.lua.table_from({
            "VarType": self.lua.table_from({"Boolean": "boolean"}),
            "RegisterVerticalLayoutCategory": self._register_category,
            "RegisterAddOnCategory": self._register_addon_category,
            "RegisterProxySetting": self._proxy_setting,
            "CreateCheckbox": self._checkbox,
        })
