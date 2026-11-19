"""Read the v2 synthetic save through its reference and real wire decoder."""


def report_at(client, key="alia-stone-pvp-eu", night=1, window=1):
    n = client.lua.globals().RelaxRaidDB["prep"]["nights"][night]
    index = n["windows"][window]["reports"][key]["report"]
    return client.lua.globals().RelaxRaid.modules.Wire.decode(n["reports"][key][index])
