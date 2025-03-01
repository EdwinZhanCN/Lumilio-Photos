package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(dbName string) *gorm.DB {
	dsn := "host=localhost user=gorm password=gorm dbname=" + dbName + " port=9920 sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		panic(err.Error())
	}

	return db
}
