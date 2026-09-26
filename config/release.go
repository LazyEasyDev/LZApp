package config

var releaseAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileRelease

	// Logging configuration for release environment
	appConfig.Log.ToTerminal = true
	appConfig.Log.AddSource = false
	appConfig.Log.Level = "info"

	//
	appConfig.Security.HMACKey = "lzapp-release-hmac-key"

	// Database configuration for release environment
	appConfig.DB.DBName = "lzapp_release"
	appConfig.DB.User = "lzapp"
	appConfig.DB.Password = "lzapp-release-password"
	appConfig.DB.Charset = "utf8mb4"

	// Redis configuration for release environment

	// HTTP configuration for release environment
	appConfig.HTTP.HTTPSPort = 443

	//remove below in released version ,they are here just for convinence
	appConfig.Log.Level = "debug"
	appConfig.Log.DirectoryRelative = "cwd"

	return appConfig
}()
