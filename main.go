package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// envOrDefault reads an environment variable, falling back to a default
// value for local development when it isn't set.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

type TicketApplication struct {
	ID            int     `json:"id"`
	UserEmail     string  `json:"user_email"`
	UserName      string  `json:"user_name"`
	Phone         string  `json:"phone"`
	Destination   string  `json:"destination"`
	TransportType string  `json:"transport_type"`
	Date          string  `json:"date"`
	Time          string  `json:"time"`
	QuotedPrice   float64 `json:"quoted_price"`
	Status        string  `json:"status"` // "Pending Price", "Quoted", "Paid & Ticket Issued"
	SeatNumber    string  `json:"seat_number"`
	AppliedAt     string  `json:"applied_at"`
}

type PricingRule struct {
	Destination   string  `json:"destination"`
	TransportType string  `json:"transport_type"`
	Price         float64 `json:"price"`
}

var (
	// In-memory data structures
	usersMutex sync.Mutex
	users      = map[string]User{
		"traveler@example.com": {
			Name:     "John Doe",
			Email:    "traveler@example.com",
			Password: "user123",
			Phone:    "+2348000000000",
		},
	}

	applicationsMutex sync.Mutex
	applications      = []TicketApplication{}
	appSeqID          = 1001

	pricingMutex sync.Mutex
	pricingRules = map[string]float64{
		"Lagos - Air":   120.00,
		"Abuja - Air":   150.00,
		"Lagos - Road":  30.00,
		"Abuja - Road":  40.00,
		"Calabar - Sea": 80.00,
	}

	// Admin credentials — read from env vars in production, fall back to
	// the old hardcoded values for local `go run .`.
	adminUsername = envOrDefault("ADMIN_USERNAME", "admin")
	adminPassword = envOrDefault("ADMIN_PASSWORD", "adminpassword123")

	// SMTP credentials for sending ticket confirmation emails.
	smtpFrom     = envOrDefault("SMTP_EMAIL", "your-email@gmail.com")
	smtpPassword = envOrDefault("SMTP_APP_PASSWORD", "your-app-password")
	smtpHost     = envOrDefault("SMTP_HOST", "smtp.gmail.com")
	smtpPort     = envOrDefault("SMTP_PORT", "587")

	// Flutterwave PUBLIC key — this one is safe to expose to the browser
	// (it's a publishable key by design), but keeping it in an env var
	// means you don't have to touch code to swap test/live keys.
	flutterwavePublicKey = envOrDefault("FLUTTERWAVE_PUBLIC_KEY", "FLWPUBK_TEST-YOUR_PUBLIC_KEY_HERE-X")
)

func main() {
	r := gin.Default()

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// Public & HTML Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Single login page — handles BOTH customer and admin sign-in.
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})

	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", nil)
	})

	r.GET("/book", func(c *gin.Context) {
		transportType := c.Query("type")
		if transportType == "" {
			transportType = "road"
		}
		c.HTML(http.StatusOK, "book.html", gin.H{
			"Type":                 transportType,
			"FlutterwavePublicKey": flutterwavePublicKey,
		})
	})

	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin.html", nil)
	})

	// Auth APIs
	r.POST("/api/auth/register", handleUserRegister)
	// Unified login: checks admin credentials first, then falls back to customer accounts.
	r.POST("/api/auth/login", handleLogin)

	// Application & Price APIs
	r.POST("/api/submit-application", handleSubmitApplication)
	r.GET("/api/get-price", handleGetPrice)
	r.POST("/api/confirm-booking", handleBookingConfirmation)

	// Admin Dashboard Data APIs
	r.GET("/api/admin/applications", getAdminApplications)
	r.POST("/api/admin/quote-price", quoteApplicationPrice)
	r.GET("/api/admin/pricing", getPricingRules)
	r.POST("/api/admin/pricing", updatePricingRule) // also used to EDIT an existing price tag (upsert by key)

	// Render (and most hosts) assign the port dynamically via $PORT —
	// the app MUST listen on that, not a hardcoded port, or the deploy
	// will fail health checks.
	port := envOrDefault("PORT", "8080")
	fmt.Printf("ASAA Travel Server running on http://localhost:%s\n", port)
	r.Run(":" + port)
}

// User Register Handler
func handleUserRegister(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid registration input"})
		return
	}

	usersMutex.Lock()
	if _, exists := users[user.Email]; exists {
		usersMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "Account with this email already exists"})
		return
	}

	users[user.Email] = user
	usersMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Registration successful. Please log in."})
}

// Unified Login Handler — one endpoint, one form, two possible roles.
// Tries the admin credentials first (identifier = "admin" username), then
// falls back to looking the identifier up as a customer email.
func handleLogin(c *gin.Context) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 1. Admin check
	if payload.Email == adminUsername && payload.Password == adminPassword {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"role":    "admin",
			"message": "Admin authenticated",
		})
		return
	}

	// 2. Customer check
	usersMutex.Lock()
	user, exists := users[payload.Email]
	usersMutex.Unlock()

	if !exists || user.Password != payload.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email/username or password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"role":   "customer",
		"user": gin.H{
			"name":  user.Name,
			"email": user.Email,
			"phone": user.Phone,
		},
	})
}

