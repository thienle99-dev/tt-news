package main

import (
	"path/filepath"
	"testing"
)

func TestSavedAIConfigOverridesEnvAndCanReset(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	envConfig := config{AIURL: "https://env.example", AIKey: "env-key", AIModel: "env-model"}
	settings, source, err := loadAISettings(db, envConfig)
	if err != nil {
		t.Fatal(err)
	}
	if source != "env" || settings.Model != "env-model" {
		t.Fatalf("unexpected env settings: source=%q settings=%#v", source, settings)
	}
	if _, err = db.Exec("INSERT INTO ai_config(id,base_url,api_key,model) VALUES(1,'https://saved.example','saved-key','saved-model')"); err != nil {
		t.Fatal(err)
	}
	settings, source, err = loadAISettings(db, envConfig)
	if err != nil {
		t.Fatal(err)
	}
	if source != "saved" || settings.BaseURL != "https://saved.example" || settings.APIKey != "saved-key" || settings.Model != "saved-model" {
		t.Fatalf("unexpected saved settings: source=%q settings=%#v", source, settings)
	}
}
