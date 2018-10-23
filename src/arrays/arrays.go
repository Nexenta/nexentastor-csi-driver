package arrays

// ContainsString - true if an array contains a string
func ContainsString(array []string, value string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
}
