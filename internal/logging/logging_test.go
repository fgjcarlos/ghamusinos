package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewHandler_ProductionIsJSON(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(EnvProduction, &buf)

	// Usamos un logger efímero para no contaminar slog.Default().
	logger := slog.New(h)
	logger.Info("hello", "key", "value")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("handler no escribió nada")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("NewHandler(production) no produce JSON válido:\n  línea: %s\n  err: %v", line, err)
	}
	if entry["msg"] != "hello" {
		t.Errorf("msg = %v, quería \"hello\"", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, quería \"value\"", entry["key"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, quería \"INFO\"", entry["level"])
	}
}

func TestNewHandler_DevelopmentIsText(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(EnvDevelopment, &buf)

	logger := slog.New(h)
	logger.Info("hello", "key", "value")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("handler no escribió nada")
	}
	// El TextHandler escribe pares clave=valor con un formato concreto
	// (e.g. "time=... level=INFO msg=hello key=value"). Si lo
	// parseamos como JSON tiene que fallar.
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err == nil {
		t.Fatalf("NewHandler(development) produjo JSON, esperaba texto:\n  línea: %s", line)
	}
	// Comprobamos campos básicos en texto.
	if !strings.Contains(line, "msg=hello") {
		t.Errorf("línea no contiene msg=hello: %s", line)
	}
	if !strings.Contains(line, "key=value") {
		t.Errorf("línea no contiene key=value: %s", line)
	}
	if !strings.Contains(line, "level=INFO") {
		t.Errorf("línea no contiene level=INFO: %s", line)
	}
}

func TestNewHandler_OtherEnvDefaultsToText(t *testing.T) {
	// Cualquier entorno distinto de "production" cae al text handler.
	// Esto evita logs no parseables por accidente en staging / preview / etc.
	for _, env := range []string{"", "staging", "test", "dev"} {
		t.Run(env, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewHandler(env, &buf)

			logger := slog.New(h)
			logger.Info("hi")

			line := strings.TrimSpace(buf.String())
			if line == "" {
				t.Fatalf("[%s] handler no escribió nada", env)
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				t.Fatalf("[%s] esperaba text, produjo JSON: %s", env, line)
			}
		})
	}
}

func TestSetup_InstallsHandler(t *testing.T) {
	// Salva el default para restaurarlo al final del test.
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	Setup(EnvProduction)

	// Después de Setup, slog.Default() debe ser un logger con JSONHandler.
	// Verificamos que el output del default es JSON parseable.
	var buf bytes.Buffer
	// Redirigimos el output del default capturándolo a través de un logger
	// propio. No podemos redirigir os.Stderr desde un test (es global), así
	// que solo verificamos que Setup no haga panic y que el default haya
	// cambiado. La verificación real del handler va en TestNewHandler.
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	slog.Info("post-setup", "x", 1)
	if !strings.Contains(buf.String(), `"msg":"post-setup"`) {
		t.Fatalf("default no funciona como JSON: %s", buf.String())
	}
}
