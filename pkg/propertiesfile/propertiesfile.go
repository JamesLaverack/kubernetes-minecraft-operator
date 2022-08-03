package propertiesfile

import (
	"fmt"
	"strings"
)

// Write formats a .properties file of key-value pairs, using an equals sign as a delimiter.
func Write(keysAndValues map[string]string) string {
	sb := strings.Builder{}
	for k, v := range keysAndValues {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return sb.String()
}
