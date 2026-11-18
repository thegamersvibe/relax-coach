// This file must NOT compile: model_test.go's TestNightAndRunAreDistinct builds it and expects a type error,
// proving a night scorer cannot be called with a dungeon run.
package main

import "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"

func scoreNight(model.RaidNight) {}

func main() { scoreNight(model.DungeonRun{}) }
