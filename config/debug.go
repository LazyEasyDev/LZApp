package config

var debugAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileDebug

	return appConfig
}()
