package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

const version = "0.1.0"

func printHelp() {
	fmt.Println("RTTea v" + version + " — proxy RTT measurement tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  rttea --target <ip> --proxy <proxy> [--user <user> --pass <password>]")
	fmt.Println()
	fmt.Println("Required flags:")
	fmt.Println("  --target    Target URL or IP to reach through the proxy")
	fmt.Println("  --proxy     Proxy address (e.g. http://1.2.3.4:8080)")
	fmt.Println()
	fmt.Println("Optional flags:")
	fmt.Println("  --user      Proxy authentication username")
	fmt.Println("  --pass      Proxy authentication password")
	fmt.Println("  --count     Number of requests to send (default: 1)")
	fmt.Println("  --timeout   Request timeout in seconds (default: 10)")
	fmt.Println("  --help      Show this help message")
	fmt.Println("  --version   Show version")
	fmt.Println()
	fmt.Println("Output:")
	fmt.Println("  Prints time_connect and time_total for each request in seconds.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  rttea --target http://example.com --proxy http://proxy:8080")
	fmt.Println("  rttea --target http://93.184.216.34 --proxy http://proxy:8080 --user admin --pass secret")
	fmt.Println("  rttea --target http://example.com --proxy http://proxy:8080 --count 5")
}

func main() {
	target := flag.String("target", "", "Target URL or IP")
	proxy := flag.String("proxy", "", "Proxy address")
	user := flag.String("user", "", "Proxy auth username")
	pass := flag.String("pass", "", "Proxy auth password")
	count := flag.Int("count", 1, "Number of requests")
	timeout := flag.Int("timeout", 10, "Request timeout in seconds")
	showHelp := flag.Bool("help", false, "Show help")
	showVersion := flag.Bool("version", false, "Show version")

	flag.Parse()

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Println("RTTea v" + version)
		os.Exit(0)
	}

	if *target == "" || *proxy == "" {
		fmt.Fprintln(os.Stderr, "Error: --target and --proxy are required.")
		fmt.Fprintln(os.Stderr, "Run with --help for usage information.")
		os.Exit(1)
	}

	// Normalise target: if no scheme, prepend http://
	targetURL := *target
	if !hasScheme(targetURL) {
		targetURL = "http://" + targetURL
	}

	// Build proxy URL (with optional auth)
	proxyStr := *proxy
	if !hasScheme(proxyStr) {
		proxyStr = "http://" + proxyStr
	}
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid proxy URL %q: %v\n", *proxy, err)
		os.Exit(1)
	}
	if *user != "" {
		proxyURL.User = url.UserPassword(*user, *pass)
	}

	timeoutDur := time.Duration(*timeout) * time.Second

	if *count < 1 {
		fmt.Fprintln(os.Stderr, "Error: --count must be >= 1")
		os.Exit(1)
	}

	fmt.Printf("RTTea v%s\n", version)
	fmt.Printf("Target: %s | Proxy: %s | Requests: %d\n", targetURL, proxyURL.Redacted(), *count)
	fmt.Println("---")

	var totalConnect, totalTotal time.Duration

	for i := 1; i <= *count; i++ {
		connectTime, totalTime, statusCode, reqErr := measure(targetURL, proxyURL, timeoutDur)
		if reqErr != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] Error: %v\n", i, *count, reqErr)
			continue
		}

		totalConnect += connectTime
		totalTotal += totalTime

		fmt.Printf("[%d/%d] status=%d  time_connect=%.4fs  time_total=%.4fs\n",
			i, *count, statusCode,
			connectTime.Seconds(), totalTime.Seconds())
	}

	if *count > 1 {
		fmt.Println("---")
		fmt.Printf("Average  time_connect=%.4fs  time_total=%.4fs\n",
			totalConnect.Seconds()/float64(*count),
			totalTotal.Seconds()/float64(*count))
	}
}

// measure performs a single HTTP GET through the proxy and returns timing data.
func measure(targetURL string, proxyURL *url.URL, timeout time.Duration) (connectTime, totalTime time.Duration, statusCode int, err error) {
	var connStart, connDone time.Time

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			connStart = time.Now()
			conn, dialErr := dialer.DialContext(ctx, network, addr)
			if dialErr == nil {
				connDone = time.Now()
			}
			return conn, dialErr
		},
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: false},
		DisableKeepAlives:  true,
		ForceAttemptHTTP2:  false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	reqStart := time.Now()
	resp, err := client.Get(targetURL)
	totalTime = time.Since(reqStart)

	if err != nil {
		return 0, totalTime, 0, err
	}
	defer resp.Body.Close()

	if !connDone.IsZero() {
		connectTime = connDone.Sub(connStart)
	}

	return connectTime, totalTime, resp.StatusCode, nil
}

// hasScheme checks if a URL string already has a scheme prefix.
func hasScheme(u string) bool {
	for i, c := range u {
		if c == ':' && i > 0 {
			return true
		}
		if c == '/' || c == '?' || c == '#' {
			return false
		}
	}
	return false
}
