package config

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/Sojil8/eCommerce-silver/models/adminModels"
	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

func SetupTemplateFunctions() template.FuncMap {
	pkg.Log.Info("Setting up template functions")

	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				pkg.Log.Debug("Title function received empty string")
				return s
			}
			result := strings.ToUpper(string(s[0])) + s[1:]
			pkg.Log.Debug("Title function processed", zap.String("input", s), zap.String("output", result))
			return result
		},
		"sub": func(a, b interface{}) float64 {
			result := toFloat64(a) - toFloat64(b)
			pkg.Log.Debug("Sub function executed",
				zap.Any("a", a),
				zap.Any("b", b),
				zap.Float64("result", result))
			return result
		},
		"add": func(a, b interface{}) interface{} {
			if aInt, ok := a.(int); ok {
				if bInt, ok := b.(int); ok {
					result := aInt + bInt
					pkg.Log.Debug("Add function executed with integers",
						zap.Int("a", aInt),
						zap.Int("b", bInt),
						zap.Int("result", result))
					return result
				}
			}
			aFloat := toFloat64(a)
			bFloat := toFloat64(b)
			result := aFloat + bFloat
			pkg.Log.Debug("Add function executed with floats",
				zap.Any("a", a),
				zap.Any("b", b),
				zap.Float64("result", result))
			return result
		},
		"until": func(count interface{}) []int {
			countInt := 0
			switch v := count.(type) {
			case int:
				countInt = v
			case float64:
				countInt = int(v)
			case string:
				var err error
				countInt, err = strconv.Atoi(v)
				if err != nil {
					pkg.Log.Warn("Failed to convert string to int in until function",
						zap.String("input", v), zap.Error(err))
				}
			}
			var result []int
			for i := 0; i < countInt; i++ {
				result = append(result, i)
			}
			pkg.Log.Debug("Until function executed",
				zap.Any("count", count),
				zap.Int("result_length", len(result)))
			return result
		},
		"mul": func(a, b interface{}) float64 {
			result := toFloat64(a) * toFloat64(b)
			pkg.Log.Debug("Mul function executed",
				zap.Any("a", a),
				zap.Any("b", b),
				zap.Float64("result", result))
			return result
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
				i, err := strconv.Atoi(v)
				if err != nil {
					pkg.Log.Warn("Failed to convert string to int in int function",
						zap.String("input", v), zap.Error(err))
					return 0
				}
				return i
			default:
				pkg.Log.Warn("Unknown type in int function", zap.Any("input", n))
				return 0
			}
		},
		"anyVariantInStock": func(variants []adminModels.Variants) bool {
			for _, v := range variants {
				if v.Stock > 0 {
					pkg.Log.Debug("Found variant in stock", zap.Int("stock", int(v.Stock)))
					return true
				}
			}
			pkg.Log.Debug("No variants in stock", zap.Int("variants_count", len(variants)))
			return false
		},
		"safe": func(s string) template.HTML {
			pkg.Log.Debug("Safe function executed", zap.String("input", s))
			return template.HTML(s)
		},
		"lt": func(a, b interface{}) bool {
			result := toFloat64(a) < toFloat64(b)
			pkg.Log.Debug("Lt function executed",
				zap.Any("a", a),
				zap.Any("b", b),
				zap.Bool("result", result))
			return result
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
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			pkg.Log.Warn("Failed to convert string to float64",
				zap.String("input", v), zap.Error(err))
			return 0
		}
		return f
	default:
		pkg.Log.Warn("Unknown type in toFloat64", zap.Any("input", n))
		return 0
	}
}