package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

var (
	db             *sql.DB
	influxClient   influxdb2.Client
	influxQueryAPI influxdb2.QueryAPI
	influxDBURL    string
	influxDBToken  string
	influxDBOrg    string
	influxDBBucket string
)

type Instance struct {
	ID            string
	IP            string
	SecretKeyHash string
}

func init() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is not set")
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping the database: %v", err)
	}

	influxDBURL = os.Getenv("INFLUXDB_URL")
	if influxDBURL == "" {
		log.Fatal("INFLUXDB_URL environment variable is not set")
	}

	influxDBToken = os.Getenv("INFLUXDB_TOKEN")
	if influxDBToken == "" {
		log.Fatal("INFLUXDB_TOKEN environment variable is not set")
	}

	influxDBOrg = os.Getenv("INFLUXDB_ORG")
	if influxDBOrg == "" {
		log.Fatal("INFLUXDB_ORG environment variable is not set")
	}

	influxDBBucket = os.Getenv("INFLUXDB_BUCKET")
	if influxDBBucket == "" {
		log.Fatal("INFLUXDB_BUCKET environment variable is not set")
	}

	influxClient = influxdb2.NewClient(influxDBURL, influxDBToken)
	influxQueryAPI = influxClient.QueryAPI(influxDBOrg)
}

func handleRequest(ctx context.Context) error {
	instances, err := getAllInstancesFromRDS()
	if err != nil {
		return fmt.Errorf("error getting instances from RDS: %v", err)
	}

	for _, instance := range instances {
		latestHash, err := getLatestHashFromInfluxDB(instance.ID)
		if err != nil {
			log.Printf("Error getting latest hash for instance %s: %v\n", instance.ID, err)
			continue
		}

		if confirmSecretKey(instance.ID, latestHash) {
			newSecretKey, err := generateNewSecretKey()
			if err != nil {
				log.Printf("Error generating new secret key for instance %s: %v\n", instance.ID, err)
				continue
			}

			if updateSecretKeyOnEC2(instance.IP, latestHash, newSecretKey) {
				if err := updateSecretKeyInRDS(instance.ID, newSecretKey); err != nil {
					log.Printf("Error updating secret key in RDS for instance %s: %v\n", instance.ID, err)
				}
			} else {
				log.Printf("Failed to update secret key for instance %s\n", instance.ID)
			}
		} else {
			log.Printf("Secret key mismatch for instance %s\n", instance.ID)
		}
	}

	return nil
}

func getAllInstancesFromRDS() ([]Instance, error) {
	rows, err := db.Query("SELECT ID, IP, SecretKeyHash FROM EC2Instances")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var instance Instance
		if err := rows.Scan(&instance.ID, &instance.IP, &instance.SecretKeyHash); err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}

	return instances, rows.Err()
}

func getLatestHashFromInfluxDB(instanceID string) (string, error) {
	query := fmt.Sprintf(`from(bucket:"%s")
		|> range(start: -10m)
		|> filter(fn: (r) => r._measurement == "secret_key_hash" and r.instance_id == "%s")
		|> last()`, influxDBBucket, instanceID)
	result, err := influxQueryAPI.Query(context.Background(), query)
	if err != nil {
		return "", err
	}
	defer result.Close()

	var hash string
	for result.Next() {
		hash = result.Record().Values()["hash"].(string)
	}
	if result.Err() != nil {
		return "", result.Err()
	}
	return hash, nil
}

func confirmSecretKey(instanceID, hash string) bool {
	var storedHash string
	err := db.QueryRow("SELECT SecretKeyHash FROM EC2Instances WHERE ID = ?", instanceID).Scan(&storedHash)
	if err != nil {
		log.Printf("Error confirming secret key for instance %s: %v\n", instanceID, err)
		return false
	}
	return storedHash == hash
}

func generateNewSecretKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func updateSecretKeyOnEC2(ip, oldKey, newKey string) bool {
	url := fmt.Sprintf("http://%s:5000/update_secret", ip)
	payload := fmt.Sprintf(`{"old_key":"%s","new_key":"%s"}`, oldKey, newKey)
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		log.Printf("Error updating secret key on EC2: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func updateSecretKeyInRDS(instanceID, newKey string) error {
	_, err := db.Exec("UPDATE EC2Instances SET SecretKeyHash = ? WHERE ID = ?", newKey, instanceID)
	return err
}

func main() {
	lambda.Start(handleRequest)
}