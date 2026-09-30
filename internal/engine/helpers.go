package engine

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"actacron/internal/storage"
	"github.com/dop251/goja"
	"github.com/google/uuid"
)

func registerConsole(vm *goja.Runtime, logs *[]string) {
	consoleObj := vm.NewObject()

	appendLog := func(level string, call goja.FunctionCall) goja.Value {
		var parts []string
		for _, arg := range call.Arguments {
			parts = append(parts, fmt.Sprint(arg.Export()))
		}
		msg := strings.Join(parts, " ")
		timestamp := time.Now().Format("15:04:05.000")
		formatted := fmt.Sprintf("[%s] [%s] %s", timestamp, level, msg)
		*logs = append(*logs, formatted)
		return goja.Undefined()
	}

	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		return appendLog("INFO", call)
	})
	consoleObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		return appendLog("WARN", call)
	})
	consoleObj.Set("error", func(call goja.FunctionCall) goja.Value {
		return appendLog("ERROR", call)
	})

	vm.Set("console", consoleObj)
}

func registerStorage(vm *goja.Runtime, db *storage.DB, pkgName string) {
	storageObj := vm.NewObject()

	storageObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Null()
		}
		key := call.Arguments[0].String()
		valStr, err := db.GetKV(pkgName, key)
		if err != nil || valStr == "" {
			return goja.Null()
		}

		// Try unmarshal JSON if possible
		var parsed interface{}
		if err := json.Unmarshal([]byte(valStr), &parsed); err == nil {
			return vm.ToValue(parsed)
		}
		return vm.ToValue(valStr)
	})

	storageObj.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		rawVal := call.Arguments[1].Export()

		var valStr string
		switch v := rawVal.(type) {
		case string:
			valStr = v
		default:
			bytes, err := json.Marshal(v)
			if err != nil {
				valStr = fmt.Sprint(v)
			} else {
				valStr = string(bytes)
			}
		}

		db.SetKV(pkgName, key, valStr)
		return goja.Undefined()
	})

	storageObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			db.DeleteKV(pkgName, call.Arguments[0].String())
		}
		return goja.Undefined()
	})

	vm.Set("storage", storageObj)
}

func registerCrypto(vm *goja.Runtime) {
	cryptoObj := vm.NewObject()

	cryptoObj.Set("md5", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		hash := md5.Sum([]byte(data))
		return vm.ToValue(hex.EncodeToString(hash[:]))
	})

	cryptoObj.Set("sha256", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		hash := sha256.Sum256([]byte(data))
		return vm.ToValue(hex.EncodeToString(hash[:]))
	})

	cryptoObj.Set("uuid", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(uuid.New().String())
	})

	vm.Set("crypto", cryptoObj)
}

func registerBase64(vm *goja.Runtime) {
	b64Obj := vm.NewObject()

	b64Obj.Set("encode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(data)))
	})

	b64Obj.Set("decode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("base64 decode error: %v", err)))
		}
		return vm.ToValue(string(decoded))
	})

	vm.Set("base64", b64Obj)
}

func registerHTTP(vm *goja.Runtime) {
	httpObj := vm.NewObject()
	client := &http.Client{Timeout: 30 * time.Second}

	doRequestWithBody := func(method string, call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http." + strings.ToLower(method) + " requires a url argument"))
		}
		url := call.Arguments[0].String()

		var bodyReader io.Reader
		if len(call.Arguments) > 1 {
			arg1 := call.Arguments[1].Export()
			switch v := arg1.(type) {
			case string:
				bodyReader = strings.NewReader(v)
			default:
				bytes, _ := json.Marshal(v)
				bodyReader = strings.NewReader(string(bytes))
			}
		}

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		req.Header.Set("Content-Type", "application/json")

		if len(call.Arguments) > 2 {
			if headers, ok := call.Arguments[2].Export().(map[string]interface{}); ok {
				for k, v := range headers {
					req.Header.Set(k, fmt.Sprint(v))
				}
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		resObj := vm.NewObject()
		resObj.Set("status", resp.StatusCode)
		resObj.Set("body", string(respBody))
		return resObj
	}

	doRequestNoBody := func(method string, call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http." + strings.ToLower(method) + " requires a url argument"))
		}
		url := call.Arguments[0].String()

		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}

		if len(call.Arguments) > 1 {
			if headers, ok := call.Arguments[1].Export().(map[string]interface{}); ok {
				for k, v := range headers {
					req.Header.Set(k, fmt.Sprint(v))
				}
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		resObj := vm.NewObject()
		resObj.Set("status", resp.StatusCode)
		resObj.Set("body", string(body))
		return resObj
	}

	httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		return doRequestNoBody("GET", call)
	})
	httpObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		return doRequestNoBody("DELETE", call)
	})
	httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		return doRequestWithBody("POST", call)
	})
	httpObj.Set("put", func(call goja.FunctionCall) goja.Value {
		return doRequestWithBody("PUT", call)
	})
	httpObj.Set("patch", func(call goja.FunctionCall) goja.Value {
		return doRequestWithBody("PATCH", call)
	})

	vm.Set("http", httpObj)
	vm.Set("fetch", httpObj.Get("get"))
}

func registerSleep(vm *goja.Runtime) {
	vm.Set("sleep", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			ms := call.Arguments[0].ToInteger()
			if ms > 0 {
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}
		}
		return goja.Undefined()
	})
}

func registerEnv(vm *goja.Runtime) {
	RegisterEnv(vm, nil, nil)
}

func RegisterEnv(vm *goja.Runtime, resolver func(string) string, allVars map[string]string) {
	envFn := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		key := call.Arguments[0].String()
		if resolver != nil {
			return vm.ToValue(resolver(key))
		}
		return vm.ToValue(os.Getenv(key))
	}

	envVal := vm.ToValue(envFn)
	if obj, ok := envVal.(*goja.Object); ok {
		obj.Set("get", envFn)
		obj.Set("all", func(call goja.FunctionCall) goja.Value {
			res := make(map[string]string)
			if allVars != nil {
				for k, v := range allVars {
					res[k] = v
				}
			} else {
				for _, e := range os.Environ() {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						res[parts[0]] = parts[1]
					}
				}
			}
			return vm.ToValue(res)
		})
	}
	vm.Set("env", envVal)
}

func registerExec(vm *goja.Runtime, allowExec bool) {
	vm.Set("exec", func(call goja.FunctionCall) goja.Value {
		if !allowExec {
			panic(vm.ToValue("system command execution (exec) is disabled for security"))
		}
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("exec requires command argument"))
		}

		cmdName := call.Arguments[0].String()
		var args []string
		if len(call.Arguments) > 1 {
			if argSlice, ok := call.Arguments[1].Export().([]interface{}); ok {
				for _, a := range argSlice {
					args = append(args, fmt.Sprint(a))
				}
			}
		}

		cmd := exec.Command(cmdName, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		resObj := vm.NewObject()
		resObj.Set("stdout", stdout.String())
		resObj.Set("stderr", stderr.String())
		resObj.Set("exitCode", exitCode)
		return resObj
	})
}
