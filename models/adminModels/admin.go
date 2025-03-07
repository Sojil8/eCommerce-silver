package adminModels

type Admin struct {
	ID       uint   `gorm:"primary key"`
	UserName string `gorm:"notnull" json:"username"`
	Email    string `gorm:"unique" json:"email"`
	Password string `gorm:"notnull" json:"password"`
}
