package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

var (
	instanceIDFile     string
	secretKeyFile      string
	registrationLambda string
	influxDBURL        string
	influxDBToken      string
	influxDBOrg        string
	influxDBBucket     string
	lambdaClient       *lambda.Lambda
	influxClient       influxdb2.Client
)

func init() {
	instanceIDFile = getEnv("INSTANCE_ID_FILE", "/var/lib/cloud/data/instance-id")
	secretKeyFile = getEnv("SECRET_KEY_FILE", "/etc/secret_key")
	registrationLambda = getEnvOrFatal("REGISTRATION_LAMBDA")
	influxDBURL = getEnvOrFatal("INFLUXDB_URL")
	influxDBToken = getEnvOrFatal("INFLUXDB_TOKEN")
	influxDBOrg = getEnvOrFatal("INFLUXDB_ORG")
	influxDBBucket = getEnvOrFatal("INFLUXDB_BUCKET")

	sess := session.Must(session.NewSession())
	lambdaClient = lambda.New(sess)
	influxClient = influxdb2.NewClient(influxDBURL, influxDBToken)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvOrFatal(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s environment variable is not set", key)
	}
	return value
}

func getInstanceID() (string, error) {
	if _, err := os.Stat(instanceIDFile); err == nil {
		id, err := ioutil.ReadFile(instanceIDFile)
		return string(id), err
	}

	newID := make([]byte, 3)
	if _, err := rand.Read(newID); err != nil {
		return "", err
	}
	id := hex.EncodeToString(newID)

	if err := ioutil.WriteFile(instanceIDFile, []byte(id), 0644); err != nil {
		return "", err
	}
	return id, nil
}

func getInstanceIP() (string, error) {
	sess := session.Must(session.NewSession())
	metadataClient := ec2metadata.New(sess)
	return metadataClient.GetMetadata("local-ipv4")
}

func registerWithLambda(isNew bool) (bool, error) {
	id, err := getInstanceID()
	if err != nil {
		return false, err
	}

	ip, err := getInstanceIP()
	if err != nil {
		return false, err
	}

	payload, err := json.Marshal(map[string]interface{}{
		"id":     id,
		"ip":     ip,
		"is_new": isNew,
	})
	if err != nil {
		return false, err
	}

	result, err := lambdaClient.Invoke(&lambda.InvokeInput{
		FunctionName: aws.String(registrationLambda),
		Payload:      payload,
	})
	if err != nil {
		return false, err
	}

	var response struct {
		StatusCode int `json:"statusCode"`
	}
	if err := json.Unmarshal(result.Payload, &response); err != nil {
		return false, err
	}

	return response.StatusCode == 200, nil
}

func getSecretKey() (string, error) {
	key, err := ioutil.ReadFile(secretKeyFile)
	return string(key), err
}

func updateSecretKey(newKey string) error {
	return ioutil.WriteFile(secretKeyFile, []byte(newKey), 0600)
}

func writeToInfluxDB() error {
	writeAPI := influxClient.WriteAPI(influxDBOrg, influxDBBucket)

	id, err := getInstanceID()
	if err != nil {
		return err
	}

	secretKey, err := getSecretKey()
	if err != nil {
		return err
	}

	p := influxdb2.NewPoint("secret_key_hash",
		map[string]string{"instance_id": id},
		map[string]interface{}{"hash": fmt.Sprintf("%x", sha256.Sum256([]byte(secretKey)))},
		time.Now())
	writeAPI.WritePoint(p)

	return nil
}

func updateSecretHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		OldKey string `json:"old_key"`
		NewKey string `json:"new_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentKey, err := getSecretKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if request.OldKey != currentKey {
		http.Error(w, "Invalid old key", http.StatusBadRequest)
		return
	}

	if err := updateSecretKey(request.NewKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func main() {
	isNew := true
	if _, err := os.Stat(instanceIDFile); err == nil {
		isNew = false
	}

	registered, err := registerWithLambda(isNew)
	if err != nil {
		log.Fatalf("Failed to register with Lambda: %v", err)
	}

	if !registered {
		log.Fatal("Failed to register with Lambda function")
	}

	http.HandleFunc("/update_secret", updateSecretHandler)
	go func() {
		log.Fatal(http.ListenAndServe(":5000", nil))
	}()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := writeToInfluxDB(); err != nil {
				log.Printf("Failed to write to InfluxDB: %v", err)
			}
		}
	}
}