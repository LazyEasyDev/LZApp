package config

var releaseAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileRelease
	//
	appConfig.Log.Level = "info"
	appConfig.Log.DirectoryRelative = "app"
	appConfig.DB.LogLevel = "silent"

	//
	appConfig.SecurityHMAC.HMACKey = "lzapp-release-hmac-key"

	//assign you config here
	return appConfig
}()
