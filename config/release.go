package config

var releaseAppConfig = func() AppConfig {
	appConfig := default_AppConfig
	appConfig.Profile = ProfileRelease
	appConfig.Log.ToTerminal = true
	appConfig.Log.AddSource = false
	appConfig.Log.Level = "debug"

	// Database configuration for release environment
	appConfig.DB.Enabled = true
	appConfig.DB.DBName = "lzapp_release"
	appConfig.DB.User = "lzapp"
	appConfig.DB.Password = "lzapp-release-password"
	appConfig.DB.Charset = "utf8mb4"

	// HTTP configuration for release environment
	appConfig.HTTP.Enabled = true
	appConfig.HTTP.HTTPSPort = 443

	appConfig.EasyRoutine.Enabled = true

	return appConfig
}()
