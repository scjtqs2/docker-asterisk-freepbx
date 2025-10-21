package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func sendNotification(config map[string]interface{}, sender string, time string, text string, rule string, smsReq SMSReciveRequest) {
	message := fmt.Sprintf("触发规则: %s\n发送时间: %s\n发送人: %s \nphoneID: %s\n短信内容: %s\nSource: %s", rule, time, sender, smsReq.PhoneID, text, smsReq.Source)
	messagePhone := fmt.Sprintf("%s\n%s\n%s\n%s", text, smsReq.PhoneID, smsReq.Time, smsReq.Source)
	sendForward(config, "短信通知", sender, message, messagePhone)
}

func sendCallNotification(config map[string]interface{}, rule string, callReq CallRequest) {
	message := fmt.Sprintf("发送时间: %s\n发送人: %s \n%s\nphoneID: %s\n来电号码: %s\nSource: %s", callReq.Time, callReq.Number, callReq.Type, callReq.PhoneID, callReq.Name, callReq.Source)
	messagePhone := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", callReq.Number, callReq.Type, callReq.PhoneID, callReq.Time, callReq.Name, callReq.Source)
	sendForward(config, "来电通知", "来电通知", message, messagePhone)
}

func sendForward(config map[string]interface{}, title string, mobileTitle string, message string, messagePhone string) {
	notifyType, ok := config["notify"].(string)
	if !ok {
		log.Error("通知类型配置错误")
		return
	}

	switch notifyType {
	case "wechat":
		url, ok := config["url"].(string)
		if ok {
			sendWechat(url, title, message)
		}
	case "bark":
		url, ok := config["url"].(string)
		if ok {
			sendBark(url, mobileTitle, messagePhone)
		}
	case "gotify":
		url, ok1 := config["url"].(string)
		token, ok2 := config["token"].(string)
		if ok1 && ok2 {
			sendGotify(url, token, mobileTitle, messagePhone)
		}
	case "email":
		smtpHost, ok1 := config["smtp_host"].(string)
		smtpPort, ok2 := config["smtp_port"].(string)
		username, ok3 := config["username"].(string)
		password, ok4 := config["password"].(string)
		from, ok5 := config["from"].(string)
		to, ok6 := config["to"].(string)
		if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
			sendEmail(smtpHost, smtpPort, username, password, from, to, title, message)
		}
	case "qq":
		qq, ok1 := config["qq"].(string)
		token, ok2 := config["token"].(string)
		if ok1 && ok2 {
			sendQQPush(token, qq, fmt.Sprintf("%s\n%s", title, message))
		}
	case "feishu":
		url, ok := config["url"].(string)
		if ok {
			sendFeishu(url, title, message)
		}
	case "dingtalk":
		url, ok := config["url"].(string)
		if ok {
			sendDingtalk(url, title, message)
		}
	case "telegram":
		botToken, ok1 := config["bot_token"].(string)
		chatID, ok2 := config["chat_id"].(string)
		proxyURL, _ := config["proxy"].(string) // 代理配置，可选
		if ok1 && ok2 {
			sendTelegram(botToken, chatID, message, proxyURL)
		}
	default:
		log.Warnf("未知的通知类型: %s", notifyType)
	}
}

func sendWechat(url string, title, message string) {
	type Content struct {
		Content string `json:"content"`
	}
	type body struct {
		Msgtype string  `json:"msgtype"`
		Text    Content `json:"text"`
	}
	messend := body{Msgtype: "text", Text: Content{fmt.Sprintf("%s\n%s", title, message)}}
	// smssend := fmt.Sprintf(`{"msgtype":"text","text":{"content":"触发规则: %s \n发送时间: %s \n发送人：%s \n短信内容：%s"}}`, rule, ReceivingDateTime, SenderNumber, TextDecoded)
	jsonBytes, _ := json.Marshal(messend)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Errorf("创建微信请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送微信通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Infof("微信通知响应状态: %s", resp.Status)
}

// BarkRequest Bark请求参数
type BarkRequest struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	IsArchive int    `json:"isArchive,omitempty"`
	Group     string `json:"group,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Level     string `json:"level,omitempty"`
	Sound     string `json:"sound,omitempty"`
	Badge     string `json:"badge,omitempty"`
	URL       string `json:"url,omitempty"`
	Call      string `json:"call,omitempty"`
	Copy      string `json:"copy,omitempty"`
	AutoCopy  int    `json:"autoCopy,omitempty"`
}

func sendBark(url, title, body string) {
	// 构建请求参数
	msgMap := BarkRequest{
		Title:     title,
		Body:      body,
		IsArchive: 1,
	}

	// 检测验证码模式 - 使用修复后的函数
	pattern := `(?i)(验证码|授权码|校验码|检验码|确认码|激活码|动态码|安全码|验证代码|CODE|Verification)`
	matched, _ := regexp.MatchString(pattern, body)
	if matched {
		// 提取验证码
		code := extractVerificationCode(body)
		if code != "" {
			msgMap.Copy = code
			msgMap.AutoCopy = 1
			log.Infof("检测到验证码: %s", code)
		}
	}

	// 序列化请求数据
	requestMsg, err := json.Marshal(msgMap)
	if err != nil {
		log.Errorf("序列化请求数据失败: %v", err)
		return
	}

	log.Infof("Bark请求数据: %s", string(requestMsg))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestMsg))
	if err != nil {
		log.Errorf("创建Bark请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送Bark通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Info("Bark通知发送成功")
	} else {
		log.Errorf("Bark通知发送失败，状态码: %d", resp.StatusCode)
	}
}

type GotifyRequest struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority,omitempty"`
}

