package render

// Support check whether the topic is supported by render service
func Support(topicType string) (support bool) {
	supportTypes := map[string]struct{}{
		"talk": {},
		"q&a":  {},
	}

	if _, ok := supportTypes[topicType]; !ok {
		return false
	}
	return true
}
