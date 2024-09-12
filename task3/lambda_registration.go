package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

type Event struct {
	ID    string `json:"id"`
	IP    string `json:"ip"`
	IsNew bool   `json:"is_new"`
}

type Response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

var db *sql.DB

func init() {
	dsn := os.Getenv("DB_DSN")
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping the database: %v", err)
	}
}

func handleRequest(ctx context.Context, event Event) (Response, error) {
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM EC2Instances WHERE ID=?)", event.ID).Scan(&exists)
	if err != nil {
		log.Printf("Error querying database: %v", err)
		return Response{StatusCode: 500, Body: "Error querying database"}, err
	}

	if exists && event.IsNew {
		return Response{StatusCode: 400, Body: "ID already exists, cannot register as new"}, nil
	} else if exists && !event.IsNew {
		_, err := db.ExecContext(ctx, "UPDATE EC2Instances SET IP=? WHERE ID=?", event.IP, event.ID)
		if err != nil {
			log.Printf("Error updating IP address: %v", err)
			return Response{StatusCode: 500, Body: "Error updating IP address"}, err
		}
		return Response{StatusCode: 200, Body: "IP address updated successfully"}, nil
	} else if !exists && event.IsNew {
		_, err := db.ExecContext(ctx, "INSERT INTO EC2Instances (ID, IP) VALUES (?, ?)", event.ID, event.IP)
		if err != nil {
			log.Printf("Error adding new instance: %v", err)
			return Response{StatusCode: 500, Body: "Error adding new instance"}, err
		}
		return Response{StatusCode: 200, Body: "New instance registered successfully"}, nil
	} else {
		return Response{StatusCode: 400, Body: "Invalid request"}, nil
	}
}

func main() {
	lambda.Start(handleRequest)
}