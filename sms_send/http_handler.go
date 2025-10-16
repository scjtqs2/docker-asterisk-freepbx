package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// SMSReciveRequest 接收来自 asterisk 的请求结构
type SMSReciveRequest struct {
	Secret    string `json:"secret"`
	Number    string `json:"number"`
	Time      string `json:"time"`
	Text      string `json:"text"`
	Source    string `json:"source"`
	PhoneID   string `json:"phone_id"`
	SMSID     string `json:"sms_id"`
	Timestamp string `json:"timestamp"`
}

// SMSSendRequest defines the structure of the incoming JSON payload.
type SMSSendRequest struct {
	Secret    string `json:"secret"`
	Device    string `json:"device"`
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// APIResponse defines the structure of the JSON response.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type CallRequest struct {
	Secret    string `json:"secret"`
	Number    string `json:"number"`
	Name      string `json:"name"`
	Time      string `json:"time"`
	Type      string `json:"type"` // "incoming", "outgoing", "missed", "ended", "unknown"
	Duration  int    `json:"duration"`
	Source    string `json:"source"`
	PhoneID   string `json:"phone_id"`
	Timestamp string `json:"timestamp"`
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

func setupRoutes() {
	// API v1 分组
	v1 := router.Group("/api/v1")
	{
		v1.POST("/sms/receive", smsHandler)   // 短信接收端点
		v1.POST("/call/receive", callHandler) // 来电接受
		// 短信发送端
		v1.POST("/sms/send", sendSMSHandler)
	}
}

// sendSMSHandler is the HTTP handler for sending SMS.
func sendSMSHandler(c *gin.Context) {
	if Debug {
		//  Read the raw body into a byte slice. This consumes the original body stream.
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Errorf("Error reading request body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Cannot read request body"})
			return
		}

		//  Print the raw body to your log for inspection.
		// This is the line that shows you exactly what was received.
		log.Infof("Received raw request body for /sms/send: %s", string(bodyBytes))

		//  CRITICAL: Put a new reader with the same content back into the request body.
		// This allows c.ShouldBindJSON to read the body as if it were the first time.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	var req SMSSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "无效的 JSON 数据: " + err.Error(),
		})
		return
	}
	// 验证密钥（可选）
	if err := validateSecret(req.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "认证失败",
		})
		return
	}
	if req.Recipient == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing required fields: recipient and message",
		})
		return
	}

	// Default device if not provided
	if req.Device == "" {
		req.Device = "quectel0"
	}

	// Get AMI config from DB for every request to ensure it's up to date
	amiConfig, err := GetAMIConfigFromDB(db)
	if err != nil {
		log.Printf("Error getting AMI config from DB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to get AMI configuration",
		})
		return
	}

	// Send the SMS via AMI
	// amiResponse, err := SendSMSOriginate(amiConfig, req.Device, req.Recipient, req.Message)
	// 在同一个容器内，通过shell直接写
	amiResponse, err := SendSMSShell(amiConfig, req.Device, req.Recipient, req.Message)
	if err != nil {
		log.Printf("Error sending SMS via AMI: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to send SMS" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "短信接收并处理成功: " + amiResponse,
	})
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// smsHandler 处理来自 gammu-smsd 的短信推送
func smsHandler(c *gin.Context) {
	if Debug {
		//  Read the raw body into a byte slice. This consumes the original body stream.
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Errorf("Error reading request body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Cannot read request body"})
			return
		}

		//  Print the raw body to your log for inspection.
		// This is the line that shows you exactly what was received.
		log.Infof("Received raw request body for /sms/received: %s", string(bodyBytes))

		//  CRITICAL: Put a new reader with the same content back into the request body.
		// This allows c.ShouldBindJSON to read the body as if it were the first time.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	var smsReq SMSReciveRequest
	if err := c.ShouldBindJSON(&smsReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "无效的 JSON 数据: " + err.Error(),
		})
		return
	}

	// 验证密钥（可选）
	if err := validateSecret(smsReq.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "认证失败",
		})
		return
	}
	log.WithFields(log.Fields{
		"number":   smsReq.Number,
		"time":     smsReq.Time,
		"source":   smsReq.Source,
		"sms_id":   smsReq.SMSID,
		"phone_id": smsReq.PhoneID,
	}).Info("收到短信推送")

	// 处理短信转发
	if err := processSMS(smsReq.Number, smsReq.Time, smsReq.Text, smsReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "处理短信失败: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "短信接收并处理成功",
	})
}
func validateSecret(secret string) error {
	expectedSecret := os.Getenv("FORWARD_SECRET")
	if expectedSecret != "" && secret != expectedSecret {
		return fmt.Errorf("密钥验证失败")
	}
	return nil
}

