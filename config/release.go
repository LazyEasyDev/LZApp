package config

var releaseAppConfig = func() AppConfig {
	appConfig := default_AppConfig
	appConfig.Profile = ProfileRelease
	appConfig.Log.Directory = "var/lzapp/release"
	appConfig.Log.ToTerminal = true
	appConfig.Log.AddSource = false

	appConfig.DB.Enabled = true
	appConfig.DB.DBName = "lzapp_release"

	appConfig.HTTP.Enabled = true
	appConfig.HTTP.HTTPSPort = 443

	appConfig.EasyRoutine.Enabled = true

	return appConfig
}()
