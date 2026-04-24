package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("MYSQL_URL")

	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/msm_backend?charset=utf8mb4&parseTime=True&loc=Local"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Database gagal terhubung: %v", err)
	}
	DB = db

	fmt.Println("database terhubung ke railway")
}
