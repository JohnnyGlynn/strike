package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ServerConfig struct {
	ServerName            string `json:"server_name" yaml:"server_name"`
	SigningPrivateKeyPath string `json:"private_server_signing_key_path" yaml:"private_server_singing_key_path"`
	SigningPublicKeyPath  string `json:"public_server_signing_key_path" yaml:"public_server_signing_key_path"`
	CertificatePath       string `json:"certificate_path" yaml:"certificate_path"`
}

type ClientConfig struct {
	ServerHost               string `json:"server_host" yaml:"server_host"`
	SigningPrivateKeyPath    string `json:"private_signing_key_path" yaml:"private_singing_key_path"`
	SigningPublicKeyPath     string `json:"public_signing_key_path" yaml:"public_signing_key_path"`
	EncryptionPrivateKeyPath string `json:"private_encryption_key_path" yaml:"private_encryption_key_path"`
	EncryptionPublicKeyPath  string `json:"public_encryption_key_path" yaml:"public_encryption_key_path"`
	ServerCertificatePath    string `json:"server_certificate_path" yaml:"server_certificate_key_path"`
}

func LoadServerConfigEnv() *ServerConfig {
	return &ServerConfig{
		ServerName:            os.Getenv("SERVER_NAME"),
		SigningPrivateKeyPath: os.Getenv("PRIVATE_SERVER_SIGNING_KEY_PATH"),
		SigningPublicKeyPath:  os.Getenv("PUBLIC_SERVER_SIGNING_KEY_PATH"),
		CertificatePath:       os.Getenv("CERT_PATH"),
	}
}

func LoadClientConfigEnv() *ClientConfig {
	return &ClientConfig{
		ServerHost:               os.Getenv("SERVER_HOST"),
		SigningPrivateKeyPath:    os.Getenv("PRIVATE_SIGNING_KEY_PATH"),
		SigningPublicKeyPath:     os.Getenv("PUBLIC_SIGNING_KEY_PATH"),
		EncryptionPrivateKeyPath: os.Getenv("PRIVATE_ENCRYPTION_KEY_PATH"),
		EncryptionPublicKeyPath:  os.Getenv("PUBLIC_ENCRYPTION_KEY_PATH"),
		ServerCertificatePath:    os.Getenv("SERVER_CERT_PATH"),
	}
}

// Generic to support either Server or Client config
func LoadConfigFile[cfg any](filePath string) (cfg, error) {

	var c cfg

	configFile, err := os.ReadFile(filePath)
	if err != nil {
		return c, fmt.Errorf("failed to read config file: %v", err)
	}

	if err := json.Unmarshal(configFile, &c); err != nil {
		return c, fmt.Errorf("failed to unmarshall config JSON: %v", err)
	}

	return c, nil
}

// Field validation and Path sanitizing
func ValidateFields(cfg map[string]*string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding user home directory: %w", err)
	}

	for key, value := range cfg {
		if *value == "" {
			return fmt.Errorf("%s is required", key)
		}

		// Only expand paths for keys containing "PATH" or "path"
		if strings.Contains(key, "PATH") || strings.Contains(key, "path") {
			if strings.HasPrefix(*value, "~") {
				*value = filepath.Join(homeDir, (*value)[1:])
			}
		}
	}
	return nil
}

func (c *ServerConfig) ValidateConfig() error {
	return ValidateFields(map[string]*string{
		"server_name":                     &c.ServerName,
		"private_server_signing_key_path": &c.SigningPrivateKeyPath,
		"public_server_signing_key_path":  &c.SigningPublicKeyPath,
		"certificate_path":                &c.CertificatePath,
	})
}

func (c *ClientConfig) ValidateConfig() error {
	return ValidateFields(map[string]*string{
		"server_host":                 &c.ServerHost,
		"private_signing_key_path":    &c.SigningPrivateKeyPath,
		"public_signing_key_path":     &c.SigningPublicKeyPath,
		"private_encryption_key_path": &c.EncryptionPrivateKeyPath,
		"public_encryption_key_path":  &c.EncryptionPublicKeyPath,
		"server_certificate_path":     &c.ServerCertificatePath,
	})
}

func (c *ServerConfig) ValidateEnv() error {
	return ValidateFields(map[string]*string{
		"SERVER_NAME":                     &c.ServerName,
		"PRIVATE_SERVER_SIGNING_KEY_PATH": &c.SigningPrivateKeyPath,
		"PUBLIC_SERVER_SIGNING_KEY_PATH":  &c.SigningPublicKeyPath,
		"CERT_PATH":                       &c.CertificatePath,
	})
}

func (c *ClientConfig) ValidateEnv() error {
	return ValidateFields(map[string]*string{
		"SERVER_HOST":                 &c.ServerHost,
		"PRIVATE_SIGNING_KEY_PATH":    &c.SigningPrivateKeyPath,
		"PUBLIC_SIGNING_KEY_PATH":     &c.SigningPublicKeyPath,
		"PRIVATE_ENCRYPTION_KEY_PATH": &c.EncryptionPrivateKeyPath,
		"PUBLIC_ENCRYPTION_KEY_PATH":  &c.EncryptionPublicKeyPath,
		"SERVER_CERT_PATH":            &c.ServerCertificatePath,
	})
}
