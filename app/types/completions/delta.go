package completions

import "strings"

func DeltaText(text string, previousText string) string {
	if previousText == "" {
		return text
	}
	if strings.HasPrefix(text, previousText) {
		return text[len(previousText):]
	}
	return text
}
