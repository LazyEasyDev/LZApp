package config

var DebugAppConfig = func() AppConfig {
	appConfig := newDefaultConfig()
	appConfig.Profile = ProfileDebug
	//
	appConfig.Log.Level = "debug"
	appConfig.Log.BaseDirectoryRelative = "cwd"
	appConfig.DB.LogLevel = "warn"
	//
	return appConfig
}()
