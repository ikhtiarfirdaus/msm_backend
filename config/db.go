package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	err := godotenv.Load()
	if err != nil {
		err = godotenv.Load(".env")
		if err != nil {
			log.Println("info: .env file ga kebaca")
		}
	}

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
