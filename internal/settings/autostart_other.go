//go:build !windows

package settings

func SetAutoStart(appName, exePath string, enable bool) error {
	return nil
}

func IsAutoStartEnabled(appName string) bool {
	return false
}
