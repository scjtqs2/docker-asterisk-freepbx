package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	Device    string `json:"device"`
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
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

// Conversation represents a summary of an SMS conversation.
type Conversation struct {
	OtherParty    string    `json:"other_party"`
	LastMessage   string    `json:"last_message"`
	LastMessageAt time.Time `json:"last_message_at"`
	TotalMessages int       `json:"total_messages"`
}

// SMSMessage represents a single SMS message in a conversation.
type SMSMessage struct {
	ID         int       `json:"id"`
	Direction  string    `json:"direction"`
	FromNumber string    `json:"from_number"`
	ToNumber   string    `json:"to_number"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	PhoneID    string    `json:"phone_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func setupRoutes() {
	// Public API group for receiving data from Asterisk/Gammu
	publicApi := router.Group("/api/v1")
	{
		publicApi.POST("/sms/receive", smsHandler)
		publicApi.POST("/call/receive", callHandler)
	}

	// Authenticated API group for frontend
	authApi := router.Group("/api/v1")
	authApi.Use(authMiddleware())
	{
		authApi.POST("/sms/send", sendSMSHandler)
		authApi.GET("/sms/conversations", getConversationsHandler)
		authApi.GET("/sms/conversation/:number", getConversationDetailsHandler)
	}

	// Standalone auth validation route
	router.POST("/api/v1/auth/validate", validateSecretHandler)

	// Route for the conversation detail page
	router.GET("/conversation/:number", func(c *gin.Context) {
		c.HTML(http.StatusOK, "conversation.html", gin.H{
			"Number": c.Param("number"),
		})
	})
}

// authMiddleware checks for the X-Auth-Secret header.
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Auth-Secret")
		if err := validateSecret(secret); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, APIResponse{Success: false, Message: "Authentication failed"})
			return
		}
		c.Next()
	}
}

// validateSecretHandler allows the frontend to check if a secret is valid.
func validateSecretHandler(c *gin.Context) {
	var req struct {
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	if err := validateSecret(req.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Message: "Invalid secret"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "Secret is valid"})
}

// getConversationsHandler handles fetching the list of SMS conversations.
func getConversationsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	query := `
        SELECT 
            other_party, 
            (SELECT body FROM sms_log WHERE id = T.max_id) as last_message, 
            (SELECT created_at FROM sms_log WHERE id = T.max_id) as last_message_at, 
            T.total_messages
        FROM (
            SELECT 
                IF(direction = 'incoming', from_number, to_number) as other_party, 
                MAX(id) as max_id, 
                COUNT(*) as total_messages
            FROM sms_log
            GROUP BY other_party
        ) AS T
        ORDER BY last_message_at DESC
        LIMIT ? OFFSET ?;
    `

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Errorf("Error querying conversations: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Message: "Failed to retrieve conversations"})
		return
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.OtherParty, &conv.LastMessage, &conv.LastMessageAt, &conv.TotalMessages); err != nil {
			log.Errorf("Error scanning conversation row: %v", err)
			continue
		}
		conversations = append(conversations, conv)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: conversations})
}

// getConversationDetailsHandler handles fetching all messages for a specific number.
func getConversationDetailsHandler(c *gin.Context) {
	number := c.Param("number")

	query := `
        SELECT id, direction, from_number, to_number, body, status, phone_id, created_at 
        FROM sms_log 
        WHERE from_number = ? OR to_number = ? 
        ORDER BY created_at ASC;
    `

	rows, err := db.Query(query, number, number)
	if err != nil {
		log.Errorf("Error querying conversation details for %s: %v", number, err)
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Message: "Failed to retrieve conversation details"})
		return
	}
	defer rows.Close()

	var messages []SMSMessage
	for rows.Next() {
		var msg SMSMessage
		var phoneID sql.NullString // Handle possible NULL phone_id
		if err := rows.Scan(&msg.ID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.Body, &msg.Status, &phoneID, &msg.CreatedAt); err != nil {
			log.Errorf("Error scanning message row: %v", err)
			continue
		}
		msg.PhoneID = phoneID.String
		messages = append(messages, msg)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: messages})
}