func processSMS(sender, time, text string, smsReq SMSReciveRequest) error {
	log.WithFields(log.Fields{
		"sender": sender,
		"time":   time,
		"text":   text,
	}).Info("开始处理短信")

	// 遍历所有配置的转发规则
	for name, cfg := range config {
		c, ok := cfg.(map[string]interface{})
		if !ok {
			log.Warnf("配置格式错误: %s", name)
			continue
		}

		rule, ok := c["rule"].(string)
		if !ok {
			log.Warnf("规则配置错误: %s", name)
			continue
		}

		ruleType, ok := c["type"].(string)
		if !ok {
			log.Warnf("类型配置错误: %s", name)
			continue
		}

		// 根据规则类型匹配
		if shouldSendNotification(ruleType, rule, text) {
			log.Infof("触发规则: %s, 类型: %s", name, ruleType)
			sendNotification(c, sender, time, text, rule, smsReq)
		}
	}

	return nil
}

func shouldSendNotification(ruleType, rule, text string) bool {
	switch ruleType {
	case "all":
		return true
	case "keyword":
		return strings.Contains(text, rule)
	case "regex":
		matched, err := regexp.MatchString(rule, text)
		if err != nil {
			log.Errorf("正则表达式错误: %v", err)
			return false
		}
		return matched
	default:
		log.Warnf("未知的规则类型: %s", ruleType)
		return false
	}
}

// callHandler 处理来自 gammu-smsd 的来电推送
func callHandler(c *gin.Context) {
	if Debug {
		//  Read the raw body into a byte slice. This consumes the original body stream.
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Errorf("Error reading request body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Cannot read request body"})
			return
		}

		//  Print the raw body to your log for inspection.
		// This is the line that shows you exactly what was received.
		log.Infof("Received raw request body for /call/reviced: %s", string(bodyBytes))

		//  CRITICAL: Put a new reader with the same content back into the request body.
		// This allows c.ShouldBindJSON to read the body as if it were the first time.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	var callReq CallRequest
	if err := c.ShouldBindJSON(&callReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "无效的 JSON 数据: " + err.Error(),
		})
		return
	}

	// 验证密钥（可选）
	if err := validateSecret(callReq.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "认证失败",
		})
		return
	}
	log.WithFields(log.Fields{
		"number":   callReq.Number,
		"name":     callReq.Name,
		"time":     callReq.Time,
		"source":   callReq.Source,
		"type":     callReq.Type,
		"phone_id": callReq.PhoneID,
		"duration": callReq.Duration,
	}).Info("收到call推送")

	// 处理短信转发
	if err := processCALL(callReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "处理call失败: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "call接收并处理成功",
	})
}

func processCALL(callReq CallRequest) error {
	log.WithFields(log.Fields{
		"number":   callReq.Number,
		"name":     callReq.Name,
		"time":     callReq.Time,
		"type":     callReq.Type,
		"duration": callReq.Duration,
	}).Info("开始处理call")

	// 遍历所有配置的转发规则
	for name, cfg := range config {
		c, ok := cfg.(map[string]interface{})
		if !ok {
			log.Warnf("call配置格式错误: %s", name)
			continue
		}

		rule, ok := c["rule"].(string)
		if !ok && rule != "all" {
			log.Warnf("call规则配置错误: %s", name)
			continue
		}

		ruleType, ok := c["type"].(string)
		if !ok && ruleType != "all" {
			log.Warnf("call类型配置错误: %s", name)
			continue
		}
		// 根据规则类型匹配
		sendCallNotification(c, rule, callReq)
	}

	return nil
}
