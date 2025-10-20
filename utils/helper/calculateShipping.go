package helper

func CalculateShipping(total float64) float64 {

	shipping := 0.0

	switch {
	case total <= 200:
		shipping = 10
	case total <= 500:
		shipping = 20
	case total <= 1000:
		shipping = 35
	case total <= 2500:
		shipping = 46
	case total <= 5200:
		shipping = 70
	case total > 5200:
		shipping = 100
	}
	return shipping

}
