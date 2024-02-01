package main

import (
	"database/sql"
	"net/http"
	"strings"
	"fmt"
	"io"
	"os"

	"go.elastic.co/apm/module/apmhttp/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/go-sql-driver/mysql"
)

// Define the structure of your data
type Data struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"` // This will be masked in the output
	Email   string `json:"email"`   // This will be masked in the output
}

// RequestData defines the structure for incoming JSON payload on the POST request
type RequestData struct {
	ID int `json:"id"`
}


func apmMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		handler := apmhttp.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.SetRequest(r)
			err := next(c)
			if err != nil {
				c.Error(err)
			}
		}))

		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}


func maskData(data string) string {
	// Mask the data except for the domain part of the email and the last 4 chars of the address
	var masked string
	if strings.Contains(data, "@") { // Simple check to assume it's an email
		parts := strings.Split(data, "@")
		if len(parts[0]) > 1 {
			masked = strings.Repeat("*", len(parts[0])-1) + parts[0][len(parts[0])-1:]
		} else {
			masked = "*"
		}
		masked += "@" + parts[1]
	} else { // Assume it's an address otherwise
		if len(data) > 4 {
			masked = strings.Repeat("*", len(data)-4) + data[len(data)-4:]
		} else {
			masked = strings.Repeat("*", len(data))
		}
	}
	return masked
}
func getAddress(c echo.Context) error {
	requestData := new(RequestData)
	if err := c.Bind(requestData); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid input")
	}

	db, err := sql.Open("mysql", "root:my-secret-pw@tcp(localhost:3306)/pii_data")
	if err != nil {
		return err
	}
	defer db.Close()

	// Prepare the SQL statement for selecting the user's address
	stmt, err := db.Prepare("SELECT address FROM users WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	var address string
	err = stmt.QueryRow(requestData.ID).Scan(&address)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("User with ID %d not found", requestData.ID))
		}
		return err
	}

	maskedAddress := maskData(address)
	return c.JSON(http.StatusOK, map[string]string{"address": maskedAddress})
}

func logRequestBody(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read request body")
		}

		// Log the request body
		fmt.Printf("Request Body: %s\n", string(body))

		// Restore the body to its original state
		req.Body = io.NopCloser(strings.NewReader(string(body)))

		// Call the next handler
		return next(c)
	}
}

func main() {
	os.Setenv("ELASTIC_APM_SERVICE_NAME", "api-masking")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "NotMyPassword")
	os.Setenv("ELASTIC_APM_SERVER_URL", "https://48ad4f8dab13werid-url3273401c565c.apm.us-central1.gcp.cloud.es.io:443")
	os.Setenv("ELASTIC_APM_ENVIRONMENT", "my-development")

        e := echo.New()
	e.Use(apmMiddleware)

	e.Use(logRequestBody)

	// Add Logger middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}, latency=${latency_human}\n",
	}))

	// Define the HTTP route for retrieving address info based on id using POST method
	e.POST("/get-address", getAddress)
	
	// Start the server on port 8080
	e.Logger.Fatal(e.Start(":8080"))

}

