package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url" // Added import
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

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
	CorsOrigin  string `json:"cors_origin"` // Add CorsOrigin field
}

// Update the EmailRequest struct
type EmailRequest struct {
	UserKey    string                 `json:"user_key"` // Changed from UserID
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
			// Pass the request object to sendSingleEmail
			if err := ms.sendSingleEmail(&req, rec, r); err != nil {
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

// Add new function to get user_id from user_key
func (ms *MailService) getUserIDFromKey(ctx context.Context, userKey string) (string, error) {
	var profiles []struct {
		UserID string `json:"user_id"`
	}

	err := ms.supaClient.DB.From("profile").
		Select("user_id").
		Eq("user_key", userKey).
		Execute(ctx, &profiles)

	if err != nil {
		return "", fmt.Errorf("failed to fetch profile: %v", err)
	}

	if len(profiles) == 0 {
		return "", fmt.Errorf("no profile found for user_key: %s", userKey)
	}

	return profiles[0].UserID, nil
}

// Define structs for logs and emails
type LogEntry struct {
	UserID     string `json:"user_id"`
	ServiceID  string `json:"service_id"`
	TemplateID string `json:"template_id"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

type EmailEntry struct {
	UserID       string `json:"user_id"`
	ServiceID    string `json:"service_id"`
	TemplateID   string `json:"template_id"`
	EmailAddress string `json:"email_address"`
	Name         string `json:"name,omitempty"`
	PhoneNumber  string `json:"phone_number,omitempty"`
}

// Modify sendSingleEmail function
func (ms *MailService) sendSingleEmail(req *EmailRequest, recipient Recipient, r *http.Request) error {
	ctx := context.Background()

	// First get the user_id from user_key
	userID, err := ms.getUserIDFromKey(ctx, req.UserKey)
	if err != nil {
		return fmt.Errorf("invalid user_key: %v", err)
	}

	// Fetch the user's rate_limit from the profile table
	var profiles []struct {
		RateLimit int `json:"rate_limit"`
	}
	err = ms.supaClient.DB.From("profile").
		Select("rate_limit").
		Eq("user_id", userID).
		Execute(ctx, &profiles)
	if err != nil || len(profiles) == 0 {
		return fmt.Errorf("failed to fetch user's rate limit: %v", err)
	}

	// Check if rate_limit is greater than zero
	if profiles[0].RateLimit <= 0 {
		return fmt.Errorf("rate limit exceeded")
	}

	var services []Service
	err = ms.supaClient.DB.From("services").
		Select("*").
		Eq("service_id", req.ServiceID).
		Eq("user_id", userID). // Use resolved userID
		Execute(ctx, &services)
	if err != nil || len(services) == 0 {
		return fmt.Errorf("invalid service_id")
	}

	service := services[0]

	// Capture the Origin header from the request
	origin := r.Header.Get("Origin")
	log.Printf("Incoming request origin: %s", origin) // Added log for debugging

	// Parse the origin
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("invalid origin: %s", origin)
	}

	// Check if the request's origin is allowed
	if service.CorsOrigin != "" {
		allowedOrigins := strings.Split(service.CorsOrigin, ",")
		originAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			allowedOrigin = strings.TrimSpace(allowedOrigin)
			// Parse the allowed origin
			parsedAllowedOrigin, err := url.Parse(allowedOrigin)
			if err != nil {
				continue // Skip invalid allowed origins
			}
			// Compare scheme
			if parsedAllowedOrigin.Scheme != parsedOrigin.Scheme {
				continue
			}
			// Check if origin hostname is the same or a subdomain
			if parsedOrigin.Hostname() == parsedAllowedOrigin.Hostname() ||
				strings.HasSuffix(parsedOrigin.Hostname(), "."+parsedAllowedOrigin.Hostname()) {
				originAllowed = true
				break
			}
		}
		if !originAllowed {
			return fmt.Errorf("origin not allowed: %s", origin)
		}
	}

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
		Eq("user_id", userID). // Use resolved userID
		Execute(ctx, &templates)
	if err != nil || len(templates) == 0 {
		return fmt.Errorf("invalid template_id")
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

	// Attempt to send the email
	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", service.HostAddress, service.Port),
		auth,
		service.EmailID,
		[]string{recipient.EmailAddress},
		[]byte(message.String()),
	)

	// Decrement the rate_limit only if email was sent successfully
	if err == nil {
		updatedProfile := struct {
			RateLimit int `json:"rate_limit"`
		}{
			RateLimit: profiles[0].RateLimit - 1,
		}

		err = ms.supaClient.DB.From("profile").
			Update(updatedProfile).
			Eq("user_id", userID).
			Execute(ctx, nil)
		if err != nil {
			log.Printf("Failed to update user's rate limit: %v", err)
		}
	}

	// Create log entry regardless of success or failure
	logEntry := LogEntry{
		UserID:     userID,
		ServiceID:  req.ServiceID,
		TemplateID: req.TemplateID,
		Status:     "success",
		Message:    fmt.Sprintf("Email sent to %s", recipient.EmailAddress),
	}
	if err != nil {
		// If sending email failed, set status to "failed" and include the error message
		logEntry.Status = "failed"
		logEntry.Message = err.Error()
	}

	// Insert log entry and handle errors
	if insertErr := ms.supaClient.DB.From("logs").Insert(logEntry).Execute(ctx, nil); insertErr != nil {
		log.Printf("Failed to insert log entry: %v", insertErr)
	}

	if err != nil {
		return fmt.Errorf("email sending error: %v", err)
	}

	// Check if the email entry already exists
	var existingEmails []EmailEntry
	err = ms.supaClient.DB.From("emails").
		Select("*").
		Eq("user_id", userID).
		Eq("email_address", recipient.EmailAddress).
		Eq("template_id", req.TemplateID).
		Execute(ctx, &existingEmails)
	if err != nil {
		log.Printf("Failed to check existing emails: %v", err)
	} else if len(existingEmails) == 0 {
		// Create email entry since it doesn't exist
		emailEntry := EmailEntry{
			UserID:       userID,
			ServiceID:    req.ServiceID,
			TemplateID:   req.TemplateID,
			EmailAddress: recipient.EmailAddress,
			Name:         recipient.Name,
			PhoneNumber:  "", // Add PhoneNumber field to match the emails table schema
		}
		// Insert email entry and handle errors
		if insertErr := ms.supaClient.DB.From("emails").Insert(emailEntry).Execute(ctx, nil); insertErr != nil {
			log.Printf("Failed to insert email entry: %v", insertErr)
		}
	}

	return nil
}

func (ms *MailService) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	mailService, err := NewMailService()
	if err != nil {
		log.Fatalf("Failed to initialize mail service: %v", err)
	}

	r := mux.NewRouter()

	// Email sending endpoint
	r.HandleFunc("/send-emails", mailService.SendEmailsHandler).Methods("POST")

	// Health check endpoint
	r.HandleFunc("/health", mailService.HealthCheckHandler).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
