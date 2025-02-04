package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Configuration
var (
	N                = flag.Int("iteration", 10, "number of request to be made")
	create           = flag.Bool("create", false, "falg to trigger create")
	validate         = flag.Bool("validate", true, "falg to trigger create")
	mobileSession    = flag.Bool("m", true, "create mobile sessions")
	desktopSession   = flag.Bool("d", true, "create desktop sessions")
	host             = flag.String("h", "http://localhost:8081", "pass the host with protocol and without trailing slash")
	email            = flag.String("e", "sriram.rajendran+%d@freshworks.com", "pass the email in double quotes")
	xsrf             = flag.String("xsrf", "2345", "pass the xsrf token alone")
	password         = flag.String("p", "qwerty12", "pass the password")
	mobileUserAgents = []string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0.1 Mobile/15E148 Safari/604.",
		"Mozilla/5.0 (iPad; CPU OS 17_7_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 10; ONEPLUS A6003) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.135 Mobile Safari/537.36 EdgA/131.0.2903.87",
		"Mozilla/5.0 (Linux; Android 10; VOG-L29) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.135 Mobile Safari/537.36 OPR/76.2.4027.73374",
	}
	desktopUserAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 OPR/115.0.0.",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.102 Safari/537.36 Edge/18.1958",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.1",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.3",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115",
	}

	lenMobileUserAgent  = len(mobileUserAgents)
	lenDesktopUserAgent = len(desktopUserAgents)
	mobile, desktop     = 0, 0
)

const (
	xsrfCookiePrefix = "XSRF-TOKEN"
	LoginURL         = "/api/v2/login"
	ValidateURL      = "/api/v2/users/current"
	MCookieFile      = "./MobileCookies.txt"
	DCookieFile      = "./DesktopCookies.txt"
	ThrottleDelay    = 50 * time.Millisecond // Delay between API calls
	cookieName       = "_d"
	MAXEMAIL         = 100
)

func getUserAgent() (string, string) {
	n := rand.IntN(lenDesktopUserAgent + lenMobileUserAgent)
	if n%2 == 0 {
		mobile++
		return mobileUserAgents[n/2], "mobile"

	} else {
		desktop++
		return desktopUserAgents[n/2], "desktop"
	}
}

// createSession creates a session, stores only the cookie value in a file
func createSession(wg *sync.WaitGroup, Mfile, Dfile *os.File) {
	defer wg.Done()

	reqBody, _ := json.Marshal(map[string]string{
		"username": getUserName(),
		"password": *password,
	})
	req, err := http.NewRequest("POST", *host+LoginURL, bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Add("Cookie", xsrfCookiePrefix+"="+*xsrf)
	req.Header.Add("X-XSRF-TOKEN", *xsrf)
	ua, t := getUserAgent()
	req.Header.Add("User-Agent", ua)
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error during login:", err)
		return
	}
	defer resp.Body.Close()

	// Write only cookies to the cookie file if the login is successful
	if resp.StatusCode == 200 {

		for _, cookie := range resp.Cookies() {
			if cookie.Name == cookieName {
				if t == "mobile" {
					_, err = Mfile.WriteString("\n" + cookie.Value)
				} else {
					_, err = Dfile.WriteString("\n" + cookie.Value)
				}

			}

			if err != nil {
				fmt.Println("Error writing cookie value to file:", err)
			} else {
				// fmt.Printf("Session %d created and cookie value stored\n", sessionID)
			}
		}

	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to create session %v, %v\n", string(body), string(reqBody))
	}
}

// validateSession validates a session using the stored cookie values
func validateSession(cookie string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Use the session's cookie value for validation
	client := &http.Client{}
	req, err := http.NewRequest("GET", *host+ValidateURL, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Add the cookie value to the request
	req.Header.Add("Cookie", xsrfCookiePrefix+"="+*xsrf+";"+cookieName+"="+cookie)
	req.Header.Add("X-XSRF-TOKEN", *xsrf)

	// Validate the session
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error during validation:", err)
		return
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode == 200 {
		// fmt.Printf("Session %v is valid\n", cookie)
	} else {
		fmt.Printf("Session %v is invalid. Status code: %v\n", cookie, resp.StatusCode)
	}
}

func main() {
	flag.Parse()
	Mfile := createAndOpenFile(MCookieFile)
	Dfile := createAndOpenFile(DCookieFile)
	defer Mfile.Close()
	defer Dfile.Close()

	var wg sync.WaitGroup

	if *create {
		Dfile.Truncate(0)
		Mfile.Truncate(0)
		for i := 1; i <= *N; i++ {
			time.Sleep(ThrottleDelay)
			wg.Add(1)
			go createSession(&wg, Mfile, Dfile)

		}
	}

	wg.Wait()

	if *validate {
		fmt.Println("validating Mobile sessions")
		validateBasedOnFile(MCookieFile, &wg)
		fmt.Println("validating Desktop sessions")
		validateBasedOnFile(DCookieFile, &wg)
	}

	wg.Wait()

	fmt.Println("All sessions processed.")
	fmt.Println("desktop=", desktop, " mobile=", mobile)
}

func getUserName() string {

	return fmt.Sprintf(*email, rand.IntN(MAXEMAIL))
}

func createAndOpenFile(CookieFile string) *os.File {
	// Create the cookie file if it doesn't exist
	if _, err := os.Stat(CookieFile); os.IsNotExist(err) {
		_, err := os.Create(CookieFile)
		if err != nil {
			fmt.Println("Error creating cookie file:", err)
			panic("Aiyaao")
		}
	}

	// Open the file in append mode
	file, err := os.OpenFile(CookieFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		panic("Aiyaao open pana mudila")
	}

	return file
}

func validateBasedOnFile(CookieFile string, wg *sync.WaitGroup) {
	// Read cookie values from the file
	data, err := os.ReadFile(CookieFile)
	if err != nil {
		fmt.Printf("Error reading cookie file for session %v\n", err)
		return
	}

	cookieValues := strings.Split(string(data), "\n")

	for i := 0; i < len(cookieValues); i++ {
		time.Sleep(ThrottleDelay)
		if c := cookieValues[i]; c != "" {
			wg.Add(1)
			go validateSession(c, wg)
		}
	}
}
