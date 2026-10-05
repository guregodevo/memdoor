package cmd

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
