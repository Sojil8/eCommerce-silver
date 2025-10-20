package config

import (
	"html/template"
	"strconv"
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
		"sub": func(a, b interface{}) float64 {
			return toFloat64(a) - toFloat64(b)
		},
		"add": func(a, b interface{}) interface{} {
			if aInt, ok := a.(int); ok {
				if bInt, ok := b.(int); ok {
					return aInt + bInt
				}
			}
			aFloat := toFloat64(a)
			bFloat := toFloat64(b)
			return aFloat + bFloat
		},
		"until": func(count interface{}) []int {
			countInt := 0
			switch v := count.(type) {
			case int:
				countInt = v
			case float64:
				countInt = int(v)
			case string:
				countInt, _ = strconv.Atoi(v)
			}
			var result []int
			for i := 0; i < countInt; i++ {
				result = append(result, i)
			}
			return result
		},
		"mul": func(a, b interface{}) float64 {
			return toFloat64(a) * toFloat64(b)
		},
		"float64": toFloat64,
		"int": func(n interface{}) int {
			switch v := n.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case uint:
				return int(v)
			case float64:
				return int(v)
			case string:
				i, _ := strconv.Atoi(v)
				return i
			default:
				return 0
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
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"lt": func(a, b interface{}) bool {
			return toFloat64(a) < toFloat64(b)
		},
	}
}

func toFloat64(n interface{}) float64 {
	switch v := n.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}