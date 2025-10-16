package main

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// AMIConfig holds the necessary credentials for connecting to Asterisk's AMI.
type AMIConfig struct {
	Host     string
	Port     string
	Username string
	Secret   string
}

// GetAMIConfigFromDB queries the FreePBX database to get AMI manager credentials.
func GetAMIConfigFromDB(db *sql.DB) (*AMIConfig, error) {
	log.Println("Querying database for AMI credentials...")

	// The AMI host is the FreePBX container itself.
	// The default AMI port is 5038.
	config := &AMIConfig{
		Host: "127.0.0.1", // Docker service name for the FreePBX container
		Port: "5038",
	}

	rows, err := db.Query("SELECT keyword, value FROM freepbx_settings WHERE keyword IN ('AMPMGRUSER', 'AMPMGRPASS')")
	if err != nil {
		return nil, fmt.Errorf("failed to query freepbx_settings: %w", err)
	}
	defer rows.Close()

	foundUser := false
	foundPass := false

	for rows.Next() {
		var keyword, value string
		if err := rows.Scan(&keyword, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if keyword == "AMPMGRUSER" {
			config.Username = value
			foundUser = true
		} else if keyword == "AMPMGRPASS" {
			config.Secret = value
			foundPass = true
		}
	}

	if !foundUser || !foundPass {
		return nil, fmt.Errorf("could not find AMPMGRUSER or AMPMGRPASS in the freepbx_settings table")
	}

	log.Printf("Successfully fetched AMI credentials for user: %s", config.Username)
	return config, nil
}

func initConfig() error {
	// 读取发送配置文件
	viperconfig = viper.New()
	viperconfig.SetConfigName("forward")
	viperconfig.SetConfigType("yaml")
	viperconfig.AddConfigPath("/data/config")
	if err := viperconfig.ReadInConfig(); err != nil {
		return fmt.Errorf("读取推送配置失败: %v", err)
	}
	if err := viperconfig.Unmarshal(&config); err != nil {
		return fmt.Errorf("解析推送配置失败: %v", err)
	}
	log.Info("读取推送配置完成")
	return nil
}
