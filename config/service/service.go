package service

import (
	"github.com/neckchi/schedulehub/config/domain"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

type ConfigService struct {
	Config   *domain.Config
	Location string
}

// Watch reloads the config every d duration
func (s *ConfigService) Watch(d time.Duration) {
	for {
		err := s.Reload()
		if err != nil {
			log.Error(err)
		}
		time.Sleep(d)
	}
}

// Reload reads the config and applies changes
func (s *ConfigService) Reload() error {
	data, err := os.ReadFile(s.Location)
	if err != nil {
		return err
	}
	err = s.Config.SetFromBytes(data)
	if err != nil {
		return err
	}
	return nil
}
