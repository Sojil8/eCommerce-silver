package config

import (
	// Adjust import path based on your project structure

	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

type Breadcrumb struct {
	Name string
	URL  string
}

func GenerateBreadcrumbs(items ...Breadcrumb) []Breadcrumb {
	pkg.Log.Info("Generating breadcrumbs", zap.Int("input_items_count", len(items)))

	breadcrumbs := []Breadcrumb{
		{Name: "Home", URL: "/home"},
	}

	// Log input items for debugging
	for i, item := range items {
		pkg.Log.Debug("Input breadcrumb",
			zap.Int("index", i),
			zap.String("name", item.Name),
			zap.String("url", item.URL))
	}

	breadcrumbs = append(breadcrumbs, items...)

	// Log final breadcrumbs for debugging
	pkg.Log.Debug("Generated breadcrumbs", zap.Int("total_count", len(breadcrumbs)))
	for i, crumb := range breadcrumbs {
		pkg.Log.Debug("Breadcrumb details",
			zap.Int("index", i),
			zap.String("name", crumb.Name),
			zap.String("url", crumb.URL))
	}

	return breadcrumbs
}
