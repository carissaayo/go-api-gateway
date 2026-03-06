package gateway

func (gw *Gateway) RegisterBackend(name string, url string, weight int) error {
	return gw.proxy.AddBackend(name, url, weight)
}
