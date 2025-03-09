package database

import (
	"context"
	"crypto/md5"
	"errors"
	"github.com/google/uuid"
	goRedis "github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"time"
)

type RedisRepository interface {
	Get(namespace, key string) ([]byte, bool)
	AddToChannel(namespace, key string, value []byte, expiry time.Duration)
	Set(watchKey string) error
}

type RedisSettings struct {
	DB         *int
	DBUser     *string
	DBPassword *string
	Host       *string
	Port       *string
	Protocol   *int
}

type RedisConnection struct {
	client *goRedis.Client
	ctx    context.Context
	ch     chan RedisCache
}

const (
	maxRetries = 2
	poolSize   = 30
)

type RedisCache struct {
	cacheType  string
	cacheKey   string
	cacheValue []byte
	expiry     time.Duration
}

// Constructor to create an instance of redis respository with connection pool setup
func NewRedisConnection(settings RedisSettings) *RedisConnection {
	ctx := context.Background()
	redisClient := goRedis.NewClient(&goRedis.Options{
		Addr: *settings.Host + ":" + *settings.Port,
		DB:   *settings.DB,
		//Protocol: *settings.Protocol,
		//Username: *settings.DBUser,
		//Password: *settings.DBPassword,
		PoolSize: poolSize,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Error(err)
	} else {
		log.Infof("Connected to Redis - %s", redisClient)
	}
	return &RedisConnection{
		client: redisClient,
		ctx:    context.Background(),
		ch:     make(chan RedisCache, 50),
	}
}

func GenerateUUIDFromString(namespace, key string) string {
	hash := md5.Sum([]byte(namespace))
	namespaceUUID := uuid.Must(uuid.FromBytes(hash[:]))
	generatedUUID := uuid.NewMD5(namespaceUUID, []byte(key))
	return generatedUUID.String()
}

func (r *RedisConnection) AddToChannel(namespace, key string, value []byte, expiry time.Duration) {
	r.ch <- RedisCache{cacheType: namespace, cacheKey: GenerateUUIDFromString(namespace, key),
		cacheValue: value, expiry: expiry}
}

func (r *RedisConnection) Set(watchKey string) error {
	close(r.ch)
	txp := func(tx *goRedis.Tx) error {
		_, err := tx.TxPipelined(r.ctx, func(pipe goRedis.Pipeliner) error {
			for data := range r.ch {
				setRes := pipe.SetNX(r.ctx, data.cacheKey, data.cacheValue, data.expiry)
				if err := setRes.Err(); err != nil {
					log.Errorf("Error caching %s: %v", data.cacheKey, err)
				} else {
					log.Infof("Background Task:Successfully cached %s for %v", data.cacheKey, data.cacheType)
				}
			}

			return nil
		})
		if err != nil {
			log.Errorf("error in pipeline %v", err.Error())
			return err
		}
		return nil
	}

	for i := 0; i < maxRetries; i++ {
		err := r.client.Watch(r.ctx, txp, GenerateUUIDFromString("watchKey", watchKey))
		if err == nil {
			// Success
			//Reinitialize channel so we can keep adding items later using the same repository
			r.ch = make(chan RedisCache, 50)
			return nil
		}
		if err == goRedis.TxFailedErr {
			continue
		}
		// Return any other error
		return err
	}
	return errors.New("increment reached maximum number of retries")
}

func (r *RedisConnection) Get(namespace, key string) ([]byte, bool) {
	hashKey := GenerateUUIDFromString(namespace, key)

	// Get cache from Redis
	storedValue, err := r.client.Get(r.ctx, hashKey).Bytes()
	if err == goRedis.Nil {
		log.Infof("Background Task: %s with key: %s does not exist", namespace, hashKey)
		return nil, false
	} else if err != nil {
		log.Errorf("error getting value %v", err.Error())
		return nil, false
	}
	log.Printf("Background Task: %s with key: %s exist", namespace, hashKey)
	return storedValue, true
}
