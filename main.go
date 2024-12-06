package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"fmt"

	"net/smtp"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	supabase "github.com/lengzuo/supa"
)

type MailService struct {
	supaClient *supabase.Client
	services   map[string]Service
}

type Service struct {
	ServiceID   string `json:"service_id"`
	UserID      string `json:"user_id"`
	HostAddress string `json:"host_address"`
	Port        int    `json:"port"`
	EmailID     string `json:"email_id"`
	Password    string `json:"password"`
	CorsOrigin  string `json:"cors_origin"`
}

type Profile struct {
	ProfileID string `json:"profile_id"`
	UserID    string `json:"user_id"`
	UserKey   string `json:"user_key"`
}

type EmailRequest struct {
	UserKey    string                 `json:"user_key"`
	ServiceID  string                 `json:"service_id"`
	TemplateID string                 `json:"template_id"`
	Recipients []Recipient            `json:"recipients"`
	Parameters map[string]interface{} `json:"parameters"`
}

type Recipient struct {
	EmailAddress string `json:"email_address"`
	Name         string `json:"name,omitempty"`
}

type EmailResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

func NewMailService() (*MailService, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	conf := supabase.Config{
		ApiKey:     os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		ProjectRef: os.Getenv("SUPABASE_PROJECT_REF"),
		Debug:      false,
	}

	supaClient, err := supabase.New(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Supabase client: %v", err)
	}

	return &MailService{
		supaClient: supaClient,
		services:   make(map[string]Service),
	}, nil
}

