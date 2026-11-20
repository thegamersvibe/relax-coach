"""Disjoint unittest module partitions; named mutation tests ignore the partition.

Direct file execution still runs the complete suite. No skips or timeout changes.
"""
import sys
import unittest


def main(module, groups):
    args = [arg for arg in sys.argv[1:] if arg.startswith("--partition=")]
    if not args:
        unittest.main(module=module)
        return
    if len(args) != 1:
        raise ValueError("one partition required")
    group = args[0].split("=", 1)[1]
    sys.argv.remove(args[0])
    if group not in groups:
        raise ValueError("unknown partition: " + group)
    # The mutation runner appends -k. Its baseline and mutant must run the same
    # named test even if its ordinary integration lives in a separate module.
    if "-k" in sys.argv:
        unittest.main(module=module)
        return

    def flatten(suite):
        for test in suite:
            if isinstance(test, unittest.TestSuite):
                yield from flatten(test)
            else:
                yield test

    all_tests = list(flatten(unittest.defaultTestLoader.loadTestsFromModule(module)))
    names = {test._testMethodName for test in all_tests}
    explicit = [name for members in groups.values() if members is not None for name in members]
    if len(explicit) != len(set(explicit)) or not set(explicit) <= names:
        raise ValueError("overlapping or missing partition tests")
    if sum(members is None for members in groups.values()) != 1:
        raise ValueError("one remainder partition required")
    wanted = set(groups[group]) if groups[group] is not None else names - set(explicit)
    module.load_tests = lambda loader, tests, pattern: unittest.TestSuite(
        test for test in flatten(tests) if test._testMethodName in wanted)
    unittest.main(module=module)
