package services

import (
	"log"

	"github.com/Sojil8/eCommerce-silver/utils/helper"
)

func Cloudnary() {
	if err := helper.InitCloudinary(); err != nil {
		log.Fatal(err)
	}
}
