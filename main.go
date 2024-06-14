package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type probeArgs []string

func (p *probeArgs) Set(val string) error {
	*p = append(*p, val)
	return nil
}

func (p probeArgs) String() string {
	return strings.Join(p, ",")
}

// Define the Result type
type Result struct {
	URL           string `json:"url"`
	StatusCode    int    `json:"status_code"`
	RedirectedURL string `json:"redirected_url,omitempty"`
}

func main() {

	// concurrency flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level (split equally between HTTPS and HTTP requests)")

	// probe flags
	var probes probeArgs
	flag.Var(&probes, "p", "add additional probe (proto:port)")

	// skip default probes flag
	var skipDefault bool
	flag.BoolVar(&skipDefault, "s", false, "skip the default probes (http:80 and https:443)")

	// timeout flag
	var to int
	flag.IntVar(&to, "t", 10000, "timeout (milliseconds)")

	// prefer https
	var preferHTTPS bool
	flag.BoolVar(&preferHTTPS, "prefer-https", false, "only try plain HTTP if HTTPS fails")

	// HTTP method to use
	var method string
	flag.StringVar(&method, "method", "GET", "HTTP method to use")

	flag.Parse()

	// make an actual time.Duration out of the timeout
	timeout := time.Duration(to * 1000000)

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	// domain/port pairs are initially sent on the httpsURLs channel.
	// If they are listening and the --prefer-https flag is set then
	// no HTTP check is performed; otherwise they're put onto the httpURLs
	// channel for an HTTP check.
	httpsURLs := make(chan string)
	httpURLs := make(chan string)
	output := make(chan Result)

	// HTTPS workers
	var httpsWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpsWG.Add(1)

		go func() {
			for url := range httpsURLs {
				fmt.Printf("Probing HTTPS URL: %s\n", url)

				// always try HTTPS first
				withProto := "https://" + url
				statusCode, redirectedURL := getStatusAndRedirect(client, withProto, method)
				if statusCode != 0 {
					fmt.Printf("Found HTTPS URL: %s with status code: %d\n", withProto, statusCode)
					output <- Result{
						URL:           withProto,
						StatusCode:    statusCode,
						RedirectedURL: redirectedURL,
					}

					// skip trying HTTP if --prefer-https is set
					if preferHTTPS {
						continue
					}
				}

				httpURLs <- url
			}

			httpsWG.Done()
		}()
	}

	// HTTP workers
	var httpWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpWG.Add(1)

		go func() {
			for url := range httpURLs {
				fmt.Printf("Probing HTTP URL: %s\n", url)

				withProto := "http://" + url
				statusCode, redirectedURL := getStatusAndRedirect(client, withProto, method)
				if statusCode != 0 {
					fmt.Printf("Found HTTP URL: %s with status code: %d\n", withProto, statusCode)
					output <- Result{
						URL:           withProto,
						StatusCode:    statusCode,
						RedirectedURL: redirectedURL,
					}
					continue
				}
			}

			httpWG.Done()
		}()
	}

	// Close the httpURLs channel when the HTTPS workers are done
	go func() {
		httpsWG.Wait()
		close(httpURLs)
	}()

	// Collect results into a map grouped by status code
	results := make(map[int][]Result)

	// Output worker
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go func() {
		for res := range output {
			results[res.StatusCode] = append(results[res.StatusCode], res)
		}
		outputWG.Done()
	}()

	// Close the output channel when the HTTP workers are done
	go func() {
		httpWG.Wait()
		close(output)
	}()

	// accept domains on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		domain := strings.ToLower(sc.Text())
		fmt.Printf("Processing domain: %s\n", domain)

		// submit standard port checks
		if (!skipDefault) && (len(probes) == 0) {
			httpsURLs <- domain
			httpURLs <- domain
		} else {
			// submit any additional proto:port probes
			for _, p := range probes {
				switch p {
				case "xlarge":
					// Adding port templates
					xlarge := []string{"81", "300", "591", "593", "832", "981", "1010", "1311", "2082", "2087", "2095", "2096", "2480", "3000", "3128", "3333", "4243", "4567", "4711", "4712", "4993", "5000", "5104", "5108", "5800", "6543", "7000", "7396", "7474", "8000", "8001", "8008", "8014", "8042", "8069", "8080", "8081", "8088", "8090", "8091", "8118", "8123", "8172", "8222", "8243", "8280", "8281", "8333", "8443", "8500", "8834", "8880", "8888", "8983", "9000", "9043", "9060", "9080", "9090", "9091", "9200", "9443", "9800", "9981", "12443", "16080", "18091", "18092", "20720", "28017"}
					for _, port := range xlarge {
						httpsURLs <- fmt.Sprintf("%s:%s", domain, port)
						httpURLs <- fmt.Sprintf("%s:%s", domain, port)
					}
				case "large":
					large := []string{"81", "591", "2082", "2087", "2095", "2096", "3000", "8000", "8001", "8008", "8080", "8083", "8443", "8834", "8888"}
					for _, port := range large {
						httpsURLs <- fmt.Sprintf("%s:%s", domain, port)
						httpURLs <- fmt.Sprintf("%s:%s", domain, port)
					}
				default:
					pair := strings.SplitN(p, ":", 2)
					if len(pair) != 2 {
						continue
					}

					// This is a little bit funny as "https" will imply an
					// http check as well unless the --prefer-https flag is
					// set. On balance I don't think that's *such* a bad thing
					// but it is maybe a little unexpected.
					if strings.ToLower(pair[0]) == "https" {
						httpsURLs <- fmt.Sprintf("%s:%s", domain, pair[1])
					} else {
						httpURLs <- fmt.Sprintf("%s:%s", domain, pair[1])
					}
				}
			}
		}
	}

	// once we've sent all the URLs off we can close the
	// input/httpsURLs channel. The workers will finish what they're
	// doing and then call 'Done' on the WaitGroup
	close(httpsURLs)

	// check there were no errors reading stdin (unlikely)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until the output waitgroup is done
	outputWG.Wait()

	// Generate JSON file name
	jsonFileName := generateJSONFileName()
	jsonOutputFilePath, _ := filepath.Abs(jsonFileName)

	// Display the message in the terminal that we are creating the JSON file
	fmt.Println("Creating JSON file...")

	// Create and write to the JSON file
	writeJSONFile(jsonOutputFilePath, results)
}

