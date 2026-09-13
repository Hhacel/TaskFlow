// Package e2e contains full end-to-end tests for the TaskFlow system.
//
// Unlike the other test suites in this repository, these tests treat
// TaskFlow as a real black box: they build the actual api, orchestrator and
// worker binaries, run them as separate OS processes wired to a real
// PostgreSQL database and a real (embedded) NATS server, and drive them
// exclusively through the public HTTP API — exactly as a real client or the
// production docker-compose stack would. Every test then asserts on the
// resulting rows in PostgreSQL directly, using the public pkg/persistence
// package.
//
// These tests require a reachable PostgreSQL instance (see
// docker-compose.test.yaml / `make test-e2e`) and are skipped when running
// `go test -short`.
package e2e

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/hhace/taskflow/pkg/persistence"
)

// Fixed, non-default ports used by the test service processes so they never
// collide with a dev docker-compose stack that might already be running on
// the machine (api:8081, worker:8082, orchestrator:8084).
const (
	apiPort          = "18081"
	workerPort       = "18082"
	orchestratorPort = "18084"
)

// service tracks a single running TaskFlow binary spawned for the test suite.
type service struct {
	name    string
	cmd     *exec.Cmd
	baseURL string
}

// env bundles every live component needed to drive the system end-to-end.
type env struct {
	db         *gorm.DB
	repo       persistence.RepositoryInterface
	natsServer *natsserver.Server
	apiURL     string
	services   []*service
}

var testEnv *env

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	e, err := setupEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e setup failed:", err)
		os.Exit(1)
	}
	testEnv = e

	code := m.Run()

	e.teardown()
	os.Exit(code)
}

func setupEnv() (*env, error) {
	dbCfg := testDBConfig()

	db, err := persistence.Connect(dbCfg, 10)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := persistence.Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	ns, err := startEmbeddedNATS()
	if err != nil {
		return nil, fmt.Errorf("start embedded nats: %w", err)
	}

	binDir, err := buildBinaries()
	if err != nil {
		return nil, fmt.Errorf("build service binaries: %w", err)
	}

	baseEnv := append(os.Environ(),
		"CONFIG_PATH=",
		"DB_HOST="+dbCfg.Host,
		"DB_PORT="+dbCfg.Port,
		"DB_USER="+dbCfg.User,
		"DB_PASSWORD="+dbCfg.Password,
		"DB_NAME="+dbCfg.Database,
		"NATS_URL="+ns.ClientURL(),
		"LOG_LEVEL=error",
	)

	var services []*service

	apiSvc, err := startService("api", filepath.Join(binDir, exeName("api")), append(append([]string{}, baseEnv...), "PORT="+apiPort), apiPort)
	if err != nil {
		return nil, err
	}
	services = append(services, apiSvc)

	orchSvc, err := startService("orchestrator", filepath.Join(binDir, exeName("orchestrator")), append(append([]string{}, baseEnv...), "PORT="+orchestratorPort), orchestratorPort)
	if err != nil {
		return nil, err
	}
	services = append(services, orchSvc)

	workerSvc, err := startService("worker", filepath.Join(binDir, exeName("worker")), append(append([]string{}, baseEnv...), "PORT="+workerPort), workerPort)
	if err != nil {
		return nil, err
	}
	services = append(services, workerSvc)

	return &env{
		db:         db,
		repo:       persistence.NewRepository(db),
		natsServer: ns,
		apiURL:     apiSvc.baseURL,
		services:   services,
	}, nil
}

func (e *env) teardown() {
	for _, s := range e.services {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			_ = s.cmd.Wait()
		}
	}
	if e.natsServer != nil {
		e.natsServer.Shutdown()
	}
	_ = persistence.Close(e.db)
}

// buildBinaries compiles the api, orchestrator and worker binaries once into
// a temporary directory so every test process runs the exact same code that
// would be built for production.
func buildBinaries() (string, error) {
	dir, err := os.MkdirTemp("", "taskflow-e2e-bin")
	if err != nil {
		return "", err
	}

	targets := []struct {
		name string
		pkg  string
	}{
		{"api", "github.com/hhace/taskflow/apps/api/cmd"},
		{"orchestrator", "github.com/hhace/taskflow/apps/orchestrator/cmd"},
		{"worker", "github.com/hhace/taskflow/apps/worker/cmd"},
	}

	for _, target := range targets {
		out := filepath.Join(dir, exeName(target.name))
		cmd := exec.Command("go", "build", "-o", out, target.pkg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("build %s: %w", target.name, err)
		}
	}

	return dir, nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// startService launches a built TaskFlow binary and waits until its /health
// endpoint reports success, meaning it has connected to both Postgres (api,
// orchestrator) and NATS (all three) and is ready to serve traffic.
func startService(name, binPath string, env []string, port string) (*service, error) {
	cmd := exec.Command(binPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	baseURL := "http://127.0.0.1:" + port
	if err := waitForHealth(baseURL, 20*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("%s did not become healthy: %w", name, err)
	}

	return &service{name: name, cmd: cmd, baseURL: baseURL}, nil
}

func waitForHealth(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s/health", baseURL)
}

func startEmbeddedNATS() (*natsserver.Server, error) {
	opts := &natsserver.Options{
		Host: "127.0.0.1",
		Port: -1, // random free port
	}

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, fmt.Errorf("embedded NATS server not ready")
	}

	return ns, nil
}

func testDBConfig() persistence.Config {
	return persistence.Config{
		Host:                   getEnv("TEST_DB_HOST", "localhost"),
		Port:                   getEnv("TEST_DB_PORT", "5433"),
		User:                   getEnv("TEST_DB_USER", "taskflow"),
		Password:               getEnv("TEST_DB_PASSWORD", "taskflow"),
		Database:               getEnv("TEST_DB_NAME", "taskflow_test"),
		SSLMode:                getEnv("TEST_DB_SSLMODE", "disable"),
		MaxOpenConns:           10,
		MaxIdleConns:           5,
		ConnMaxLifetimeMinutes: 15,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// resetDB truncates every TaskFlow table so each test starts from a clean slate.
func resetDB(t *testing.T) {
	t.Helper()
	err := testEnv.db.Exec("TRUNCATE TABLE task_results, task_dependencies, tasks, workflows RESTART IDENTITY CASCADE").Error
	require.NoError(t, err)
}

// doRequest performs a real HTTP request against the spawned API process and
// returns the status code and raw response body.
func doRequest(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, testEnv.apiURL+path, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, respBody
}

// waitForCondition polls cond until it returns true or timeout elapses,
// failing the test otherwise. Used to await asynchronous Orchestrator/Worker
// processing that happens over NATS after an HTTP call returns.
func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}
