package util

func Contains(list []string, wantedItem string) bool {
	for _, item := range list {
		if item == wantedItem {
			return true
		}
	}
	return false
}
