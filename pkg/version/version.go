package version

import "strings"

func ParseMinorVersion(version string) string {
	if strings.Count(version, ".") > 1 {
		splits := strings.Split(version, ".")
		return splits[0] + "." + splits[1]
	}
	return version
}
