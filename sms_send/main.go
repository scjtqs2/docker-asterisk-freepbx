package main

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

var (
	viperconfig *viper.Viper
	config      = map[string]interface{}{}
	router      *gin.Engine
	db          *sql.DB
	Debug       bool
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("启动短信转发服务...")
	// 读取配置文件
	if err := initConfig(); err != nil {
		log.Errorf("初始化配置失败: %v", err)
	}

	// --- Database Connection ---
	dbHost := os.Getenv("DBHOST")
	dbPort := os.Getenv("DBPORT")
	dbUser := os.Getenv("DBUSER")
	dbPass := os.Getenv("DBPASS")
	dbName := os.Getenv("DBNAME")
	Debug = os.Getenv("DEBUG") == "true"
	if dbHost == "" || dbUser == "" || dbPass == "" || dbName == "" {
		log.Fatal("Database environment variables (DBHOST, DBUSER, DBPASS, DBNAME) must be set.")
	}
	if dbPort == "" {
		dbPort = "3306" // Default MySQL port
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)

	var err error

	// Retry connecting to the database on startup
	for i := 0; i < 10; i++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Successfully connected to the database.")
				break
			}
		}
		log.Printf("Failed to connect to database, retrying in 5 seconds... (Attempt %d/10)", i+1)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatalf("Could not connect to the database after several retries: %v", err)
	}
	defer db.Close()

	// 初始化 Gin
	initGin()

	// 启动 HTTP 服务器
	startHTTPServer()
}
