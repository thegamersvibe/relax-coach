-- Forever Coach: turns snapshot + gear + talents + dungeons + sessions into ranked findings.
-- Finding: { impact = 1..100, area = "Build|Gear|Play|Next", text, fix, where?, why?, estimate? }
-- Ranking rule: verified + cheap + nearby beats theoretical and far away.
local _, FC = ...
local A = {}
FC.Audit = A

local function clamp(x) return math.max(1, math.min(100, math.floor(x + 0.5))) end
local function add(list, impact, area, text, fix, extra)
    local f = { impact = clamp(impact), area = area, text = text, fix = fix, order = #list }
    if extra then for k, v in pairs(extra) do f[k] = v end end
    list[#list + 1] = f
end

local function recentSessions(c, n)
    local out, s = {}, c.sessions or {}
    for i = #s, math.max(1, #s - (n or 5) + 1), -1 do out[#out + 1] = s[i] end
    local live = FC.Session and FC.Session:Current()
    if live and live.seconds > 60 then table.insert(out, 1, live) end
    return out
end

local function specOk(m, spec)
    if not m.specs then return true end
    for _, s in ipairs(m.specs) do if s == spec then return true end end
    return false
end
local function raceOk(m, race)
    if not m.races then return true end
    for _, r in ipairs(m.races) do if r == race then return true end end
    return false
end

local KIND_WEIGHT = { here = 1.0, route = 0.9, routePast = 0.8, dungeon = 0.75, craft = 0.7, unknown = 0.5, world = 0.4, offroute = 0.35 }

function A:Run(c)
    local snap, gear, T = c.snapshot, c.gear or {}, FC:Rules().thresholds or {}
    local F = {}
    if not snap then return F end
    local lvl, cls = snap.level or 1, snap.class
    local spec = gear.spec

    -- Build: talent points and the guide's order
    local unspent = snap.talents and snap.talents.unspent
    local plan = c.talentPlan
    if type(unspent) == "number" and unspent > 0 and lvl >= 10 then
        local fix = "Open talents (N) and spend them"
        if plan and plan.next then fix = "Next per guide: " .. plan.next.name .. " (" .. tostring(plan.source) .. ")" end
        add(F, 80 + math.min(unspent, 4) * 4, "Build", unspent .. " unspent talent point" .. (unspent > 1 and "s" or ""), fix)
    elseif plan and plan.next and plan.detailed then
        add(F, 35, "Build", "Talents differ from the guide at level " .. plan.next.level, "Guide wants " .. plan.next.name .. " (" .. tostring(plan.source) .. "). Respec only if it's cheap.")
    end
    if plan and plan.detailed and (plan.offPlan or 0) >= 2 then
        add(F, 20, "Build", plan.offPlan .. " talent points outside the guide's plan", "Fine if intentional; " .. tostring(plan.source) .. " for the reference order", { why = "low priority: respecs cost gold" })
    end

    -- Build: class milestones (verified spell IDs; alternatives count; unknown is not "missing")
    -- quests get their own tip (they need a trip); missed trainer skills are grouped into one tip
    local missedTrainer, trainerTop = {}, 0
    for _, m in ipairs(FC:Rules().spellMilestones or {}) do
        if m.class == cls and lvl >= m.level and specOk(m, spec) and raceOk(m, snap.race) then
            local known, answered = false, false
            for _, id in ipairs(m.spellIds or { m.spellId }) do
                local k = snap.spells and snap.spells[id]
                if k == true then known = true end
                if k ~= nil then answered = true end
            end
            if answered and not known then
                local base = m.importance or 60
                if m.kind == "trainer" then
                    missedTrainer[#missedTrainer + 1] = { name = m.name, imp = base }
                    trainerTop = math.max(trainerTop, base)
                else
                    add(F, base + math.min(lvl - m.level, 4) * 2, "Build", m.name .. " not learned (class quest, available since " .. m.level .. ")", m.how)
                end
            end
        end
    end
    if #missedTrainer > 0 then
        table.sort(missedTrainer, function(a, b) return a.imp > b.imp end)
        local names = {}
        for i, m in ipairs(missedTrainer) do if i <= 5 then names[#names + 1] = m.name end end
        local more = #missedTrainer > 5 and (" +" .. (#missedTrainer - 5) .. " more") or ""
        add(F, math.min(trainerTop, 85) + math.min(#missedTrainer, 5), "Build",
            #missedTrainer .. " class skill" .. (#missedTrainer > 1 and "s" or "") .. " not learned: " .. table.concat(names, ", ") .. more,
            "Visit your class trainer")
    end

    -- Build: trainer (class trainers only; cleared by TRAINER_UPDATE/CLOSED after buying)
    local tr = c.trainer
    if tr and (tr.available or 0) > 0 and tr.level == lvl then
        local names = (tr.names and #tr.names > 0) and (": " .. table.concat(tr.names, ", ")) or ""
        add(F, 55, "Build", tr.available .. " skill(s) left to train at your last trainer visit" .. names, "Train when you're next in town")
    elseif lvl >= 4 and lvl % 2 == 0 and (not tr or (tr.level or 0) < lvl) then
        add(F, 30, "Next", "New spell ranks at level " .. lvl, "Train when you're next in town; no special trip needed")
    end

    -- Gear
    if gear.status == "missing" then
        add(F, 15, "Next", "Gear checks off: GearQuest Forever not installed", "Install GearQuest Forever to enable gear checks")
    elseif gear.status == "classdata" or gear.status == "nodata" then
        add(F, 15, "Next", "Gear checks off: GearQuest has no " .. tostring(cls) .. " data", "Enable 'GearQuest Forever: " .. tostring(cls) .. "' in AddOns, then /reload")
    elseif gear.slots then
        if gear.specSource == "default" and lvl >= 10 then
            add(F, 45, "Build", "Gear is scored for the default spec '" .. tostring(spec) .. "'", "Type /coach spec <" .. table.concat(FC.Gear:SpecOptions(cls), "|") .. "> to pick yours")
        end
        -- owned upgrades first: free and instant
        for i, o in ipairs(gear.owned or {}) do
            if i > 3 then break end
            add(F, 88, "Gear", "Upgrade in your bags: " .. tostring(o.name) .. " for " .. tostring(o.label), "Equip it (about +" .. o.gain .. "% for that slot)", { where = "in your bags" })
        end
        local ownedSlot = {}
        for _, o in ipairs(gear.owned or {}) do ownedSlot[o.inv] = true end
        for inv, e in pairs(gear.slots) do
            local b = e.best
            if b and not ownedSlot[inv] then
                local gap = e.gapPct
                local w = KIND_WEIGHT[b.kind] or 0.5
                -- Gear fills name from an optional fact record; a missing one must not abort the whole audit.
                local bestName = b.name or ("item " .. tostring(b.id))
                local how = bestName .. " — " .. (b.how ~= "" and b.how or tostring(b.source))
                if b.notes and #b.notes > 0 then how = how .. " (" .. table.concat(b.notes, "; ") .. ")" end
                local early = lvl < 10 and e.jewelry
                if e.scoreStatus == "empty" and lvl >= 5 and not early then
                    add(F, 30 + 40 * w, "Gear", "Empty " .. e.label .. " slot", how, { where = b.where })
                elseif e.scoreStatus ~= "empty" and e.scoreStatus ~= "unknown" and gap and gap >= (T.gearUpgradePct or 12) then
                    add(F, (25 + math.min(gap, 60) / 2) * (0.5 + w / 2) + 10 * w, "Gear", e.label .. ": " .. bestName .. " is " .. gap .. "% better", how,
                        { where = b.where, estimate = e.scoreStatus == "estimated" or nil })
                end
            end
        end
    end

    -- Bags
    local bi = snap.baginfo
    if bi and bi.known and bi.slots > 0 and bi.free <= (T.bagFreeSlotsWarn or 3) then
        add(F, 40, "Next", "Bags almost full (" .. bi.free .. " free)", "Vendor grey items at the next town")
    end
    if bi and (bi.bagSlotsEmpty or 0) > 0 and lvl >= 5 then
        add(F, 35, "Next", bi.bagSlotsEmpty .. " empty bag slot" .. (bi.bagSlotsEmpty > 1 and "s" or ""), "Buy or equip a bag; more space means fewer vendor trips")
    end

    -- Dungeons: quests to grab before you go
    local dg = c.dungeons
    if dg and dg.list and dg.list[1] then
        local d = dg.list[1]
        local missing = 0
        for _, q in ipairs(d.quests) do if not q.inLog and not q.tooLow then missing = missing + 1 end end
        if missing > 0 then
            add(F, 32, "Next", d.name .. " (" .. tostring(d.level) .. ") fits your level", missing .. " quest(s) to pick up first; see the report's Dungeons section", { where = d.location })
        end
    end

    -- Play (estimates; needs enough session time)
    local ss = recentSessions(c, 5)
    local secs, combat, stopped, dead, afk, taxi, deaths, capped, powerSecs, xp = 0, 0, 0, 0, 0, 0, {}, 0, 0, 0
    local procs = {}
    for _, s in ipairs(ss) do
        secs = secs + (s.seconds or 0); combat = combat + (s.combat or 0); stopped = stopped + (s.recovery or 0)
        dead = dead + (s.dead or 0); afk = afk + (s.afk or 0); taxi = taxi + (s.taxi or 0)
        capped = capped + (s.capped or 0); powerSecs = powerSecs + (s.powerSeconds or 0); xp = xp + (s.xp or 0)
        for _, d in ipairs(s.deaths or {}) do deaths[#deaths + 1] = d end
        for name, p in pairs(s.procs or {}) do
            procs[name] = procs[name] or { windows = 0, used = 0 }
            procs[name].windows = procs[name].windows + p.windows; procs[name].used = procs[name].used + p.used
        end
    end
    local active = secs - afk - taxi - dead
    c.xpPerHour = nil
    if active >= (T.minSessionMinutes or 10) * 60 then
        local hours = active / 3600
        local dph = #deaths / hours
        if #deaths >= 2 and dph >= (T.deathsPerHour or 1) then
            add(F, 45 + math.min(dph, 4) * 8, "Play", string.format("%.1f deaths per hour played (%d recent)", dph, #deaths),
                "Pull one group at a time, keep a potion or bandage ready", { estimate = true })
        end
        local down = stopped / active * 100
        if down >= (T.downtimePct or 30) then
            local fa = snap.skills and snap.skills["First Aid"]
            local tip = (not fa) and "Learn First Aid and carry bandages; " or ""
            local food = (cls == "WARRIOR" or cls == "ROGUE") and "keep food for your level" or "keep food and water for your level"
            add(F, 25 + math.min(down, 60) / 2, "Play", string.format("Standing still %d%% of active play (estimate)", math.floor(down)),
                tip .. food .. ". Includes looting and vendors, so treat it as a rough signal.", { estimate = true })
        end
        if cls == "WARRIOR" and powerSecs > 120 then
            local cp = capped / powerSecs * 100
            if cp >= (T.rageCappedPct or 8) then
                add(F, 30 + math.min(cp, 40) / 2, "Play", string.format("Rage near cap %d%% of combat", math.floor(cp)), "Spend extra rage: Heroic Strike or Cleave above ~60 rage")
            end
        end
        for name, p in pairs(procs) do
            if p.windows >= (T.procMinWindows or 8) then
                local waste = (p.windows - p.used) / p.windows * 100
                if waste >= (T.procWastedPct or 30) then
                    add(F, 30 + math.min(waste, 80) / 3, "Play", string.format("%s: cast %d of %d times it lit up (estimate)", name, p.used, p.windows),
                        "Put " .. name .. " on an easy key and use it when it lights up", { estimate = true })
                end
            end
        end
        if xp > 0 then c.xpPerHour = math.floor(xp / hours) end
        c.activeMinutes = math.floor(active / 60)
    end

    -- Rested XP: only worth a mention when you're not already resting
    if (snap.rested or 0) == 0 and lvl < 60 and not snap.resting then
        add(F, 10, "Next", "No rested XP", "End your session in an inn or a city")
    end

    table.sort(F, function(a, b)
        if a.impact ~= b.impact then return a.impact > b.impact end
        if a.area ~= b.area then return a.area < b.area end
        return a.text < b.text
    end)
    return F
end
