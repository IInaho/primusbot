package system

import "fmt"

// FormatCwd renders the working-directory block injected into prompts.
func FormatCwd(cwd string) string {
	return fmt.Sprintf("<cwd>%s</cwd>", cwd)
}

// FormatEnv renders the environment block injected into prompts.
func FormatEnv(cwd, date, goos, goarch string) string {
	return fmt.Sprintf("<env>\n<cwd>%s</cwd>\n<date>%s</date>\n<os>%s</os>\n<arch>%s</arch>\n</env>", cwd, date, goos, goarch)
}
