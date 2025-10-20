package adminModels

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	ProductName   string     `gorm:"not null" json:"productName"`
	Description   string     `gorm:"not null" json:"description"`
	Brand         string     `json:"brand"`
	Price         float64    `gorm:"not null" json:"price"`
	OriginalPrice float64    `json:"originalPrice"`
	CategoryName  string     `gorm:"not null" json:"categoryName"`
	CategoryID    uint       `gorm:"not null" json:"categoryId"`
	Images        ImageURLs  `gorm:"type:text" json:"images"`
	IsListed      bool       `gorm:"default:true"`
	InStock       bool       `gorm:"default:true"`
	Variants      []Variants `gorm:"foreignKey:ProductID" json:"variants,omitempty"`
	Offers        []ProductOffer
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
