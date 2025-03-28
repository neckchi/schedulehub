package interfaces

import (
	"context"
	"fmt"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	log "github.com/sirupsen/logrus"
	"time"
)

type HeaderParams struct {
	Headers map[string]string
	Params  map[string]string
}

// Might have different way of fetching schedule
type Schedule interface {
	FetchSchedule(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, q *schema.QueryParams, scac schema.CarrierCode) ([]*schema.Schedule, error)
}

type ScheduleArgs struct {
	Token       *TokenResponse
	Scac        schema.CarrierCode
	Env         *env.Manager
	Query       *schema.QueryParams
	Origin      []map[string]any //json file
	Destination []map[string]any
}

// Each carrier has different struct to manage the heaader params and schedule
type ScheduleProvider interface {
	ScheduleHeaderParams(*ScheduleArgs) HeaderParams
	GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error)
}

type ScheduleConfig struct {
	ScheduleURL    string
	Method         string
	ScheduleExpiry time.Duration
	Namespace      string
}

type ScheduleService struct {
	Token
	Location
	ScheduleConfig
	ScheduleProvider
}

func (ss *ScheduleService) FetchSchedule(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, q *schema.QueryParams, scac schema.CarrierCode) ([]*schema.Schedule, error) {
	var headerParams HeaderParams

	switch {
	case func() bool {
		tokenProvider, ok := ss.Token.(*OAuth2)
		return ok && tokenProvider != nil
	}():
		tokenData, err := GetToken(ss.Token, ctx, c, e)
		if err != nil {
			return nil, fmt.Errorf("failed to get auth token: %w", err)
		}
		token := &TokenResponse{Data: tokenData}
		arguments := &ScheduleArgs{Token: token, Env: e, Query: q}
		headerParams = ss.ScheduleProvider.ScheduleHeaderParams(arguments)

	case func() bool {
		location, _ := ss.Location.(*LocationService)
		return location != nil
	}():
		pol, _ := ss.Location.GetLocationDetails(ctx, c, e, q.PointFrom)
		pod, _ := ss.Location.GetLocationDetails(ctx, c, e, q.PointTo)
		if pod == nil || pol == nil {
			log.Info("Either POL or POD is unavailable ")
			break
		} else {
			arguments := &ScheduleArgs{Scac: scac, Env: e, Query: q, Origin: pol, Destination: pod}
			headerParams = ss.ScheduleProvider.ScheduleHeaderParams(arguments)
		}
	default:
		arguments := &ScheduleArgs{Scac: scac, Env: e, Query: q}
		headerParams = ss.ScheduleProvider.ScheduleHeaderParams(arguments)
	}
	if headerParams.Headers != nil {
		responseJson, err := c.Fetch(ctx, ss.ScheduleConfig.Method, &ss.ScheduleConfig.ScheduleURL, &headerParams.Params, &headerParams.Headers, ss.ScheduleConfig.Namespace, ss.ScheduleConfig.ScheduleExpiry)
		if err != nil {
			log.Info(err)
			return nil, err
		}
		finalSchedule, err := ss.ScheduleProvider.GenerateSchedule(responseJson)
		if err != nil {
			return nil, err
		}
		return finalSchedule, nil
	}
	return nil, nil
}
