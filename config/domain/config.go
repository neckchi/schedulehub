package domain

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"sync"
)

// Config is an abstraction around the map that holds the config values
type Config struct {
	config map[string]interface{}
	lock   sync.RWMutex
}

// SetFromBytes sets the internal config based on YAML
func (c *Config) SetFromBytes(data []byte) error {
	var rawConfig interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return err
	}
	appConfig, ok := rawConfig.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config is not a map")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.config = appConfig
	return nil
}

// Get the config for a particular service
func (c *Config) Get(serviceName string) (map[string]interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	a, ok := c.config["base"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("base config is not a map")
	}

	// If no config is defined for the service
	if _, ok = c.config[serviceName]; !ok {
		// Return the base config
		return a, nil
	}

	b, ok := c.config[serviceName].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("service %q config is not a map", serviceName)
	}

	// Merge the base config with the service config
	config := make(map[string]interface{})
	for k, v := range a {
		config[k] = v
	}
	for k, v := range b {
		config[k] = v
	}

	return config, nil
}
