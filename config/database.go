package config

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/nabungyuk/nabungyuk/models"
)

var DB *gorm.DB

// ConnectDB initializes database connection
func ConnectDB() {
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "3306")
	user := GetEnv("DB_USER", "root")
	password := GetEnv("DB_PASSWORD", "")
	dbName := GetEnv("DB_NAME", "nabungyuk")

	// Format: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbName)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Connected to MySQL database successfully")
}

// CloseDB closes the database connection
func CloseDB() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
			log.Println("Database connection closed")
		}
	}
}

// MigrateDB runs auto-migrations for all models
func MigrateDB() {
	if DB == nil {
		log.Fatal("Database not initialized. Call ConnectDB first")
	}

	err := DB.AutoMigrate(
		&models.User{},
		&models.Transaction{},
		&models.SavingGoal{},
		&models.SavingDeposit{},
		&models.Reminder{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration completed successfully")
}

// SetupDatabase sets up the database: connect and migrate
func SetupDatabase() {
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "3306")
	user := GetEnv("DB_USER", "root")
	password := GetEnv("DB_PASSWORD", "")
	dbName := GetEnv("DB_NAME", "nabungyuk")

	// First, connect to MySQL without database to create it if needed
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port)

	var err error
	tempDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("Failed to connect to MySQL server: %v", err)
	}

	// Create database if not exists
	result := tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName))
	if result.Error != nil {
		log.Fatalf("Failed to create database: %v", result.Error)
	}
	log.Printf("Database '%s' is ready", dbName)

	// Close temporary connection
	sqlDB, _ := tempDB.DB()
	sqlDB.Close()

	// Now connect to the target database
	dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbName)

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Connected to MySQL database successfully")
}

// Ensure models are imported for migrations
func init() {
	_ = os.Getenv("DB_NAME") // ensure os import is used
}