// sendSMSHandler is the HTTP handler for sending SMS.
func sendSMSHandler(c *gin.Context) {
	var req SMSSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Message: "无效的 JSON 数据: " + err.Error()})
		return
	}

	if req.Recipient == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Message: "Missing required fields: recipient and message"})
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
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Message: "Failed to get AMI configuration"})
		return
	}

	// Send the SMS via shell command
	amiResponse, err := SendSMSShell(amiConfig, req.Device, req.Recipient, req.Message)

	smsStatus := "sent"
	if err != nil {
		smsStatus = "failed"
		log.Printf("Error sending SMS via AMI: %v", err)
		// Log the failed attempt before returning an error response
		if logErr := insertSMSLog("outgoing", "unknown", req.Recipient, req.Message, smsStatus, req.Device); logErr != nil {
			log.Errorf("Failed to log outgoing SMS: %v", logErr)
		}
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Message: "Failed to send SMS: " + err.Error()})
		return
	}

	// Log the successful outgoing SMS
	if logErr := insertSMSLog("outgoing", "unknown", req.Recipient, req.Message, smsStatus, req.Device); logErr != nil {
		log.Errorf("Failed to log outgoing SMS: %v", logErr)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "短信发送成功: " + amiResponse})
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Auth-Secret")
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
	var smsReq SMSReciveRequest
	if err := c.ShouldBindJSON(&smsReq); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Message: "无效的 JSON 数据: " + err.Error()})
		return
	}

	// 验证密钥（可选）
	if err := validateSecret(smsReq.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Message: "认证失败"})
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
	if err := processSMS(smsReq); err != nil {
		log.Errorf("Failed to process SMS for forwarding: %v", err)
	}

	// Log the incoming SMS
	if logErr := insertSMSLog("incoming", smsReq.Number, "unknown", smsReq.Text, "received", smsReq.PhoneID); logErr != nil {
		log.Errorf("Failed to log incoming SMS: %v", logErr)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "短信接收并处理成功"})
}
func validateSecret(secret string) error {
	expectedSecret := os.Getenv("FORWARD_SECRET")
	if expectedSecret == "" {
		log.Warn("FORWARD_SECRET is not set. Authentication is disabled.")
		return nil // If no secret is configured, allow access
	}
	if secret != expectedSecret {
		return fmt.Errorf("密钥验证失败")
	}
	return nil
}

func processSMS(smsReq SMSReciveRequest) error {
	log.WithFields(log.Fields{
		"sender": smsReq.Number,
		"time":   smsReq.Time,
		"text":   smsReq.Text,
	}).Info("开始处理短信")

	// Parse the ISO 8601 time string from the request
	parsedTime, err := time.Parse(time.RFC3339, smsReq.Time)
	if err != nil {
		log.Warnf("Could not parse time string '%s', using current time. Error: %v", smsReq.Time, err)
		parsedTime = time.Now()
	}

	// Format the time for the notification message
	loc, _ := time.LoadLocation("Asia/Shanghai")
	formattedTime := parsedTime.In(loc).Format("2006-01-02 15:04:05")

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
		if shouldSendNotification(ruleType, rule, smsReq.Text) {
			log.Infof("触发规则: %s, 类型: %s", name, ruleType)
			sendNotification(c, smsReq.Number, formattedTime, smsReq.Text, rule, smsReq)
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
	var callReq CallRequest
	if err := c.ShouldBindJSON(&callReq); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Message: "无效的 JSON 数据: " + err.Error()})
		return
	}

	// 验证密钥（可选）
	if err := validateSecret(callReq.Secret); err != nil {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Message: "认证失败"})
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

	// Process call for forwarding (if any)
	if err := processCALL(callReq); err != nil {
		log.Errorf("Failed to process call for forwarding: %v", err)
	}

	// Log the call
	if logErr := insertCallLog(callReq.Type, callReq.Number, callReq.Name, callReq.Duration, callReq.Time, callReq.PhoneID, callReq.Source); logErr != nil {
		log.Errorf("Failed to log call: %v", logErr)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Message: "call接收并处理成功"})
}

func processCALL(callReq CallRequest) error {
	log.WithFields(log.Fields{
		"number":   callReq.Number,
		"name":     callReq.Name,
		"time":     callReq.Time,
		"type":     callReq.Type,
		"duration": callReq.Duration,
	}).Info("开始处理call")

	// Parse the ISO 8601 time string from the request
	parsedTime, err := time.Parse(time.RFC3339, callReq.Time)
	if err != nil {
		log.Warnf("Could not parse time string '%s', using current time. Error: %v", callReq.Time, err)
		parsedTime = time.Now()
	}

	// Format the time for the notification message
	loc, _ := time.LoadLocation("Asia/Shanghai")
	callReq.Time = parsedTime.In(loc).Format("2006-01-02 15:04:05")

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

func insertSMSLog(direction, fromNumber, toNumber, body, status, phoneID string) error {
	query := `INSERT INTO sms_log (direction, from_number, to_number, body, status, phone_id) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, direction, fromNumber, toNumber, body, status, phoneID)
	if err != nil {
		return fmt.Errorf("failed to insert SMS log: %w", err)
	}
	log.Infof("SMS logged: Direction=%s, From=%s, To=%s, Status=%s", direction, fromNumber, toNumber, status)
	return nil
}

func insertCallLog(callType, phoneNumber, contactName string, durationSeconds int, callTime, phoneID, source string) error {
	query := `INSERT INTO call_log (call_type, phone_number, contact_name, duration_seconds, call_time, phone_id, source) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, callType, phoneNumber, contactName, durationSeconds, callTime, phoneID, source)
	if err != nil {
		return fmt.Errorf("failed to insert call log: %w", err)
	}
	log.Infof("Call logged: Type=%s, Number=%s, Duration=%d", callType, phoneNumber, durationSeconds)
	return nil
}
