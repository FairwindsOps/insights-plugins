package util

// Contains returns true if the string is present in the slice.
func Contains(list []string, wantedItem string) bool {
	for _, item := range list {
		if item == wantedItem {
			return true
		}
	}
	return false
}
