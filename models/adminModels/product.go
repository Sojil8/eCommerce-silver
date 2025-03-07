package adminModels

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

type Product struct {
	*gorm.Model
	ProductName string    `gorm:"not null" json:"productName"`
	Description string    `gorm:"not null" json:"description"`
	Price       float64   `gorm:"not null" json:"price"`
	Category_id uint      `gorm:"not null" json:"category_id"`
	Images      ImageURLs `gorm:"type:text" json:"images"`
	IsListed    bool `gorm:"default:true"`
}


// ImageURLs is a custom type to handle string slice storage in PostgreSQL
type ImageURLs []string

// Value implements the driver.Valuer interface
func (i ImageURLs) Value() (driver.Value, error) {
	return json.Marshal(i)
}

// Scan implements the sql.Scanner interface
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
