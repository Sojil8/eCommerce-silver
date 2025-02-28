package adminModels

type Admin struct {
	Id       uint   `gorm:"primary key"`
	UserName string `gorm:"notnull" json:"username"`
	Email    string `gorm:"unique" json:"email"`
	Password string `gorm:"notnull" json:"password"`
}
