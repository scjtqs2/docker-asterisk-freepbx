package main

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
)

// AMIConfig holds the necessary credentials for connecting to Asterisk's AMI.
type AMIConfig struct {
	Host     string
	Port     string
	Username string
	Secret   string
}

// DBConfig holds the database connection details.
type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	DBName   string
}

// GetAMIConfigFromDB queries the FreePBX database to get AMI manager credentials.
func GetAMIConfigFromDB(db *sql.DB) (*AMIConfig, error) {
	log.Println("Querying database for AMI credentials...")

	// The AMI host is the FreePBX container itself.
	// The default AMI port is 5038.
	amiConfig := &AMIConfig{
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
			amiConfig.Username = value
			foundUser = true
		} else if keyword == "AMPMGRPASS" {
			amiConfig.Secret = value
			foundPass = true
		} else if keyword == "ASTMANAGERHOST" {
			amiConfig.Host = value
		} else if keyword == "ASTMANAGERPORT" {
			amiConfig.Port = value
		}
	}

	if !foundUser || !foundPass {
		return nil, fmt.Errorf("could not find AMPMGRUSER or AMPMGRPASS in the freepbx_settings table")
	}

	log.Printf("Successfully fetched AMI credentials for user: %s", amiConfig.Username)
	return amiConfig, nil
}

func initConfig() (*DBConfig, error) {
	// Read forwarding configuration
	viperconfig = viper.New()
	viperconfig.SetConfigName("forward")
	viperconfig.SetConfigType("yaml")
	viperconfig.AddConfigPath("/data/config")
	if err := viperconfig.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read push configuration: %v", err)
	}
	if err := viperconfig.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to parse push configuration: %v", err)
	}
	log.Info("Push configuration loaded successfully")

	// Read database configuration from environment variables
	dbConfig := &DBConfig{
		Host:     os.Getenv("DBHOST"),
		Port:     os.Getenv("DBPORT"),
		Username: os.Getenv("DBUSER"),
		Password: os.Getenv("DBPASS"),
		DBName:   os.Getenv("DBNAME"),
	}

	return dbConfig, nil
}
