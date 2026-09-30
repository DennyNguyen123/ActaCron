package parser

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"actacron/internal/domain"
	gojaparser "github.com/dop251/goja/parser"
	"github.com/robfig/cron/v3"
)

var (
	jsdocRegex      = regexp.MustCompile(`/\*\*([\s\S]*?)\*/`)
	nameRegex       = regexp.MustCompile(`@name\s+([a-zA-Z0-9_-]+)`)
	descRegex       = regexp.MustCompile(`@description\s+([^\r\n]+)`)
	cronRegex       = regexp.MustCompile(`@cron\s+([^\r\n]+)`)
	cronStartRegex  = regexp.MustCompile(`@cron_start\s+([^\r\n]+)`)
	cronEndRegex    = regexp.MustCompile(`@cron_end\s+([^\r\n]+)`)
	timezoneRegex   = regexp.MustCompile(`@timezone\s+([^\r\n]+)`)
	retryRegex      = regexp.MustCompile(`@retry\s+([^\r\n]+)`)
	maxRunsRegex    = regexp.MustCompile(`@max_runs\s+([0-9]+)`)
	noOverlapRegex  = regexp.MustCompile(`@no_overlap\s+(true|false)`)
	mcpRegex        = regexp.MustCompile(`@mcp\s+(true|false)`)
	allowExecRegex  = regexp.MustCompile(`@allowExec\s+(true|false)`)
	timeoutRegex    = regexp.MustCompile(`@timeout\s+([0-9]+)`)
	paramRegex      = regexp.MustCompile(`@param\s+\{([^}]+)\}\s+([a-zA-Z0-9_-]+)(?:\s*-\s*([^\r\n]+))?`)
	cronParserValid = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
)

var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
}

func cleanCommentString(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "//"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	if idx := strings.Index(s, "/*"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

func parseValidDateString(raw string) (string, bool) {
	clean := cleanCommentString(raw)
	if clean == "" {
		return "", false
	}
	for _, layout := range dateLayouts {
		if _, err := time.Parse(layout, clean); err == nil {
			return clean, true
		}
	}
	return "", false
}

func Parse(code string, filename string) (*domain.FunctionMeta, error) {
	// 1. AST syntax check
	_, err := gojaparser.ParseFile(nil, filename, code, 0)
	if err != nil {
		return nil, fmt.Errorf("javascript syntax error: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	meta := &domain.FunctionMeta{
		Name:      baseName,
		FilePath:  filename,
		IsEnabled: true,
		UpdatedAt: time.Now(),
		Params:    []domain.ParamSchema{},
	}

	// 2. Extract JSDoc comment
	match := jsdocRegex.FindStringSubmatch(code)
	if len(match) > 1 {
		doc := match[1]

		if nameMatch := nameRegex.FindStringSubmatch(doc); len(nameMatch) > 1 {
			meta.Name = strings.TrimSpace(nameMatch[1])
		}

		if descMatch := descRegex.FindStringSubmatch(doc); len(descMatch) > 1 {
			meta.Description = strings.TrimSpace(descMatch[1])
		}

		if cronMatch := cronRegex.FindStringSubmatch(doc); len(cronMatch) > 1 {
			expr := cleanCommentString(cronMatch[1])
			if expr != "" {
				if _, err := cronParserValid.Parse(expr); err != nil {
					return nil, fmt.Errorf("invalid cron expression '%s': %w", expr, err)
				}
				meta.CronExpr = expr
				meta.NoOverlap = true // default to true if @cron is present
			}
		}

		if noOverlapMatch := noOverlapRegex.FindStringSubmatch(doc); len(noOverlapMatch) > 1 {
			meta.NoOverlap = (noOverlapMatch[1] == "true")
		}

		if startMatch := cronStartRegex.FindStringSubmatch(doc); len(startMatch) > 1 {
			if validStr, ok := parseValidDateString(startMatch[1]); ok {
				meta.CronStart = validStr
			} else {
				log.Printf("[Parser] Warning: invalid @cron_start format '%s' in %s, ignoring boundary", strings.TrimSpace(startMatch[1]), filename)
			}
		}

		if endMatch := cronEndRegex.FindStringSubmatch(doc); len(endMatch) > 1 {
			if validStr, ok := parseValidDateString(endMatch[1]); ok {
				meta.CronEnd = validStr
			} else {
				log.Printf("[Parser] Warning: invalid @cron_end format '%s' in %s, ignoring boundary", strings.TrimSpace(endMatch[1]), filename)
			}
		}

		if tzMatch := timezoneRegex.FindStringSubmatch(doc); len(tzMatch) > 1 {
			tz := cleanCommentString(tzMatch[1])
			if tz != "" {
				meta.Timezone = tz
			}
		}

		if retryMatch := retryRegex.FindStringSubmatch(doc); len(retryMatch) > 1 {
			clean := cleanCommentString(retryMatch[1])
			parts := strings.Fields(clean)
			if len(parts) > 0 {
				if count, err := strconv.Atoi(parts[0]); err == nil && count >= 0 {
					meta.RetryCount = count
					if len(parts) > 1 {
						meta.RetryDelay = parts[1]
					} else {
						meta.RetryDelay = "5s"
					}
				}
			}
		}

		if maxRunsMatch := maxRunsRegex.FindStringSubmatch(doc); len(maxRunsMatch) > 1 {
			clean := cleanCommentString(maxRunsMatch[1])
			if n, err := strconv.Atoi(clean); err == nil && n >= 0 {
				meta.MaxRuns = n
			}
		}

		if mcpMatch := mcpRegex.FindStringSubmatch(doc); len(mcpMatch) > 1 {
			meta.IsMCP = mcpMatch[1] == "true"
		}

		if execMatch := allowExecRegex.FindStringSubmatch(doc); len(execMatch) > 1 {
			meta.AllowExec = execMatch[1] == "true"
		}

		if timeoutMatch := timeoutRegex.FindStringSubmatch(doc); len(timeoutMatch) > 1 {
			if sec, err := strconv.Atoi(timeoutMatch[1]); err == nil && sec > 0 {
				meta.TimeoutSeconds = sec
			}
		}

		paramMatches := paramRegex.FindAllStringSubmatch(doc, -1)
		for _, pm := range paramMatches {
			pType := strings.TrimSpace(pm[1])
			pName := strings.TrimSpace(pm[2])
			pDesc := ""
			if len(pm) > 3 {
				pDesc = strings.TrimSpace(pm[3])
			}
			meta.Params = append(meta.Params, domain.ParamSchema{
				Name:        pName,
				Type:        pType,
				Description: pDesc,
				Required:    true,
			})
		}
	}

	return meta, nil
}