func (ms *MailService) SendEmailsHandler(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := EmailResponse{Success: true}
	var wg sync.WaitGroup
	errorChan := make(chan error, len(req.Recipients))

	for _, recipient := range req.Recipients {
		wg.Add(1)
		go func(rec Recipient) {
			defer wg.Done()
			if err := ms.sendSingleEmail(&req, rec); err != nil {
				errorChan <- fmt.Errorf("error sending to %s: %v", rec.EmailAddress, err)
			}
		}(recipient)
	}

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	for err := range errorChan {
		response.Success = false
		response.Errors = append(response.Errors, err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendSingleEmail sends an email to a single recipient using the specified email service and template.
// It performs the following steps:
//  1. Fetches the email service configuration from the database
//  2. Parses any date parameters in RFC3339 format
//  3. Retrieves the email template from the database
//  4. Executes the template with recipient data and parameters
//  5. Constructs and sends the email via SMTP
//  6. Logs the email sending attempt and outcome
//  7. Records the sent email in the emails table
//
// Parameters:
//   - req: Email request containing service ID, template ID, user ID and parameters
//   - recipient: Recipient details including email address and name
//
// Returns:
//   - error: nil if successful, otherwise an error describing what went wrong
//
// The function supports template functions including:
//   - formatDate: Formats time.Time as "2006-01-02 15:04:05"
//   - upper: Converts text to uppercase
//   - lower: Converts text to lowercase
//   - title: Converts text to title case
func (ms *MailService) sendSingleEmail(req *EmailRequest, recipient Recipient) error {
	ctx := context.Background()

	// First get UserID from profile using UserKey
	var profiles []Profile
	err := ms.supaClient.DB.From("profile").
		Select("*").
		Eq("user_key", req.UserKey).
		Execute(ctx, &profiles)
	if err != nil || len(profiles) == 0 {
		return fmt.Errorf("invalid user key: %v", err)
	}
	userID := profiles[0].UserID

	// Then get service using UserID
	var services []Service
	err = ms.supaClient.DB.From("services").
		Select("*").
		Eq("service_id", req.ServiceID).
		Eq("user_id", userID).
		Execute(ctx, &services)
	if err != nil || len(services) == 0 {
		return fmt.Errorf("failed to fetch service: %v", err)
	}
	service := services[0]

	if rawDate, ok := req.Parameters["date"].(string); ok {
		parsedDate, err := time.Parse(time.RFC3339, rawDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %v", err)
		}
		req.Parameters["date"] = parsedDate
	}

	var templates []struct {
		Content string `json:"content"`
		Subject string `json:"subject"`
	}
	err = ms.supaClient.DB.From("templates").
		Select("content", "subject").
		Eq("template_id", req.TemplateID).
		Eq("user_id", userID).
		Execute(ctx, &templates)
	if err != nil || len(templates) == 0 {
		return fmt.Errorf("failed to fetch template: %v", err)
	}

	tmplData := templates[0]

	// Create template with function map for additional template functionality
	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
	}

	tmpl, err := template.New("email").Funcs(funcMap).Parse(tmplData.Content)
	if err != nil {
		return fmt.Errorf("template parsing error: %v", err)
	}

	// Create template context with recipient data and parameters
	templateContext := map[string]interface{}{
		"recipient": recipient,
		"params":    req.Parameters,
	}

	var body strings.Builder
	if err := tmpl.Execute(&body, templateContext); err != nil {
		return fmt.Errorf("template execution error: %v", err)
	}

	headers := map[string]string{
		"MIME-Version":              "1.0",
		"Content-Type":              "text/html; charset=UTF-8",
		"Subject":                   tmplData.Subject,
		"From":                      service.EmailID,
		"To":                        recipient.EmailAddress,
		"X-Priority":                "3",
		"X-Mailer":                  "Portfolio Mailer",
		"Content-Transfer-Encoding": "8bit",
	}

	var message strings.Builder
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")
	message.WriteString(body.String())

	auth := smtp.PlainAuth("", service.EmailID, service.Password, service.HostAddress)
	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", service.HostAddress, service.Port),
		auth,
		service.EmailID,
		[]string{recipient.EmailAddress},
		[]byte(message.String()),
	)

	if err != nil {
		logData := map[string]interface{}{
			"user_id":       userID,
			"service_id":    req.ServiceID,
			"template_id":   req.TemplateID,
			"status":        "failed",
			"error_message": err.Error(),
			"email_address": recipient.EmailAddress,
			"created_at":    time.Now(),
		}
		ms.supaClient.DB.From("logs").Insert(logData).Execute(ctx, nil)
		return fmt.Errorf("email sending error: %v", err)
	}

	logData := map[string]interface{}{
		"user_id":     userID,
		"service_id":  req.ServiceID,
		"template_id": req.TemplateID,
		"status":      "success",
		"message":     fmt.Sprintf("Email sent to %s", recipient.EmailAddress),
		"created_at":  time.Now(),
	}
	ms.supaClient.DB.From("logs").Insert(logData).Execute(ctx, nil)

	emailData := map[string]interface{}{
		"user_id":       userID,
		"service_id":    req.ServiceID,
		"template_id":   req.TemplateID,
		"email_address": recipient.EmailAddress,
		"name":          recipient.Name,
		"sent_at":       time.Now(),
	}
	ms.supaClient.DB.From("emails").Insert(emailData).Execute(ctx, nil)

	return nil
}

func (ms *MailService) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Add CORS middleware
func corsMiddleware(service *Service, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service.CorsOrigin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != service.CorsOrigin {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", service.CorsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func main() {
	mailService, err := NewMailService()
	if err != nil {
		log.Fatalf("Failed to initialize mail service: %v", err)
	}

	r := mux.NewRouter()

	// Get service for CORS configuration
	r.HandleFunc("/send-emails", func(w http.ResponseWriter, r *http.Request) {
		var req EmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		var profiles []Profile
		err := mailService.supaClient.DB.From("profile").
			Select("*").
			Eq("user_key", req.UserKey).
			Execute(ctx, &profiles)
		if err != nil || len(profiles) == 0 {
			http.Error(w, "Invalid user key", http.StatusUnauthorized)
			return
		}

		var services []Service
		err = mailService.supaClient.DB.From("services").
			Select("*").
			Eq("service_id", req.ServiceID).
			Execute(ctx, &services)
		if err != nil || len(services) == 0 {
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		}

		corsMiddleware(&services[0], mailService.SendEmailsHandler).ServeHTTP(w, r)
	}).Methods("POST")

	r.HandleFunc("/health", mailService.HealthCheckHandler).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
