package config

var debugAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileDebug
	//
	appConfig.Log.Level = "debug"
	appConfig.Log.DirectoryRelative = "cwd"

	return appConfig
}()
