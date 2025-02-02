package storage

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

func TestMain(m *testing.M) {
	code, err := runMain(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

const (
	testDBName       = "test"
	testUserName     = "test"
	testUserPassword = "test"
)

var (
	getDSN          func() string
	getSUConnection func() (*pgx.Conn, error)
)

func initGetDSN(hostAndPort string) {
	getDSN = func() string {
		return fmt.Sprintf(
			"postgres://%s:%s@%s/%s?sslmode=disable",
			testUserName,
			testUserPassword,
			hostAndPort,
			testDBName,
		)
	}
}

func initGetSUConnection(hostPort string) error {
	host, port, err := getHostPort(hostPort)
	if err != nil {
		return fmt.Errorf("failed to extract the host and port parts from the string %s: %w", hostPort, err)
	}
	getSUConnection = func() (*pgx.Conn, error) {
		conn, err := pgx.Connect(pgx.ConnConfig{
			Host:     host,
			Port:     port,
			Database: "postgres",
			User:     "postgres",
			Password: "postgres",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get a super user connection: %w", err)
		}
		return conn, nil
	}
	return nil
}

func runMain(m *testing.M) (int, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return 1, fmt.Errorf("failed to initialize a pool: %w", err)
	}

	pg, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "16",
			Name:       "migration-integration-tests",
			Env: []string{
				"POSTGRES_USER=postgres",
				"POSTGRES_PASSWORD=postgres",
			},
			ExposedPorts: []string{"5432/tcp"},
		},
		func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		return 1, fmt.Errorf("failed to run the postgres container: %w", err)
	}

	defer func() {
		if err := pool.Purge(pg); err != nil {
			log.Printf("failed to purge the postgres container: %v", err)
		}
	}()

	hostPort := pg.GetHostPort("5432/tcp")
	initGetDSN(hostPort)
	if err := initGetSUConnection(hostPort); err != nil {
		return 1, fmt.Errorf("failed to connect as admin to postgresql DB: %w", err)
	}

	pool.MaxWait = 10 * time.Second
	var conn *pgx.Conn
	if err := pool.Retry(func() error {
		conn, err = getSUConnection()
		if err != nil {
			return fmt.Errorf("failed to connect to the DB: %w", err)
		}
		return nil
	}); err != nil {
		return 1, fmt.Errorf("failed to connect to the DB: %w", err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to correctly close the connection: %v", err)
		}
	}()

	if err := createTestDB(conn); err != nil {
		return 1, fmt.Errorf("failed to create a test DB: %w", err)
	}

	exitCode := m.Run()

	return exitCode, nil
}

func createTestDB(conn *pgx.Conn) error {
	_, err := conn.Exec(
		fmt.Sprintf(
			`CREATE USER %s PASSWORD '%s'`,
			testUserName,
			testUserPassword,
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create a test user: %w", err)
	}

	_, err = conn.Exec(
		fmt.Sprintf(`
			CREATE DATABASE %s
				OWNER '%s'
				ENCODING 'UTF8'
				LC_COLLATE = 'en_US.utf8'
				LC_CTYPE = 'en_US.utf8'
			`, testDBName, testUserName,
		),
	)

	if err != nil {
		return fmt.Errorf("failed to create a test DB: %w", err)
	}

	return nil
}

func getHostPort(hostPort string) (string, uint16, error) {
	hostPortParts := strings.Split(hostPort, ":")
	if len(hostPortParts) != 2 {
		return "", 0, fmt.Errorf("got an invalid host-port string: %s", hostPort)
	}

	portStr := hostPortParts[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to cast the port %s to an int: %w", portStr, err)
	}
	return hostPortParts[0], uint16(port), nil
}

func TestUserAdd(t *testing.T) {
	dsn := getDSN()
	if err := runMigrations(dsn); err != nil {
		t.Errorf("failed to run migrations using dsn %s: %v", dsn, err)
		return
	}

	cfg := models.Config{
		ContextTimeout: 10 * time.Second,
	}

	cases := []struct {
		Name        string
		User        models.User
		ExpectedErr error
	}{
		{
			Name: "adding new user - OK",
			User: models.User{
				UserID:   "testuser",
				Password: "testpassword",
				Accrual:  0,
			},
			ExpectedErr: nil,
		},
	}

	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	for i, tc := range cases {
		i, tc := i, tc

		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			actualErr := db.UserAdd(&cfg, tc.User)
			if err := checkErrors(actualErr, tc.ExpectedErr); err != nil {
				t.Error(err)
				return
			}
		})
	}
}

func TestOrderAdd(t *testing.T) {
	dsn := getDSN()
	if err := runMigrations(dsn); err != nil {
		t.Errorf("failed to run migrations using dsn %s: %v", dsn, err)
		return
	}

	cfg := models.Config{
		ContextTimeout: 10 * time.Second,
	}

	cases := []struct {
		Name        string
		UserID      string
		OrderID     string
		ExpectedErr error
	}{
		{
			Name:        "adding new order - OK",
			UserID:      "testuser",
			OrderID:     "2377225624",
			ExpectedErr: nil,
		},
	}

	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	for i, tc := range cases {
		i, tc := i, tc

		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			actualErr := db.OrderAdd(&cfg, tc.UserID, tc.OrderID)
			if err := checkErrors(actualErr, tc.ExpectedErr); err != nil {
				t.Error(err)
				return
			}
		})
	}
}

func checkErrors(actual error, expected error) error {
	if actual == nil && expected == nil {
		return nil
	}
	if expected == nil {
		return fmt.Errorf("expected a nil error, but actually got %w", actual)
	}
	if actual == nil {
		return fmt.Errorf("expected an error %w, but actually got nil", expected)
	}
	if actual.Error() != expected.Error() {
		return fmt.Errorf("expected error %w, got %w", expected, actual)
	}
	return nil
}
