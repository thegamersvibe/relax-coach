"""Read the v2 synthetic save through its reference and real wire decoder."""


def assert_references(test, client):
    """Every dense body index is live; every row resolves within its character."""
    nights = client.lua.globals().RelaxRaidDB["prep"]["nights"]
    test.assertEqual(set(nights.keys()), set(range(1, len(nights) + 1)))
    test.assertLessEqual(len(nights), 3)
    for night in nights.values():
        used = {}
        windows = night["windows"]
        test.assertEqual(set(windows.keys()), set(range(1, len(windows) + 1)))
        test.assertLessEqual(len(windows), 80)
        for window in windows.values():
            test.assertTrue(list(window["reports"].keys()))
            test.assertLessEqual(len(list(window["reports"].keys())), 40)
            for key, row in window["reports"].items():
                bodies = night["reports"][key]
                test.assertIsNotNone(bodies, key)
                test.assertIsInstance(row["report"], int)
                test.assertIsNotNone(bodies[row["report"]], (key, row["report"]))
                used.setdefault(key, set()).add(row["report"])
        test.assertEqual(set(night["reports"].keys()), set(used))
        for key, bodies in night["reports"].items():
            test.assertEqual(set(bodies.keys()), set(range(1, len(bodies) + 1)))
            test.assertEqual(set(bodies.keys()), used[key], "unreferenced body: " + key)


def report_at(client, key="alia-stone-pvp-eu", night=1, window=1):
    n = client.lua.globals().RelaxRaidDB["prep"]["nights"][night]
    index = n["windows"][window]["reports"][key]["report"]
    return client.lua.globals().RelaxRaid.modules.Wire.decode(n["reports"][key][index])
