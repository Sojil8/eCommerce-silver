package config

import (
	"strings"
	"text/template"

	"github.com/Sojil8/eCommerce-silver/models/adminModels" // Adjust import based on your project structure
)

func SetupTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(string(s[0])) + s[1:]
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"until": func(count int) []int {
			var result []int
			for i := 0; i < count; i++ {
				result = append(result, i)
			}
			return result
		},
		"mul": func(a float64, b uint) float64 {
			return a * float64(b)
		},
		"anyVariantInStock": func(variants []adminModels.Variants) bool {
			for _, v := range variants {
				if v.Stock > 0 {
					return true
				}
			}
			return false
		},
	}
}
