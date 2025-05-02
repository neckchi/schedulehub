package dependencies

import (
	"github.com/neckchi/schedulehub/external/carrier_p2p_schedule"
	"github.com/neckchi/schedulehub/external/carrier_vessel_schedule"
	"github.com/neckchi/schedulehub/internal/database"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	env "github.com/neckchi/schedulehub/internal/secret"
	"sync"
	"time"
)

// all dependencies required by this app
type Dependencies struct {
	HTTPClient *httpclient.HttpClient
	EnvManager *env.Manager
	VesselSvc  *carrier_vessel_schedule.VesselScheduleServiceFactory
	P2PSvc     *carrier_p2p_schedule.P2PScheduleServiceFactory
	OracleDB   database.OracleRepository
	RedisDB    database.RedisRepository
}

// dependenciesInstance holds the singleton instance of Dependencies.
var (
	dependenciesInstance *Dependencies
	once                 sync.Once
	initErr              error
)

// NewDependencies initializes dependencies only once and returns the same instance on subsequent calls.
func NewDependencies() (*Dependencies, error) {
	once.Do(func() {
		// Initialize environment manager
		envManager, err := env.NewManager()
		if err != nil {
			initErr = err
			return
		}

		// Initialize Redis connection
		redisSettings := database.RedisSettings{
			DB:         envManager.RedisDb,
			DBUser:     envManager.RedisUser,
			DBPassword: envManager.RedisPw,
			Host:       envManager.RedisHost,
			Port:       envManager.RedisPort,
			Protocol:   envManager.RedisPrtl,
		}
		redis, err := database.NewRedisConnection(redisSettings)
		if err != nil {
			initErr = err
			return
		}

		// Initialize external services
		externalMVSApiConfig := carrier_vessel_schedule.NewVesselScheduleServiceFactory(envManager)
		externalP2PApiConfig := carrier_p2p_schedule.NewP2PScheduleServiceFactory(envManager)

		// Initialize Oracle database connection
		oracleSetting := database.OracleSettings{
			DBUser:      envManager.DbUser,
			DBPassword:  envManager.DbPw,
			Host:        envManager.Host,
			Port:        envManager.Port,
			ServiceName: envManager.ServiceName,
		}
		oracle, err := database.NewOracleDBConnectionPool(oracleSetting, 100, 3)
		if err != nil {
			initErr = err
			return
		}

		// Initialize HTTP client
		httpClient := httpclient.CreateHttpClientInstance(
			redis,
			httpclient.WithCtxTimeout(7*time.Second),
			httpclient.WithMaxRetries(2),
			httpclient.WithRetryDelay(2*time.Second),
			httpclient.WithMaxIdleConns(200),
			httpclient.WithMaxConnsPerHost(200),
			httpclient.WithMaxIdleConnsPerHost(200),
			httpclient.WithIdleConnTimeout(90),
			httpclient.WithDisableKeepAlives(false),
		)

		// Set the singleton instance
		dependenciesInstance = &Dependencies{
			HTTPClient: httpClient,
			EnvManager: envManager,
			VesselSvc:  externalMVSApiConfig,
			P2PSvc:     externalP2PApiConfig,
			OracleDB:   oracle,
			RedisDB:    redis,
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	return dependenciesInstance, nil
}
