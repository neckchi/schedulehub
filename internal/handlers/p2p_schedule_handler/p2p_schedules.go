package p2p_schedule_handler

import (
	"context"
	"github.com/neckchi/schedulehub/external/carrier_p2p_schedule"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/exceptions"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/middleware"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"github.com/neckchi/schedulehub/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"runtime"
)

func btoMb(b uint64) uint64 {
	return b / 1000 / 1000
}

func PrintMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Infof("Alloc - Bytes in use by the heap: %v MB", btoMb(m.Alloc))
	log.Infof("TotalAlloc(Memory Intensive) - The cumulative total number of bytes allocated in the heap:%v MB", btoMb(m.TotalAlloc))
	log.Infof("Sys  - Total byte of memory obtained from the OS(included heap and non-heap in runtime): %v MB", btoMb(m.Sys))
	log.Infof("NumGC - Total number of completed garage collection cycles:%v", m.NumGC)
	log.Infof("Total goroutines:%v", runtime.NumGoroutine())
}

///If Alloc keeps growing without dropping, you might have a memory leak (objects aren’t being garbage-collected).
///If HeapInuse grows but Alloc stabilizes, memory is being freed but not yet returned to the OS (normal behavior in Go).
///If Sys approaches your EC2 instance’s RAM limit (e.g., 1 GiB for t2.micro), you’re at risk of running out of memory.

type P2PScheduleService struct {
	client *httpclient.HttpClient
	env    *env.Manager
	ps     *carrier_p2p_schedule.P2PScheduleServiceFactory
	redis  database.RedisRepository
}

func NewP2PScheduleService(
	client *httpclient.HttpClient,
	env *env.Manager,
	ps *carrier_p2p_schedule.P2PScheduleServiceFactory,
	redis database.RedisRepository,
) *P2PScheduleService {
	return &P2PScheduleService{client, env, ps, redis}
}

func P2PScheduleHandler(s *P2PScheduleService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fw := utils.NewFlushWriter(w)
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel() // Ensure cancellation when function exits
		queryParams, _ := r.Context().Value(middleware.P2PQueryParamsKey).(schema.QueryParams)
		done := make(chan int) // this is going to ensure that our goroutine are shut down in the event that we call done from the P2PScheduleHandler function
		defer close(done)
		service := NewScheduleStreamingService(ctx, done, s.client, s.env, s.ps, &queryParams)
		fanOutscheduleChannels := service.FanOutScheduleChannels()
		fannedInStream := service.FanIn(fanOutscheduleChannels...)
		service.StreamResponse(fw, fannedInStream)
		go func() {
			err := s.redis.Set(r.URL.String())
			if err != nil {
				exceptions.InternalErrorHandler(w, err)
			}
		}()
	})
}