func sendGotify(url, token, title, message string) {
	msg := GotifyRequest{
		Title:    title,
		Message:  message,
		Priority: 9,
	}
	payloadBytes, _ := json.Marshal(msg)
	req, err := http.NewRequest("POST", url+"/message?token="+token, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Errorf("创建Gotify请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送Gotify通知失败: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Info("Gotify通知发送成功")
}

func sendEmail(smtpHost, smtpPort, username, password, from, to, subject, body string) {
	auth := smtp.PlainAuth("", username, password, smtpHost)
	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, msg)
	if err != nil {
		log.Errorf("邮件发送失败: %v", err)
		return
	}
	log.Info("邮件发送成功")
}

// FeishuRequest 飞书机器人请求结构
type FeishuRequest struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

func sendFeishu(webhookURL, title, message string) {
	// 构建飞书消息
	feishuMsg := FeishuRequest{
		MsgType: "text",
		Content: struct {
			Text string `json:"text"`
		}{
			Text: fmt.Sprintf("%s\n%s", title, message),
		},
	}

	payload, err := json.Marshal(feishuMsg)
	if err != nil {
		log.Errorf("序列化飞书请求失败: %v", err)
		return
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Errorf("创建飞书请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送飞书通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Info("飞书通知发送成功")
	} else {
		log.Errorf("飞书通知发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

// DingtalkRequest 钉钉机器人请求结构
type DingtalkRequest struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	At struct {
		IsAtAll bool `json:"isAtAll"`
	} `json:"at"`
}

func sendDingtalk(webhookURL, title, message string) {
	// 构建钉钉消息
	dingtalkMsg := DingtalkRequest{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{
			Content: fmt.Sprintf("%s\n%s", title, message),
		},
		At: struct {
			IsAtAll bool `json:"isAtAll"`
		}{
			IsAtAll: false,
		},
	}

	payload, err := json.Marshal(dingtalkMsg)
	if err != nil {
		log.Errorf("序列化钉钉请求失败: %v", err)
		return
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Errorf("创建钉钉请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送钉钉通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Info("钉钉通知发送成功")
	} else {
		log.Errorf("钉钉通知发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

// TelegramRequest Telegram 发送消息请求结构
type TelegramRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// sendTelegram 发送Telegram消息，支持代理
func sendTelegram(botToken, chatID, message, proxyURL string) {
	// Telegram Bot API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// 构建消息
	tgMsg := TelegramRequest{
		ChatID: chatID,
		Text:   message,
	}

	payload, err := json.Marshal(tgMsg)
	if err != nil {
		log.Errorf("序列化Telegram请求失败: %v", err)
		return
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Errorf("创建Telegram请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建HTTP客户端，支持代理
	client := &http.Client{Timeout: 30 * time.Second}

	// 如果配置了代理，设置代理传输
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			log.Errorf("解析代理URL失败: %v", err)
			return
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
		client.Transport = transport
		log.Infof("使用代理发送Telegram消息: %s", proxyURL)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送Telegram通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Info("Telegram通知发送成功")
	} else {
		log.Errorf("Telegram通知发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

// extractVerificationCode 从内容中提取验证码
func extractVerificationCode(content string) string {
	// 修复后的正则表达式 - 移除不支持的语法
	patterns := []string{
		// 模式1：匹配明确的验证码格式
		`(验证码|校验码|动态码)[：:\s]*[\(（\[【{「]?([0-9\s]{4,7})[」}】\]]?[）\)]?`,
		// 模式2：匹配CODE格式
		`[Cc][Oo][Dd][Ee][：:\s]*[\(（\[【{「]?([0-9A-Za-z]{4,6})[」}】\]]?[）\)]?`,
		// 模式3：匹配括号内的数字
		`[\(（\[【{「]([0-9]{4,6})[」}】\]]?[）\)]`,
		// 模式4：匹配纯数字验证码
		`\b([0-9]{4,6})\b`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			code := strings.TrimSpace(matches[1])
			// 清理空格
			code = strings.ReplaceAll(code, " ", "")
			if code != "" {
				return code
			}
		}
	}

	return ""
}

type PostData map[string]interface{}

func sendQQPush(token, cqq, msg string) {
	log.Infof("发送QQPush通知: token=%s, cqq=%s, msg=%s", token, cqq, msg)

	posturl := fmt.Sprintf("https://wx.scjtqs.com/qq/push/pushMsg?token=%s", token)
	header := make(http.Header)
	header.Set("content-type", "application/json")

	postdata, err := json.Marshal(PostData{
		"qq": cqq,
		"content": []PostData{
			{
				"msgtype": "text",
				"text":    msg,
			},
		},
		"token": token,
	})
	if err != nil {
		log.Errorf("序列化QQPush请求数据失败: %v", err)
		return
	}

	req, err := http.NewRequest("POST", posturl, bytes.NewBuffer(postdata))
	if err != nil {
		log.Errorf("创建QQPush请求失败: %v", err)
		return
	}
	req.Header = header

	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送QQPush通知失败: %v", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("读取QQPush响应失败: %v", err)
		return
	}

	log.Infof("QQPush通知发送成功, 响应: %s", string(body))
}
