package helper

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"go.uber.org/zap"
)

func GenerateOrderID() string {
	pkg.Log.Debug("Generating order ID")

	timestamp := time.Now().Format("20060102")
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNum := rng.Intn(10000)
	orderID := fmt.Sprintf("ORD-%s-%04d", timestamp, randomNum)

	pkg.Log.Info("Order ID generated successfully",
		zap.String("orderID", orderID))
	return orderID
}