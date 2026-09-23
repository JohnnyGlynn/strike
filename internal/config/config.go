package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Ports a server binds when the config doesn't say otherwise. The k8s
// manifests, the Dockerfile's EXPOSE and the README all assume these.
const (
	DefaultClientPort     = 8080
	DefaultFederationPort = 9090
)

type ServerConfig struct {
	Name                  string `json:"name" yaml:"name"`
	SigningPrivateKeyPath string `json:"private_server_signing_key_path" yaml:"private_server_singing_key_path"`
	SigningPublicKeyPath  string `json:"public_server_signing_key_path" yaml:"public_server_signing_key_path"`
	CertificatePath       string `json:"certificate_path" yaml:"certificate_path"`
	FederationCAPath      string `json:"federation_ca_path" yaml:"federation_ca_path"`
	FederationPeers       string `json:"federation_peers" yaml:"federation_peers"`
	IdentityFile          string `json:"id_file" yaml:"id_file"`
	DBConnectionString    string `json:"db_connection_string" yaml:"db_connection_string"`

	// Optional — left unset they fall back to the defaults above.
	ClientPort     int `json:"client_port" yaml:"client_port"`
	FederationPort int `json:"federation_port" yaml:"federation_port"`
}

type ClientConfig struct {
	ServerHost               string `json:"server_host" yaml:"server_host"`
	SigningPrivateKeyPath    string `json:"private_signing_key_path" yaml:"private_singing_key_path"`
	SigningPublicKeyPath     string `json:"public_signing_key_path" yaml:"public_signing_key_path"`
	EncryptionPrivateKeyPath string `json:"private_encryption_key_path" yaml:"private_encryption_key_path"`
	EncryptionPublicKeyPath  string `json:"public_encryption_key_path" yaml:"public_encryption_key_path"`
	ServerCertificatePath    string `json:"server_certificate_path" yaml:"server_certificate_key_path"`
}

func LoadServerConfigEnv() (*ServerConfig, error) {
	clientPort, err := portFromEnv("CLIENT_PORT")
	if err != nil {
		return nil, err
	}

	federationPort, err := portFromEnv("FEDERATION_PORT")
	if err != nil {
		return nil, err
	}

	return &ServerConfig{
		Name:                  os.Getenv("SERVER_NAME"),
		SigningPrivateKeyPath: os.Getenv("PRIVATE_SERVER_SIGNING_KEY_PATH"),
		SigningPublicKeyPath:  os.Getenv("PUBLIC_SERVER_SIGNING_KEY_PATH"),
		CertificatePath:       os.Getenv("CERT_PATH"),
		FederationCAPath:      os.Getenv("FED_CA_PATH"),
		FederationPeers:       os.Getenv("FEDERATION_PEERS"),
		IdentityFile:          os.Getenv("IDENTITY_FILE"),
		DBConnectionString:    os.Getenv("DB_CONNECTION_STRING"),
		ClientPort:            clientPort,
		FederationPort:        federationPort,
	}, nil
}

// portFromEnv returns 0 for an unset variable so validatePorts can apply the
// default. A variable that's set but unparseable is an error rather than a
// silent fallback — quietly binding 8080 because of a typo is the failure mode
// this whole change exists to remove.
func portFromEnv(key string) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return 0, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, raw)
	}

	return port, nil
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
			*value = expandTilde(homeDir, *value)
		}
	}
	return nil
}

func expandTilde(homeDir, value string) string {
	if strings.HasPrefix(value, "~") {
		return filepath.Join(homeDir, value[1:])
	}
	return value
}

// expandOptionalPath expands a leading "~" in a path field that's allowed to
// be blank. Federation trust is pinned-key by default (see PeerConfig.PubKey
// and LoadFederationTLSConfig) — a server doesn't need a CA at all unless it
// specifically wants to also trust peers via a shared CA chain.
func expandOptionalPath(value *string) error {
	if *value == "" {
		return nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding user home directory: %w", err)
	}
	*value = expandTilde(homeDir, *value)
	return nil
}

// Ports below 1024 are privileged — binding one needs root or
// CAP_NET_BIND_SERVICE, which a local `make server-run` doesn't have. Rejecting
// them here turns a confusing "permission denied" at listen time into a config
// error that names the field.
const (
	MinPort = 1024
	MaxPort = 65535
)

// validatePorts fills in the defaults for ports left unset, then rejects
// anything a listener couldn't bind. Both ports on one number would start a
// server that's silently missing half its surface, so that's an error too.
func (c *ServerConfig) validatePorts(clientKey, federationKey string) error {
	if c.ClientPort == 0 {
		c.ClientPort = DefaultClientPort
	}

	if c.FederationPort == 0 {
		c.FederationPort = DefaultFederationPort
	}

	for key, port := range map[string]int{clientKey: c.ClientPort, federationKey: c.FederationPort} {
		if port < MinPort || port > MaxPort {
			return fmt.Errorf("%s: %d is out of range — use %d-%d (below %d is privileged)", key, port, MinPort, MaxPort, MinPort)
		}
	}

	if c.ClientPort == c.FederationPort {
		return fmt.Errorf("%s and %s must differ (both are %d)", clientKey, federationKey, c.ClientPort)
	}

	return nil
}

func (c *ServerConfig) ValidateConfig() error {
	if err := ValidateFields(map[string]*string{
		"name":                            &c.Name,
		"private_server_signing_key_path": &c.SigningPrivateKeyPath,
		"public_server_signing_key_path":  &c.SigningPublicKeyPath,
		"certificate_path":                &c.CertificatePath,
		"federation_peers":                &c.FederationPeers,
		"id_file":                         &c.IdentityFile,
		"db_connection_string":            &c.DBConnectionString,
	}); err != nil {
		return err
	}

	if err := c.validatePorts("client_port", "federation_port"); err != nil {
		return err
	}

	return expandOptionalPath(&c.FederationCAPath)
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
	if err := ValidateFields(map[string]*string{
		"SERVER_NAME":                     &c.Name,
		"PRIVATE_SERVER_SIGNING_KEY_PATH": &c.SigningPrivateKeyPath,
		"PUBLIC_SERVER_SIGNING_KEY_PATH":  &c.SigningPublicKeyPath,
		"CERT_PATH":                       &c.CertificatePath,
		"FEDERATION_PEERS":                &c.FederationPeers,
		"IDENTITY_FILE":                   &c.IdentityFile,
		"DB_CONNECTION_STRING":            &c.DBConnectionString,
	}); err != nil {
		return err
	}

	if err := c.validatePorts("CLIENT_PORT", "FEDERATION_PORT"); err != nil {
		return err
	}

	return expandOptionalPath(&c.FederationCAPath)
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
