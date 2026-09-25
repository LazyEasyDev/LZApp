package config

var releaseAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileRelease

	//
	appConfig.Log.ToTerminal = true
	appConfig.Log.AddSource = false

	// Database configuration for release environment
	appConfig.DB.DBName = "lzapp_release"
	appConfig.DB.User = "lzapp"
	appConfig.DB.Password = "lzapp-release-password"
	appConfig.DB.Charset = "utf8mb4"

	// HTTP configuration for release environment
	appConfig.HTTP.HTTPSPort = 443

	//remove below in released version ,they are here just for convinence
	appConfig.Log.Level = "debug"
	appConfig.Log.DirectoryRelative = "cwd"

	return appConfig
}()
