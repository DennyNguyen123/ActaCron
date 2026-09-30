package parser

import (
	"fmt"
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
	mcpRegex        = regexp.MustCompile(`@mcp\s+(true|false)`)
	allowExecRegex  = regexp.MustCompile(`@allowExec\s+(true|false)`)
	timeoutRegex    = regexp.MustCompile(`@timeout\s+([0-9]+)`)
	paramRegex      = regexp.MustCompile(`@param\s+\{([^}]+)\}\s+([a-zA-Z0-9_-]+)(?:\s*-\s*([^\r\n]+))?`)
	cronParserValid = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
)

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
			expr := strings.TrimSpace(cronMatch[1])
			// Strip inline comments if any
			if idx := strings.Index(expr, "//"); idx != -1 {
				expr = strings.TrimSpace(expr[:idx])
			}
			if idx := strings.Index(expr, "/*"); idx != -1 {
				expr = strings.TrimSpace(expr[:idx])
			}
			if expr != "" {
				if _, err := cronParserValid.Parse(expr); err != nil {
					return nil, fmt.Errorf("invalid cron expression '%s': %w", expr, err)
				}
				meta.CronExpr = expr
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