// Generate a unique JSON file name based on the current date and time
func generateJSONFileName() string {
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_scan.json", timestamp)
}

func writeJSONFile(jsonOutputFilePath string, results map[int][]Result) {
	jsonData := make(map[string]interface{})

	for statusCode, resList := range results {
		codeStr := fmt.Sprintf("%d", statusCode)
		var entries []interface{}

		for _, res := range resList {
			if statusCode >= 300 && statusCode < 400 {
				// For 3xx status codes, include redirection information with URL first
				entry := struct {
					URL           string `json:"url"`
					RedirectedURL string `json:"redirected_url"`
				}{
					URL:           res.URL,
					RedirectedURL: res.RedirectedURL,
				}
				entries = append(entries, entry)
			} else {
				// For other status codes, just include the URL
				entries = append(entries, res.URL)
			}
		}

		jsonData[codeStr] = entries
	}

	jsonFile, err := os.Create(jsonOutputFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create JSON file: %s\n", err)
		return
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(jsonData); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write to JSON file: %s\n", err)
		return
	}

	fmt.Printf("JSON file created successfully at: %s\n", jsonOutputFilePath)
}


func getStatusAndRedirect(client *http.Client, url, method string) (int, string) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, ""
	}

	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
	}

	if err != nil {
		return 0, ""
	}

	redirectedURL := ""
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectedURL = resp.Header.Get("Location")
	}

	return resp.StatusCode, redirectedURL
}

