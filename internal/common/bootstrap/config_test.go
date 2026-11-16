package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file"
)

// 使用不同于默认值的配置，防止读取失败后回退默认值掩盖键名错误。
func TestBusinessConfigKeys(t *testing.T) {
	dir := t.TempDir()
	content := "[app]\nname = 'custom-server'\ntoken = 'custom-token'\n[backend]\nbase_url = 'http://backend:9999'\n[mongo]\nuri = 'mongodb://mongo:27018'\ndatabase = 'custom-db'\ncollection = 'custom-players'\n"
	if err := os.WriteFile(filepath.Join(dir, "app.toml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c := config.NewConfigurator(config.WithSources(file.NewSource(file.WithPath(dir))))
	t.Cleanup(c.Close)
	for key, want := range map[string]string{
		"app.app.name": "custom-server", "app.app.token": "custom-token",
		"app.backend.base_url": "http://backend:9999", "app.mongo.uri": "mongodb://mongo:27018",
		"app.mongo.database": "custom-db", "app.mongo.collection": "custom-players",
	} {
		if got := c.Get(key).String(); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
