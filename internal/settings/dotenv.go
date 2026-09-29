package settings

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

func ReadEnv(filePath string) (map[string]string, error) {
	result := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			result[k] = v
		}
	}
	return result, scanner.Err()
}

func WriteEnv(filePath string, envMap map[string]string) error {
	var keys []string
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("# ActaCron Environment Variables\n")
	for _, k := range keys {
		val := envMap[k]
		if strings.Contains(val, " ") || strings.Contains(val, "#") {
			val = fmt.Sprintf(`"%s"`, val)
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, val))
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}
