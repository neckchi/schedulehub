package interfaces

import (
	//"encoding/json"
	"context"
	"encoding/json"
	httpclient "schedulehub/internal/http"
	env "schedulehub/internal/secret"
	"time"
)

type Location interface {
	GetLocationDetails(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, p string) ([]map[string]any, error)
}

type LocationProvider interface {
	LocationHeaderParams(e *env.Manager, portType string) HeaderParams
}

type LocationService struct {
	LocationUrl    string
	Method         string
	Secrets        LocationProvider
	LocationExpiry time.Duration
	Namespace      string
}

//
//type LocationService struct {
//	LocationConfig
//}

func (ls *LocationService) GetLocationDetails(ctx context.Context, c *httpclient.HttpClient, e *env.Manager, p string) ([]map[string]any, error) {
	headerParams := ls.Secrets.LocationHeaderParams(e, p)
	responseJson, err := c.Fetch(ctx, ls.Method, &ls.LocationUrl, &headerParams.Params, &headerParams.Headers, ls.Namespace, ls.LocationExpiry)
	if err != nil {
		return nil, err
	}
	var locationResponse []map[string]any
	if err := json.Unmarshal(responseJson, &locationResponse); err != nil {
		return nil, err
	}
	return locationResponse, nil
}
