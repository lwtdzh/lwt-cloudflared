package i18n

import (
	"os"
	"strings"
)

// IsChinese checks if the system locale is set to Chinese
func IsChinese() bool {
	for _, env := range []string{"LANG", "LC_ALL", "LANGUAGE"} {
		val := os.Getenv(env)
		if strings.HasPrefix(strings.ToLower(val), "zh") {
			return true
		}
	}
	return false
}

// T returns the Chinese text if the system locale is Chinese, otherwise returns the English text
func T(en, zh string) string {
	if IsChinese() {
		return zh
	}
	return en
}
