package handlers

import (
	"context"
	"net/http"
	"schedulehub/external"
	"schedulehub/internal/database"
	httpclient "schedulehub/internal/http"
	"schedulehub/internal/middleware"
	"schedulehub/internal/schema"
	env "schedulehub/internal/secret"
)

type flushWriter struct {
	http.ResponseWriter
}

func (fw flushWriter) Flush() {
	if flusher, ok := fw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func P2PScheduleHandler(client *httpclient.HttpClient, env *env.Manager,
	p2p *external.ScheduleServiceFactory, rr database.RedisRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fw := flushWriter{w}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel() // Ensure cancellation when function exits
		queryParams, _ := r.Context().Value(middleware.P2PQueryParamsKey).(schema.QueryParams)
		done := make(chan int) // this is going to ensure that our goroutine are shut down in the event that we call done from the P2PScheduleHandler function
		defer close(done)
		service := NewScheduleStreamingService(ctx, done, client, env, p2p, &queryParams)
		scheduleChannels := service.GenerateScheduleChannels()
		fannedInStream := service.FanIn(scheduleChannels...)
		service.StreamResponse(fw, r, fannedInStream)
		go rr.Set(r.URL.String())
		//finalSchedule, err := validateSchedules(ctx, fannedInStream, w)
		//if err != nil {
		//	cancel()
		//	return
		//}

		////buildResponse(w, r, rr, queryParams, finalSchedule)
		//buildResponse(w, r, rr, queryParams, fannedInStream)
		//// Cache the result
		//go rr.Set(r.URL.String())

		// Check for context cancellation

	})
}

//func validateSchedules(ctx context.Context, fannedInStream <-chan any, w http.ResponseWriter) ([]*schema.Schedule, error) {
//	var finalSchedule []*schema.Schedule
//	for result := range fannedInStream {
//		select {
//		case <-ctx.Done():
//			log.Println("Validation aborted due to cancellation")
//			return nil, ctx.Err()
//		default:
//			schedules, _ := result.([]*schema.Schedule)
//			for _, schedule := range schedules {
//				if err := schema.ResponseValidate.Struct(schedule); err != nil {
//					if validationErrors, ok := err.(validator.ValidationErrors); ok {
//						log.Errorf("%+v\n", validationErrors.Error())
//						exceptions.ValidationErrorHandler(w, validationErrors)
//						return nil, err
//					}
//				}
//			}
//			finalSchedule = append(finalSchedule, schedules...)
//		}
//	}
//	return finalSchedule, nil
//}

//func buildResponse(w http.ResponseWriter, r *http.Request, rr *database.Repository, queryParams *schema.QueryParams, fannedIn <-chan any) {
//	productID := rr.GenerateUUIDFromString("schedule product", r.URL.String())
//	var response any
//	var finalSchedule []*schema.Schedule
//	for schedules := range fannedIn {
//		finalSchedule = append(finalSchedule, schedules.([]*schema.Schedule)...)
//	}
//
//	if len(finalSchedule) == 0 {
//		response = map[string]any{
//			"productid": productID,
//			"details":   fmt.Sprintf("%s -> %s schedule not found", *queryParams.PointFrom, *queryParams.PointTo),
//		}
//	} else {
//		response = schema.Product{
//			ProductID:   productID,
//			Origin:      *queryParams.PointFrom,
//			Destination: *queryParams.PointTo,
//			//NoOfSchedule: noOfSchedule,
//			Schedules: finalSchedule,
//		}
//	}
//	_ = json.NewEncoder(w).Encode(response)
//}

//func buildResponse(w http.ResponseWriter, r *http.Request, rr *database.Repository, queryParams *schema.QueryParams, finalSchedule []*schema.Schedule) {
//	productID := rr.GenerateUUIDFromString("schedule product", r.URL.String())
//	var response any
//	noOfSchedule := len(finalSchedule)
//	if noOfSchedule == 0 {
//		response = map[string]any{
//			"productid": productID,
//			"details":   fmt.Sprintf("%s -> %s schedule not found", *queryParams.PointFrom, *queryParams.PointTo),
//		}
//	} else {
//		response = schema.Product{
//			ProductID:   productID,
//			Origin:      *queryParams.PointFrom,
//			Destination: *queryParams.PointTo,
//			//NoOfSchedule: noOfSchedule,
//			Schedules: finalSchedule,
//		}
//	}
//	_ = json.NewEncoder(w).Encode(response)
//}
