package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

func main() {
	concurrencyPtr := flag.Int("t", 8, "Number of threads to utilize. Default is 8.")
	timeoutPtr := flag.Int("timeout", 8, "Timeout for each request in seconds. Default is 8 seconds.")
	retryPtr := flag.Int("retry", 3, "Number of retries for failed requests. Default is 3.")
	retrySleepPtr := flag.Int("retry-sleep", 1, "Sleep duration between retries in seconds. Default is 1 second.")
	flag.Parse()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			DialContext:         (&net.Dialer{Timeout: time.Duration(*timeoutPtr) * time.Second}).DialContext,
			TLSHandshakeTimeout: time.Duration(*timeoutPtr) * time.Second,
		},
	}

	fmt.Println("Please type or paste the URLs:")

	work := make(chan string)
	urlCount := 0

	go func() {
		s := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("Input URL: ")
			if !s.Scan() {
				break
			}
			urlText := s.Text()
			if urlText == "" {
				continue
			}
			if _, err := url.ParseRequestURI(urlText); err != nil {
				fmt.Printf("Invalid URL: %s\n", urlText)
				continue
			}
			work <- urlText
			urlCount++
		}
		if s.Err() != nil {
			log.Printf("Error reading from stdin: %v\n", s.Err())
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < *concurrencyPtr; i++ {
		wg.Add(1)
		go doWork(work, client, wg, *retryPtr, *retrySleepPtr)
	}
	wg.Wait()

	// Check if URLs were provided
	if urlCount == 0 {
		fmt.Println("No valid URLs provided. Exiting.")
	} else {
		// Wait for user input before closing
		fmt.Println("Press Enter to exit...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}

func doWork(work chan string, client *http.Client, wg *sync.WaitGroup, retries int, retrySleep int) {
	defer wg.Done()
	for urlText := range work {
		var resp *http.Response
		var err error

		// Retry loop
		for attempts := 0; attempts <= retries; attempts++ {
			req, reqErr := http.NewRequest("GET", urlText, nil)
			if reqErr != nil {
				log.Printf("Error creating request: %v, URL: %s\n", reqErr, urlText)
				break
			}
			req.Header.Set("Connection", "close")

			resp, err = client.Do(req)
			if err == nil || attempts == retries {
				if err != nil {
					log.Printf("Request failed: %v, URL: %s\n", err, urlText)
				} else {
					// Print success status code 5 times
					for i := 0; i < 5; i++ {
						log.Printf("Response status: %d, URL: %s\n", resp.StatusCode, urlText)
					}
					resp.Body.Close()
				}
				break
			}

			// Log retry attempt and sleep before retrying
			log.Printf("Retry %d for URL: %s\n", attempts+1, urlText)
			time.Sleep(time.Duration(retrySleep) * time.Second)
		}
	}
}
