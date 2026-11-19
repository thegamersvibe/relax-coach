-- Planner data, no logic. Quests whose work is inside a dungeon although QuestieDB lists no objective
-- rows and no end trigger for them (custom quests), so the planner cannot see it: questID -> the dungeon,
-- by a name RelaxQuestPlanner's dungeon list knows. Read through RelaxQuestPlanner.Data.PlaceInside.
RelaxQuestPlanner = RelaxQuestPlanner or {}
RelaxQuestPlanner.PlaceInside = {
    [95682] = "Excavation Site: Wetlands",
    [95776] = "The Drowned City",
}
