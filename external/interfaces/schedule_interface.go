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

// Define a type constraint for the return type
type ScheduleOutputType interface {
	[]*schema.P2PSchedule | *schema.MasterVesselSchedule
}

// Might have different way of fetching schedule
type Schedule[T ScheduleOutputType, Q any] interface {
	FetchSchedule(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, q Q, scac schema.CarrierCode) (T, error)
}

type ScheduleArgs[Q any] struct {
	Token       *TokenResponse
	Scac        schema.CarrierCode
	Env         *env.Manager
	Query       Q
	Origin      []map[string]any //json file
	Destination []map[string]any
}

// Each carrier has different struct to manage the heaader params and schedule generation so we built interface to ignore the underlying struct
// as long as  the struct has the function mentioned in the below interface, the fn will be working out.
type ScheduleProvider[T ScheduleOutputType, Q any] interface {
	ScheduleHeaderParams(*ScheduleArgs[Q]) HeaderParams
	GenerateSchedule(responseJson []byte) (T, error)
}

type ScheduleConfig struct {
	ScheduleURL    string
	Method         string
	ScheduleExpiry time.Duration
	Namespace      string
}

type ScheduleService[T ScheduleOutputType, Q any] struct {
	Token
	Location
	ScheduleConfig
	ScheduleProvider[T, Q]
}

func (ss *ScheduleService[T, Q]) FetchSchedule(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, querySchema Q, scac schema.CarrierCode) (T, error) {
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
		arguments := &ScheduleArgs[Q]{Token: token, Env: e, Query: querySchema}
		headerParams = ss.ScheduleProvider.ScheduleHeaderParams(arguments)

	case func() bool {
		location, _ := ss.Location.(*LocationService)
		return location != nil
	}():
		if queryLocation, ok := any(querySchema).(*schema.QueryParams); ok {
			pol, _ := ss.Location.GetLocationDetails(ctx, c, e, queryLocation.PointFrom)
			pod, _ := ss.Location.GetLocationDetails(ctx, c, e, queryLocation.PointTo)
			if pod == nil || pol == nil {
				log.Info("Either POL or POD is unavailable ")
				break
			} else {
				arguments := &ScheduleArgs[Q]{Scac: scac, Env: e, Query: querySchema, Origin: pol, Destination: pod}
				headerParams = ss.ScheduleProvider.ScheduleHeaderParams(arguments)
			}
		}
	default:
		arguments := &ScheduleArgs[Q]{Scac: scac, Env: e, Query: querySchema}
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
