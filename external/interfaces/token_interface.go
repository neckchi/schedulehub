package interfaces

import (
	"context"
	"encoding/json"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	env "github.com/neckchi/schedulehub/internal/secret"
	"time"
)

// Abstraction
// Completely decouple!We just need a contract that needs the below function.(GetToken)
// We shouldnt care about the type of struct. it could be anything
// Each carrier has different method to deal with authorization. We shouldnt tightly bind the function to specific type of struct
type Token interface {
	GetOAuthToken(ctx context.Context, client *httpclient.HttpClient, e *env.Manager) (map[string]any, error)
}

type TokenProvider interface {
	TokenHeaderParams(e *env.Manager) HeaderParams
}

type OAuth2 struct {
	TokenUrl    string
	Method      string
	Secrets     TokenProvider
	TokenExpiry time.Duration
	Namespace   string
}

type TokenResponse struct {
	Data map[string]interface{}
}

func (o *OAuth2) GetOAuthToken(ctx context.Context, c *httpclient.HttpClient, e *env.Manager) (map[string]any, error) {
	headerParams := o.Secrets.TokenHeaderParams(e)
	responseJson, err := c.Fetch(ctx, o.Method, &o.TokenUrl, &headerParams.Params, &headerParams.Headers, o.Namespace, o.TokenExpiry)
	if err != nil {
		return nil, err
	}
	var tokenResponse map[string]any
	if err := json.Unmarshal(responseJson, &tokenResponse); err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func GetToken(t Token, ctx context.Context, client *httpclient.HttpClient, e *env.Manager) (map[string]any, error) {
	return t.GetOAuthToken(ctx, client, e)
}
