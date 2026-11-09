"""Private-data integration harness. No third-party records are stored in this repository.

Run: uv run --with lupa python test/planner/engine_test.py
Override RQP_PRIVATE_DATA to point at quest-db-sources; missing data prints SKIP.
The Lua runtime is explicitly 5.1, including the unmodified Python reference loaders.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import re
import sys

# Private reference sources are read-only, including Python's implicit cache writes.
sys.dont_write_bytecode = True

ROOT = Path(__file__).resolve().parents[2]
PRIVATE = Path(os.environ.get("RQP_PRIVATE_DATA") or "/nonexistent-rqp-private-data")
AVAILABLE = (PRIVATE / "QuestieDB/src/corrections/Forever/combined/foreverQuestDB.lua").is_file()
if AVAILABLE:
    import lupa
    from lupa.lua51 import LuaRuntime, lua_type
    # Reference modules import lupa directly; keep all loaders on the client language.
    lupa.LuaRuntime = LuaRuntime
    lupa.lua_type = lua_type
    sys.path.insert(0, str(PRIVATE / "engine"))


def array(value):
    return [value[i] for i in range(1, len(value) + 1)]


def to_lua(lua, value):
    if isinstance(value, dict):
        return lua.table_from({k: to_lua(lua, v) for k, v in value.items()})
    if isinstance(value, (list, tuple, set)):
        return lua.table_from([to_lua(lua, v) for v in value])
    return value


class Harness:
    def __init__(self, snapshot=None, dungeon_global=True, questie=True, qdb=True):
        self.lua = LuaRuntime(unpack_returned_tuples=True)
        self.g = self.lua.globals()
        self.snapshot = snapshot or json.loads((PRIVATE / "snapshot/snapshot.json").read_text())
        self.snapshot = dict(self.snapshot)
        self.snapshot["skillLines"] = [{"name": n, "rank": r} for n, r in self.snapshot.get("professions", {}).items()]
        self.snapshot["reputationLines"] = [dict(name=n, reaction=r[0], currentStanding=r[1], factionID=r[2])
                                              for n, r in self.snapshot.get("reputation", {}).items()]
        self.g.TEST_SNAPSHOT = to_lua(self.lua, self.snapshot)
        self.lua.execute((ROOT / "test/planner/mocks/wow.lua").read_text())
        self.lua.execute("LibQuestieDB={Enum={}}; TEST_ZONE={private={}}")
        enum = PRIVATE / "QuestieDB/src/corrections/enum"
        self.addon(enum / "expansions.lua", self.g.LibQuestieDB)
        self.lua.execute("""
          local e=LibQuestieDB.Enum
          e.classKeys=e.byExpansion.Classic.classKeys
          e.factionRaceMasks={Alliance=e.byExpansion.Forever.raceKeys.ALL_ALLIANCE,
                             Horde=e.byExpansion.Forever.raceKeys.ALL_HORDE}
          LibQuestieDB.RequireContract=function(n) TEST_CONTRACT=n; return n<=4 end
          LibQuestieDB.Support={Get=function(n) if n=='ZoneDB' then return TEST_ZONE end end}
          Expansions={Era=1,Tbc=2,Wotlk=3,Cata=4,MoP=5,Current=1}
          QuestieLoader={ImportModule=function(_,n)
            if n=='ZoneDB' then return TEST_ZONE elseif n=='Expansions' then return Expansions
            elseif n=='QuestieCorrections' then return {hiddenQuests=TEST_BLACKLIST} end
            return {} end, CreateModule=function() return {} end}
        """)
        for name in ("Quest", "Npc", "Item", "Object"):
            p = PRIVATE / f"QuestieDB/src/corrections/Forever/combined/forever{name}DB.lua"
            src = p.read_text()
            fields = {key: int(index) for index, key in re.findall(r"^-- \[(\d+)\] = (\w+)", src, re.M)}
            self.g.TEST_ROWS = self.lua.execute(src)
            self.g.TEST_FIELDS = to_lua(self.lua, fields)
            self.g.TEST_ENTITY_NAME = name
            self.lua.execute("""
              local rows,fields=TEST_ROWS,TEST_FIELDS
              local lists={startedBy=true,finishedBy=true,objectives=true,objectivesText=true,
                triggerEnd=true,preQuestGroup=true,preQuestSingle=true,childQuests=true,inGroupWith=true,
                exclusiveTo=true,requiredSkill=true,requiredMinRep=true,requiredMaxRep=true,
                requiredSourceItems=true,reputationReward=true,breadcrumbs=true,extraObjectives=true,
                requiredRanks=true,spawns=true,waypoints=true,questStarts=true,questEnds=true,
                npcDrops=true,objectDrops=true,itemDrops=true,questRewards=true,vendors=true,relatedQuests=true}
              local strings={name=true,subName=true,friendlyToFaction=true}
              local api={}
              function api.Get(id,key)
                local row=rows[id]; if not row then return nil end
                local v=row[fields[key]]
                if v==nil and not lists[key] and not strings[key] then return 0 end
                return v
              end
              function api.Exists(id) return rows[id]~=nil end
              function api.GetAllIds()
                local ids={}; for id in pairs(rows) do ids[#ids+1]=id end; table.sort(ids); return ids
              end
              for key in pairs(fields) do local k=key; api[k]=function(id) return api.Get(id,k) end end
              LibQuestieDB[TEST_ENTITY_NAME]=api
              _G[TEST_ENTITY_NAME..'DB']=api
            """)
        support = PRIVATE / "QuestieDB/support/Forever/Zones"
        self.lua.execute((support / "dungeons.lua").read_text())
        self.lua.execute((support / "areaIdToUiMapId.lua").read_text())
        # Use the reference's real blacklist evaluator, not a hand-selected test set.
        from questdb import DB
        blacklist = DB._blacklist(None, LuaRuntime(unpack_returned_tuples=True))
        self.g.TEST_BLACKLIST = to_lua(self.lua, {q: True for q in blacklist})
        if not questie:
            self.lua.execute("QuestieLoader=nil")
        self.lua.execute("""
          local auto; auto=function() return setmetatable({}, {
            __index=function(t,k) local v=auto(); rawset(t,k,v); return v end,
            __call=function() return auto() end, __concat=function() return '' end}) end
          FDJ=setmetatable({Constants=auto(),DB={}}, {
            __index=function(t,k) local v=auto(); rawset(t,k,v); return v end})
          ForeverDungeonJournal_NS=FDJ
          TEST_CREATE_FRAME=CreateFrame
          CreateFrame=function() return auto() end
          hooksecurefunc=function() end; C_Timer=auto(); UIParent=auto(); L=auto()
        """)
        self.fdj_errors = []
        for f in ("Dungeons.lua", "QuestChains.lua", "QuestUpdates.lua", "QuestMaps.lua"):
            try:
                self.addon(PRIVATE / "installed-2026-11-09/AddOns/ForeverDungeonJournal/Data" / f, self.g.FDJ)
            except Exception as exc:
                self.fdj_errors.append((f, str(exc)))
        self.lua.execute("CreateFrame=TEST_CREATE_FRAME")
        if dungeon_global:
            self.g.DungeonGuideForever_NS = self.lua.table()
            self.addon(PRIVATE / "installed-2026-11-09/AddOns/DungeonGuideForever/Data.lua", self.g.DungeonGuideForever_NS)
        if not qdb:
            self.lua.execute("LibQuestieDB=nil")
        for name in ("Data", "Player", "Learn", "Engine"):
            p = ROOT / f"addon/RelaxQuestPlanner/Core/{name}.lua"
            if p.exists():
                self.lua.execute(p.read_text(), name=str(p))
        self.rqp = self.g.RelaxQuestPlanner

    def addon(self, path, ns):
        self.lua.execute("return function(...)\n" + path.read_text() + "\nend", name=str(path))("TestAddon", ns)

    def player(self):
        return self.rqp.Player.Snapshot()

    def route(self, name, player=None, goal=False, **opts):
        self.g.TEST_ROUTE = None
        fn = self.rqp.Engine.BuildGoalAsync if goal else self.rqp.Engine.BuildPathAsync
        fn(name, player or self.player(), to_lua(self.lua, opts), self.lua.eval("function(r) TEST_ROUTE=r end"))
        self.lua.execute("TEST_DriveFrames(100000)")
        assert self.g.TEST_ROUTE is not None, "async callback did not complete"
        return self.g.TEST_ROUTE


class MiniHarness(Harness):
    """Small authored fixtures for gate/cycle regressions, never copied quest data."""
    def __init__(self, rows, snapshot=None):
        self.lua = LuaRuntime(unpack_returned_tuples=True)
        self.g = self.lua.globals()
        self.snapshot = snapshot or dict(level=30, race="Dwarf", raceID=3, faction="Alliance",
                                        completed=[], log=[], professions={}, **{"class": "WARRIOR"})
        self.g.TEST_SNAPSHOT = to_lua(self.lua, self.snapshot)
        self.lua.execute((ROOT / "test/planner/mocks/wow.lua").read_text())
        self.g.TEST_ROWS = to_lua(self.lua, rows)
        self.lua.execute("""
          QuestDB={}
          function QuestDB.Get(id,key) return TEST_ROWS[id] and TEST_ROWS[id][key] end
          function QuestDB.Exists(id) return TEST_ROWS[id]~=nil end
          function QuestDB.GetAllIds()
            local out={}; for q in pairs(TEST_ROWS) do out[#out+1]=q end; table.sort(out); return out
          end
          LibQuestieDB={Quest=QuestDB,RequireContract=function() return true end,
            Enum={raceMaskById={[3]=4,[6]=32,[95]=4294967296},classKeys={WARRIOR=1,HUNTER=4,DRUID=1024}}}
          LibQuestieDB.Support={Get=function() return {private={dungeons={[1]={'Test Dungeon'}}}} end}
        """)
        for name in ("Data", "Player", "Learn", "Engine"):
            p = ROOT / f"addon/RelaxQuestPlanner/Core/{name}.lua"
            self.lua.execute(p.read_text(), name=str(p))
        self.rqp = self.g.RelaxQuestPlanner

