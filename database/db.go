package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDb() {
	var err error
	dsn := os.Getenv("dataBase")
	if dsn == "" {
		fmt.Println("--------------------dsn is missing in env file---------------------")
	}
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("connetion to database faild")
	}
	fmt.Println("-----------------------database conneted------------------------------")

}

func GetDB() *gorm.DB {
	if DB == nil {
		ConnectDb()
	}
	return DB
}