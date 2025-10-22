package main

import (
	"database/sql"
	"embed"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"time"
)

var (
	viperconfig *viper.Viper
	config      = map[string]interface{}{}
	router      *gin.Engine
	db          *sql.DB
	Debug       bool
)

//go:embed all:web
var staticFiles embed.FS

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("启动短信转发服务...")
	// 读取配置文件
	dbConfig, err := initConfig()
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	Debug = os.Getenv("DEBUG") == "true"

	// --- Database Connection ---
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.DBName)

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
	// defer db.Close() // Keep DB connection open for the lifetime of the app

	// Create database tables if they don't exist
	if err := createSMSTable(); err != nil {
		log.Fatalf("Failed to create sms_log table: %v", err)
	}
	if err := createCallLogTable(); err != nil {
		log.Fatalf("Failed to create call_log table: %v", err)
	}

	// 初始化 Gin
	initGin()

	// 启动 HTTP 服务器
	startHTTPServer()
}

func createSMSTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS sms_log (
		id INT AUTO_INCREMENT PRIMARY KEY,
		direction VARCHAR(10) NOT NULL, -- 'incoming' or 'outgoing'
		from_number VARCHAR(50) NOT NULL,
		to_number VARCHAR(50) NOT NULL,
		body TEXT NOT NULL,
		status VARCHAR(20) NOT NULL, -- 'received', 'sent', 'failed'
		phone_id VARCHAR(50),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("error creating sms_log table: %w", err)
	}
	log.Println("sms_log table verified/created successfully.")
	return nil
}

func createCallLogTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS call_log (
		id INT AUTO_INCREMENT PRIMARY KEY,
		call_type VARCHAR(20) NOT NULL, -- 'incoming', 'outgoing', 'missed', etc.
		phone_number VARCHAR(50) NOT NULL,
		contact_name VARCHAR(100),
		duration_seconds INT,
		call_time VARCHAR(50),
		phone_id VARCHAR(50),
		source VARCHAR(50),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("error creating call_log table: %w", err)
	}
	log.Println("call_log table verified/created successfully.")
	return nil
}

func initGin() {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 创建路由
	router = gin.Default()

	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())

	// Serve static files from embedded filesystem
	staticFS, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		log.Fatalf("Failed to create static filesystem: %v", err)
	}
	router.StaticFS("/static", http.FS(staticFS))

	// Serve HTML templates from embedded filesystem
	htmlFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		log.Fatalf("Failed to create html filesystem: %v", err)
	}
	router.SetHTMLTemplate(template.Must(template.New("").ParseFS(htmlFS, "*.html")))

	// Root routes to serve the HTML pages
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})

	// 设置路由
	setupRoutes()
}

func startHTTPServer() {
	httpPort := os.Getenv("SMS_SEND_PORT")
	if httpPort == "" {
		httpPort = "1285"
	}
	log.Infof("HTTP 服务启动，监听端口 %s", httpPort)
	if err := router.Run(":" + httpPort); err != nil {
		log.Fatalf("HTTP 服务启动失败: %v", err)
	}
}
