package config

import (
	"html/template"
	"strings"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
)

func SetupTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(string(s[0])) + s[1:]
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"add": func(a, b float64) float64 {
			return a + b
		},
		"until": func(count int) []int {
			var result []int
			for i := 0; i < count; i++ {
				result = append(result, i)
			}
			return result
		},
		"mul": func(a float64, b interface{}) float64 {
			switch v := b.(type) {
			case int:
				return a * float64(v)
			case int64:
				return a * float64(v)
			case uint:
				return a * float64(v)
			case float64:
				return a * v
			default:
				return 0 
			}
		},
		"float64": func(n interface{}) float64 {
			switch v := n.(type) {
			case int:
				return float64(v)
			case int64:
				return float64(v)
			case uint:
				return float64(v)
			case float64:
				return v
			default:
				return 0 // Default case, could log an error here
			}
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