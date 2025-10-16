package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/heltonmarx/goami/ami"
	"io"
	"log"
	"os/exec"
	"time"
)

// SendSMSDirect connects to AMI and executes the 'quectel sms' command.
func SendSMSDirect(amiConfig *AMIConfig, device, recipient, message string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	amiHost := fmt.Sprintf("%s:%s", amiConfig.Host, amiConfig.Port)

	log.Printf("Connecting to AMI at %s", amiHost)
	socket, err := ami.NewSocket(ctx, amiHost)
	if err != nil {
		return "", fmt.Errorf("AMI connection failed: %w", err)
	}
	defer socket.Close(ctx)
	uuid, err := ami.GetUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID for AMI action: %w", err)
	}
	err = ami.Login(ctx, socket, amiConfig.Username, amiConfig.Secret, "Off", uuid)
	if err != nil {
		return "", fmt.Errorf("AMI login failed: %w", err)
	}
	log.Println("AMI login successful")
	defer ami.Logoff(ctx, socket, uuid)

	// Construct the exact CLI command
	cliCommand := fmt.Sprintf("quectel sms %s %s \"%s\"", device, recipient, message)

	log.Printf("Sending AMI Command: %s", cliCommand)
	response, err := ami.Command(ctx, socket, uuid, cliCommand)
	if err != nil {
		return "", fmt.Errorf("AMI command execution failed: %w", err)
	}

	// The useful response from a command is usually in the 'Output' field
	fullResponse := fmt.Sprintf("%+v", response)
	log.Printf("Received AMI response: %s", fullResponse)

	return fullResponse, nil
}

// SendSMSOriginate connects to AMI and executes a dialplan to send SMS with special characters.
func SendSMSOriginate(amiConfig *AMIConfig, device, recipient, message string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	amiHost := fmt.Sprintf("%s:%s", amiConfig.Host, amiConfig.Port)

	log.Printf("Connecting to AMI at %s", amiHost)
	socket, err := ami.NewSocket(ctx, amiHost)
	if err != nil {
		return "", fmt.Errorf("AMI connection failed: %w", err)
	}
	defer socket.Close(ctx)

	uuid, err := ami.GetUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID for AMI action: %w", err)
	}

	// 登录 AMI
	err = ami.Login(ctx, socket, amiConfig.Username, amiConfig.Secret, "Off", uuid)
	if err != nil {
		return "", fmt.Errorf("AMI login failed: %w", err)
	}
	log.Println("AMI login successful")
	defer ami.Logoff(ctx, socket, uuid)

	encodedMessage := base64.StdEncoding.EncodeToString([]byte(message))

	// --- CORRECTION 1: Prepare a slice of strings for the 'Variable' field ---
	// The struct expects a []string, where each element is a "key=value" pair.
	vars := []string{
		fmt.Sprintf("RECIPIENT=%s", recipient),
		fmt.Sprintf("DEVICE=%s", device),
		fmt.Sprintf("MSG_B64=%s", encodedMessage),
	}

	// Create an instance of the library's `OriginateData` struct with CORRECT types
	originateData := ami.OriginateData{
		Channel:  "Local/s@sms-from-api",
		Context:  "sms-from-api",
		Exten:    "s",
		Priority: 1,
		Async:    "true",
		Variable: vars,
	}

	//  Call the dedicated ami.Originate function
	originateUUID, err := ami.GetUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID for originate: %w", err)
	}

	log.Println("Calling ami.Originate to send SMS...")
	response, err := ami.Originate(ctx, socket, originateUUID, originateData)
	if err != nil {
		return "", fmt.Errorf("AMI Originate action failed: %w", err)
	}

	responseString := fmt.Sprintf("%+v", response)
	log.Printf("Received AMI Originate response: %s", responseString)

	if response.Get("Response") != "Success" {
		return responseString, fmt.Errorf("originate action was not successful: %s", response.Get("Message"))
	}

	return responseString, nil
}

// SendSMSShell directly executes the wrapper shell script inside the container.
// This is a much more direct and simpler approach than using AMI Originate.
func SendSMSShell(amiConfig *AMIConfig, device, recipient, message string) (string, error) {
	log.Println("Attempting to send SMS by directly executing shell script...")
	// 创建一个带超时的上下文，防止命令无限期挂起
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// cmd := exec.Command("/usr/local/bin/send_sms_wrapper.sh", device, recipient)
	script := "asterisk -rx \"quectel sms \\\"$1\\\" \\\"$2\\\" \\\"$(cat)\\\"\""
	cmd := exec.CommandContext(ctx, "sh", "-c", script, "_", device, recipient)
	//  获取命令的标准输入管道 (stdin pipe)
	// 这是处理包含换行符等特殊字符消息的关键
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	// 使用 channel 来捕获协程中的写入错误
	errChan := make(chan error, 1)
	//  在一个单独的 goroutine 中向管道写入消息
	// 这样做可以防止因为管道缓冲区满而导致的死锁
	go func() {
		defer stdin.Close()
		_, writeErr := io.WriteString(stdin, message)
		errChan <- writeErr // 将写入结果（无论是否为nil）发送到 channel
	}()

	//  执行命令并等待其完成，同时捕获标准输出和标准错误
	output, execErr := cmd.CombinedOutput()
	writeErr := <-errChan
	if writeErr != nil {
		return string(output), fmt.Errorf("failed to write message to stdin: %w", writeErr)
	}
	// 检查命令执行是否出错（包括超时）
	if execErr != nil {
		return string(output), fmt.Errorf("embedded shell command failed: %w", execErr)
	}
	// 命令成功执行
	log.Printf("Embedded shell command executed successfully. Output: %s", string(output))
	return string(output), nil
}
