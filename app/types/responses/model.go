package responses

import "strings"

func NormalizeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "auto"
	}
	return model
}
