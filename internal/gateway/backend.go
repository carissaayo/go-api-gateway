package gateway

import "github.com/carissaayo/go-api-gateway/internal/circuitbreaker"

func (gw *Gateway) RegisterBackend(name string, url string, weight int) error {
	return gw.proxy.AddBackend(name, url, weight, circuitbreaker.DefaultConfig())
}
