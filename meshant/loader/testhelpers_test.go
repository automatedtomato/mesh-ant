// testhelpers_test.go provides shared test helpers for the loader_test package.
// Helpers defined here are available to all _test.go files in this directory.
package loader_test

import "sort"

// sortedKeys extracts the keys of a map[string]bool sorted alphabetically.
// Used in error messages where map iteration order would otherwise be
// non-deterministic across test runs.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
