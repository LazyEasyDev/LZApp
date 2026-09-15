package config

var debugAppConfig = func() AppConfig {
	appConfig := default_AppConfig
	appConfig.Profile = ProfileDebug

	appConfig.Log.Level = "debug"
	appConfig.Log.Directory = "var/lzapp/debug"
	appConfig.Log.AddSource = true

	appConfig.DB.Enabled = true
	appConfig.DB.DBName = "lzapp_debug"

	appConfig.HTTP.Enabled = true
	appConfig.HTTP.HTTPSPort = 8444

	appConfig.EasyRoutine.Enabled = true

	return appConfig
}()
