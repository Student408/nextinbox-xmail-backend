package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	supabase "github.com/lengzuo/supa"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Supabase client configuration
	conf := supabase.Config{
		ApiKey:     os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		ProjectRef: os.Getenv("SUPABASE_PROJECT_REF"),
		Debug:      false,
	}

	// Initialize Supabase client
	supaClient, err := supabase.New(conf)
	if err != nil {
		fmt.Println("Failed to initialize Supabase client:", err)
		return
	}
	fmt.Println("Successfully initialized Supabase client")

	// Fetch data from the `services` table
	var services []Service
	err = supaClient.DB.From("services").Select("*").Execute(context.Background(), &services)
	if err != nil {
		fmt.Println("Error fetching data from services table:", err)
		return
	}

	// Print retrieved services data
	for _, service := range services {
		fmt.Printf("Service ID: %s, User ID: %s, Host Address: %s, Port: %d, Email ID: %s\n",
			service.ServiceID, service.UserID, service.HostAddress, service.Port, service.EmailID)
	}
}

// Service represents the structure of the `services` table
type Service struct {
	ServiceID   string `json:"service_id"`
	UserID      string `json:"user_id"`
	HostAddress string `json:"host_address"`
	Port        int    `json:"port"`
	EmailID     string `json:"email_id"`
	Password    string `json:"password"`
	CorsOrigin  string `json:"cors_origin,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
