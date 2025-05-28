package config

import (
	"log"

	"github.com/Sojil8/eCommerce-silver/helper"
)

func Cloudnary() {
	if err := helper.InitCloudinary(); err != nil {
		log.Fatal(err)
	}
}
