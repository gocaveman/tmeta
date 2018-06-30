package tmetadbr

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	"gopkg.in/ory-am/dockertest.v3"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

var mysqlEnable = flag.Bool("mysql", false, "Enable MySQL tests via docker")
var mysqlConnStr = ""
var postgresEnable = flag.Bool("postgres", false, "Enable Postgres tests via docker")
var postgresConnStr = ""

// TestMain initializes docker instances for mysql and postgres if requested
func TestMain(m *testing.M) {

	flag.Parse()

	var code int
	defer func() {
		os.Exit(code)
	}()

	if *mysqlEnable || *postgresEnable {

		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		pool, err := dockertest.NewPool("")
		if err != nil {
			log.Fatalf("Could not connect to docker: %s", err)
		}

		if *mysqlEnable {

			log.Printf("Initializing MySQL docker container...")

			// pulls an image, creates a container based on it and runs it
			resource, err := pool.Run("mysql", "5.7", []string{"MYSQL_ROOT_PASSWORD=secret"})
			if err != nil {
				log.Fatalf("Could not start resource: %s", err)
			}

			// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
			if err := pool.Retry(func() error {
				var err error
				mysqlConnStr = fmt.Sprintf("root:secret@(localhost:%s)/mysql", resource.GetPort("3306/tcp"))
				db, err := sql.Open("mysql", mysqlConnStr)
				if err != nil {
					return err
				}
				return db.Ping()
			}); err != nil {
				log.Fatalf("Could not connect to docker: %s", err)
			}

			defer func() {
				if err := pool.Purge(resource); err != nil {
					log.Fatalf("Could not purge resource: %s", err)
				}
			}()

		}

		if *postgresEnable {

			log.Printf("Initializing Postgres docker container...")

			// pulls an image, creates a container based on it and runs it
			resource, err := pool.Run("postgres", "10.3", []string{"POSTGRES_PASSWORD=secret"})
			if err != nil {
				log.Fatalf("Could not start resource: %s", err)
			}

			// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
			if err := pool.Retry(func() error {
				var err error
				postgresConnStr = fmt.Sprintf("user=postgres password=secret dbname=postgres port=%v sslmode=disable connect_timeout=5", resource.GetPort("5432/tcp"))
				db, err := sql.Open("postgres", postgresConnStr)
				if err != nil {
					return err
				}
				return db.Ping()
			}); err != nil {
				log.Fatalf("Could not connect to docker: %s", err)
			}

			defer func() {
				if err := pool.Purge(resource); err != nil {
					log.Fatalf("Could not purge resource: %s", err)
				}
			}()

		}

	}

	code = m.Run()

}
