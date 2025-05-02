package env

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Manager provides thread-safe access to environment variables and configuration settings
type Manager struct {
	envVars          map[string]string
	mutex            sync.RWMutex
	CarrierEnvConfig // Embed CarrierEnvConfig
}

type CarrierEnvConfig struct {
	ZimURL        *string
	ZimTURL       *string
	ZimToken      *string
	ZimClient     *string
	ZimSecret     *string
	IqaxURL       *string
	IqaxToken     *string
	MscURL        *string
	MaerskP2PURL  *string
	MaerskVSURL   *string
	MaerskCFURL   *string
	MaerskLocURL  *string
	MaerskToken   *string
	MaerskToken2  *string
	MscOauth      *string
	MscAudience   *string
	MscClient     *string
	MscThumbPrint *string
	MscScope      *string
	MscRsa        *string
	HapagURL      *string
	HapagClient   *string
	HapagSecret   *string
	OneURL        *string
	OneTURL       *string
	OneToken      *string
	OneAuth       *string
	CmaURL        *string
	CmaToken      *string
	RedisHost     *string
	RedisPort     *string
	RedisDb       *int
	RedisPrtl     *int
	RedisUser     *string
	RedisPw       *string
	DbUser        *string
	DbPw          *string
	Host          *string
	Port          *int
	ServiceName   *string
}

// NewManager creates a new instance of Manager and loads the configuration automatically
func NewManager() (*Manager, error) {
	manager := &Manager{envVars: make(map[string]string)}
	if err := manager.LoadEnvFile(".env"); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Attempt to load configuration when creating a new Manager instance
	if err := manager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return manager, nil
}

// LoadConfig populates the embedded CarrierEnvConfig fields from environment variables
func (m *Manager) LoadConfig() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	ZimURL := m.MustGet("ZIM_URL")
	ZimTokenURL := m.MustGet("ZIM_TURL")
	ZimToken := m.MustGet("ZIM_TOKEN")
	ZimClient := m.MustGet("ZIM_CLIENT")
	ZimSecret := m.MustGet("ZIM_SECRET")
	MaerskP2PURL := m.MustGet("MAEU_P2P")
	MaerskVSURL := m.MustGet("MAEU_VESSEL_SCHEDULE")
	MaerskCFURL := m.MustGet("MAEU_CUTOFF")
	MaerskLocURL := m.MustGet("MAEU_LOCATION")
	MaerskToken := m.MustGet("MAEU_TOKEN")
	MaerskToken2 := m.MustGet("MAEU_TOKEN2")
	IqaxURL := m.MustGet("IQAX_URL")
	IqaxToken := m.MustGet("IQAX_TOKEN")
	OneURL := m.MustGet("ONEY_URL")
	OneTURL := m.MustGet("ONEY_TURL")
	OneToken := m.MustGet("ONEY_TOKEN")
	OneAuth := m.MustGet("ONEY_AUTH")
	HapagURL := m.MustGet("HLCU_URL")
	HapagClient := m.MustGet("HLCU_CLIENT_ID")
	HapagSecret := m.MustGet("HLCU_CLIENT_SECRET")
	MscURL := m.MustGet("MSCU_URL")
	MscOauth := m.MustGet("MSCU_OAUTH")
	MscAudience := m.MustGet("MSCU_AUD")
	MscClient := m.MustGet("MSCU_CLIENT")
	MscThumbPrint := m.MustGet("MSCU_THUMBPRINT")
	MscScope := m.MustGet("MSCU_SCOPE")
	MscRsa := m.MustGet("MSCU_RSA_KEY")
	CmaURL := m.MustGet("CMA_URL")
	CmaToken := m.MustGet("CMA_TOKEN")
	RedisHost := m.MustGet("REDIS_HOST")
	RedisPort := m.MustGet("REDIS_PORT")
	RedisUser := m.MustGet("REDIS_USER")
	RedisPw := m.MustGet("REDIS_PW")
	redisDB, _ := strconv.Atoi(m.MustGet("REDIS_DB"))
	redisPrtl, _ := strconv.Atoi(m.MustGet("REDIS_PROTOCOL"))
	DbUser := m.MustGet("DB_USER")
	DbPw := m.MustGet("DB_PW")
	Host := m.MustGet("HOST")
	Port, _ := strconv.Atoi(m.MustGet("PORT"))
	ServiceName := m.MustGet("SERVICE_NAME")
	// Populate the embedded CarrierEnvConfig fields directly
	m.CarrierEnvConfig = CarrierEnvConfig{
		ZimURL:        &ZimURL,
		ZimTURL:       &ZimTokenURL,
		ZimToken:      &ZimToken,
		ZimClient:     &ZimClient,
		ZimSecret:     &ZimSecret,
		MaerskP2PURL:  &MaerskP2PURL,
		MaerskVSURL:   &MaerskVSURL,
		MaerskCFURL:   &MaerskCFURL,
		MaerskLocURL:  &MaerskLocURL,
		MaerskToken:   &MaerskToken,
		MaerskToken2:  &MaerskToken2,
		IqaxURL:       &IqaxURL,
		IqaxToken:     &IqaxToken,
		CmaURL:        &CmaURL,
		CmaToken:      &CmaToken,
		OneURL:        &OneURL,
		OneTURL:       &OneTURL,
		OneToken:      &OneToken,
		OneAuth:       &OneAuth,
		HapagURL:      &HapagURL,
		HapagClient:   &HapagClient,
		HapagSecret:   &HapagSecret,
		RedisHost:     &RedisHost,
		RedisPort:     &RedisPort,
		RedisDb:       &redisDB,
		RedisPrtl:     &redisPrtl,
		RedisUser:     &RedisUser,
		RedisPw:       &RedisPw,
		MscURL:        &MscURL,
		MscOauth:      &MscOauth,
		MscAudience:   &MscAudience,
		MscClient:     &MscClient,
		MscThumbPrint: &MscThumbPrint,
		MscScope:      &MscScope,
		MscRsa:        &MscRsa,
		DbUser:        &DbUser,
		DbPw:          &DbPw,
		Host:          &Host,
		Port:          &Port,
		ServiceName:   &ServiceName,
	}

	return nil
}

// LoadEnvFile loads environment variables from a file
func (m *Manager) LoadEnvFile(filePath string) error {
	if err := validateFilePath(filePath); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open .env file: %w", err)
	}
	defer file.Close()

	tempVars := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := m.processLine(scanner.Text(), tempVars); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	m.mutex.Lock()
	m.envVars = tempVars
	m.mutex.Unlock()
	return nil
}

// Get retrieves a value from the environment variables
func (m *Manager) Get(key string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	value, exists := m.envVars[key]
	return value, exists
}

// MustGet retrieves a value and panics if it doesn't exist
func (m *Manager) MustGet(key string) string {
	value, exists := m.Get(key)
	if !exists {
		panic(fmt.Sprintf("required environment variable %s not found", key))
	}
	return value
}

func (m *Manager) processLine(line string, tempVars map[string]string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format for line: %s", line)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if err := validateKeyValue(key, value); err != nil {
		return fmt.Errorf("invalid key-value pair: %w", err)
	}

	tempVars[key] = value
	return nil
}