// Submit Application Handler
func handleSubmitApplication(c *gin.Context) {
	var req TicketApplication
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application details"})
		return
	}

	applicationsMutex.Lock()
	req.ID = appSeqID
	appSeqID++
	req.AppliedAt = time.Now().Format("2006-01-02 15:04")

	// Determine route price tag
	pricingKey := fmt.Sprintf("%s - %s", req.Destination, capitalize(req.TransportType))
	pricingMutex.Lock()
	if price, ok := pricingRules[pricingKey]; ok {
		req.QuotedPrice = price
		req.Status = "Quoted"
	} else {
		req.QuotedPrice = 0.0
		req.Status = "Pending Admin Price"
	}
	pricingMutex.Unlock()

	applications = append(applications, req)
	applicationsMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"application_id": req.ID,
		"quoted_price":   req.QuotedPrice,
		"review_status":  req.Status,
		"message":        "Application submitted successfully.",
	})
}

// Get Destination Price Handler
func handleGetPrice(c *gin.Context) {
	dest := c.Query("destination")
	transportType := c.Query("type")

	key := fmt.Sprintf("%s - %s", dest, capitalize(transportType))

	pricingMutex.Lock()
	price, exists := pricingRules[key]
	pricingMutex.Unlock()

	if exists {
		c.JSON(http.StatusOK, gin.H{"status": "available", "price": price})
	} else {
		c.JSON(http.StatusOK, gin.H{"status": "pending_admin", "price": 0.0})
	}
}

// Confirm Booking & Issue Ticket
func handleBookingConfirmation(c *gin.Context) {
	var payload struct {
		ApplicationID int     `json:"application_id"`
		TransactionID int     `json:"transaction_id"`
		TxRef         string  `json:"tx_ref"`
		Amount        float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking confirmation parameters"})
		return
	}

	rand.Seed(time.Now().UnixNano())
	seatNumber := fmt.Sprintf("Seat-%d%c", rand.Intn(40)+1, 'A'+rand.Intn(4))

	applicationsMutex.Lock()
	var updatedApp *TicketApplication
	for i := range applications {
		if applications[i].ID == payload.ApplicationID {
			applications[i].Status = "Paid & Ticket Issued"
			applications[i].SeatNumber = seatNumber
			updatedApp = &applications[i]
			break
		}
	}
	applicationsMutex.Unlock()

	if updatedApp != nil {
		go sendTicketEmail(updatedApp.UserEmail, updatedApp.UserName, updatedApp.TransportType, updatedApp.Destination, updatedApp.Date, updatedApp.Time, seatNumber, payload.Amount)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"seat_number": seatNumber,
		"message":     "Booking confirmed, application status updated, and ticket dispatched.",
	})
}

// Get All User Applications for Admin Track Record
func getAdminApplications(c *gin.Context) {
	applicationsMutex.Lock()
	defer applicationsMutex.Unlock()
	c.JSON(http.StatusOK, applications)
}

// Admin Quote Price Handler
func quoteApplicationPrice(c *gin.Context) {
	var payload struct {
		ID    int     `json:"id"`
		Price float64 `json:"price"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	applicationsMutex.Lock()
	defer applicationsMutex.Unlock()

	for i := range applications {
		if applications[i].ID == payload.ID {
			applications[i].QuotedPrice = payload.Price
			applications[i].Status = "Quoted by Admin"
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Price quoted successfully"})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Application record not found"})
}

// Get Pricing Rules Handler
func getPricingRules(c *gin.Context) {
	pricingMutex.Lock()
	defer pricingMutex.Unlock()
	c.JSON(http.StatusOK, pricingRules)
}

// Update Pricing Rule Handler.
// Keyed by "Destination - Mode", so posting the SAME destination+mode again
// simply overwrites the existing price — this is how the admin dashboard
// both creates NEW price tags and EDITS existing ones.
func updatePricingRule(c *gin.Context) {
	var rule PricingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	key := fmt.Sprintf("%s - %s", rule.Destination, rule.TransportType)
	pricingMutex.Lock()
	pricingRules[key] = rule.Price
	pricingMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pricing set successfully"})
}

func sendTicketEmail(toEmail, name, transportType, dest, date, timeStr, seatNumber string, amount float64) {
	subject := "Subject: Your ASAA Travel Application & Ticket Confirmation\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	body := fmt.Sprintf(`
		<h2>Ticket Application Confirmed - ASAA Travel</h2>
		<p>Hello <strong>%s</strong>,</p>
		<p>Your travel ticket application has been successfully paid and processed!</p>
		<hr>
		<h3>Application Details:</h3>
		<ul>
			<li><strong>Traveler Name:</strong> %s</li>
			<li><strong>Transport Mode:</strong> %s</li>
			<li><strong>Destination:</strong> %s</li>
			<li><strong>Travel Date:</strong> %s</li>
			<li><strong>Departure Time:</strong> %s</li>
			<li><strong>Total Fare Paid:</strong> USD %.2f</li>
			<li><strong>Assigned Seat Number:</strong> %s</li>
		</ul>
		<p>Please present this confirmation email at departure.</p>
		<p>Safe Travels,<br>ASAA Travel Team</p>
	`, name, name, transportType, dest, date, timeStr, amount, seatNumber)

	msg := []byte(subject + mime + body)
	auth := smtp.PlainAuth("", smtpFrom, smtpPassword, smtpHost)

	if err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpFrom, []string{toEmail}, msg); err != nil {
		fmt.Printf("failed to send ticket email to %s: %v\n", toEmail, err)
	}
}

func capitalize(str string) string {
	if len(str) == 0 {
		return str
	}
	if str == "air" {
		return "Air"
	} else if str == "road" {
		return "Road"
	} else if str == "sea" {
		return "Sea"
	}
	return str
}
