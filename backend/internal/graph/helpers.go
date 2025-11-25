package graph

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
