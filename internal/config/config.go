package config

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	APIEndpoint  string
	PrivateKey   *rsa.PrivateKey
	PublicKeyPEM string

	// Certificate settings
	CACertPath   string
	CertPath     string
	KeyPath      string
	CFSSLApiURL  string
	CommonName   string
	Organization string
}

var logger *log.Logger

func InitLogger(verbose bool) {
	if verbose {
		logger = log.New(os.Stdout, "persys-cli: ", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "", 0)
	}
}

func Logf(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf(format, v...)
	}
}

func GetConfig() Config {
	cfg := Config{
		APIEndpoint: viper.GetString("api_endpoint"),
	}
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = "http://localhost:8084"
		log.Printf("WARNING! No api_endpoint Found in your config.yaml Defaulting to: %v", cfg.APIEndpoint)
	}

	// Certificate settings
	cfg.CACertPath = viper.GetString("ca_cert_path")
	cfg.CertPath = viper.GetString("cert_path")
	cfg.KeyPath = viper.GetString("key_path")
	cfg.CFSSLApiURL = viper.GetString("cfssl_api_url")
	cfg.CommonName = viper.GetString("common_name")
	cfg.Organization = viper.GetString("organization")

	// Set default paths if not specified
	if cfg.CACertPath == "" {
		cfg.CACertPath = filepath.Join(os.Getenv("HOME"), ".persys", "ca.crt")
	}
	if cfg.CertPath == "" {
		cfg.CertPath = filepath.Join(os.Getenv("HOME"), ".persys", "client.crt")
	}
	if cfg.KeyPath == "" {
		cfg.KeyPath = filepath.Join(os.Getenv("HOME"), ".persys", "client.key")
	}

	privKeyStr := viper.GetString("private_key")
	if privKeyStr != "" {
		block, _ := pem.Decode([]byte(privKeyStr))
		if block == nil {
			log.Fatal("Failed to decode private key")
		}
		privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Fatal("Failed to parse private key: ", err)
		}
		cfg.PrivateKey = privKey

		pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
		if err != nil {
			log.Fatal("Failed to marshal public key: ", err)
		}
		pemBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyBytes}
		pemBuf := new(bytes.Buffer)
		err = pem.Encode(pemBuf, pemBlock)
		if err != nil {
			log.Fatal("Failed to encode public key: ", err)
		}
		cfg.PublicKeyPEM = hex.EncodeToString(pemBuf.Bytes())
	}

	return cfg
}
