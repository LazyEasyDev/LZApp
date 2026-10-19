package config

var ReleaseAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileRelease
	//
	appConfig.Log.Level = "info"
	appConfig.Log.BaseDirectoryRelative = "app"
	appConfig.DB.LogLevel = "silent"

	//
	appConfig.SecurityHMAC.HMACKey = "lzapp-release-hmac-key"

	//assign you config here
	return appConfig
}()
