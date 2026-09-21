package config

import "testing"

func TestProfilesHaveIndependentSettings(t *testing.T) {
	for _, setting := range []struct {
		name   string
		shared bool
	}{
		{name: "HTTP", shared: debugAppConfig.HTTP == releaseAppConfig.HTTP},
		{name: "DB", shared: debugAppConfig.DB == releaseAppConfig.DB},
		{name: "Log", shared: debugAppConfig.Log == releaseAppConfig.Log},
		{name: "Cache", shared: debugAppConfig.Cache == releaseAppConfig.Cache},
		{name: "EasyRoutine", shared: debugAppConfig.EasyRoutine == releaseAppConfig.EasyRoutine},
	} {
		t.Run(setting.name, func(t *testing.T) {
			if setting.shared {
				t.Errorf("debug and release profiles share %s settings", setting.name)
			}
		})
	}
}

func TestDebugProfileKeepsDefaults(t *testing.T) {
	if debugAppConfig.Profile != ProfileDebug {
		t.Errorf("profile = %q, want %q", debugAppConfig.Profile, ProfileDebug)
	}
	wantHTTP := HTTPConfig{
		HTTPSPort:        8443,
		HTTPSCertificate: PEM_STR,
		HTTPSKey:         KEY_STR,
	}
	if *debugAppConfig.HTTP != wantHTTP {
		t.Error("debug HTTP settings differ from defaults")
	}
	wantDB := DBConfig{Host: "127.0.0.1", Port: 3306}
	if *debugAppConfig.DB != wantDB {
		t.Error("debug database settings differ from defaults")
	}
	wantLog := LogConfig{Directory: "logs", ToTerminal: true, ShowLogTail: 10}
	if *debugAppConfig.Log != wantLog {
		t.Error("debug logging settings differ from defaults")
	}
	if *debugAppConfig.Cache != (LocalCacheConfig{MaxTTLSeconds: 24 * 60 * 60}) {
		t.Error("debug cache settings differ from defaults")
	}
	if *debugAppConfig.EasyRoutine != (EasyRoutineConfig{Enabled: true}) {
		t.Error("debug routine settings differ from defaults")
	}
}
