package adminModels

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

type Product struct {
	*gorm.Model
	ProductName  string    `gorm:"not null" json:"productName"`
	Description  string    `gorm:"not null" json:"description"`
	Price        float64   `gorm:"not null" json:"price"`
	Quantity     uint      `gorm:"not null" json:"quantity"`
	CategoryName string    `gorm:"not null" json:"categoryName"`
	Images       ImageURLs `gorm:"type:text" json:"images"`
	IsListed     bool      `gorm:"default:true"`
}

type ImageURLs []string

func (i ImageURLs) Value() (driver.Value, error) {
	return json.Marshal(i)
}

func (i *ImageURLs) Scan(value interface{}) error {
	if value == nil {
		*i = []string{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to scan ImageURLs")
	}

	return json.Unmarshal(bytes, i)
}
