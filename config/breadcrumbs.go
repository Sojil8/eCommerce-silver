package config

type Breadcrumb struct {
	Name string
	URL  string
}

func GenerateBreadcrumbs(items ...Breadcrumb) []Breadcrumb {
	breadcrumbs := []Breadcrumb{
		{Name: "Home", URL: "/home"},
	}
	breadcrumbs = append(breadcrumbs, items...)

	return breadcrumbs
}
