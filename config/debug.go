package config

var debugAppConfig = func() AppConfig {
	appConfig := default_AppConfig
	appConfig.Profile = ProfileDebug

	return appConfig
}()
