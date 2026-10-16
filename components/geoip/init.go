package geoip

import "github.com/LazyEasyDev/GeoIP"

func Init() error {
	return GeoIP.Initialize()
}
