// The slam tool hits an HTTP/2 server with a lot of load over a single TCP connection.
//
// Run with GODEBUG=http2debug=1 or =2 to see debug info.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

var (
	host        = flag.String("host", "", "hostname to hit")
	path        = flag.String("path", "/image/jpeg", "path to hit on server")
	variant     = flag.String("variant", "single-conn", "roundtripper to use: default or single-conn")
	httpVersion = flag.String("http-version", "2", "http version to use: 1 or 2")
	sleep       = flag.Int("sleep", 15, "how long to sleep between the two requests")
)

var hc *http.Client

func main() {
	flag.Parse()
	if *host == "" {
		log.Fatalf("missing required --host flag")
	}

	if *httpVersion != "1" && *httpVersion != "2" {
		log.Fatalf("invalid http version: %q", *httpVersion)
	}

	switch *variant {
	case "single-conn":
		log.Printf("Using single-conn variant")
		setupSingleConn()
	case "default":
		log.Printf("Setting up default roundtripper")
		setupDefaultRoundtripper()
	default:
		log.Fatalf("unknown variant %q", *variant)
	}

	log.Printf("Performing first request")
	singleRequest()

	sleepTime := time.Duration(*sleep) * time.Second
	log.Printf("Now sleeping for %v", sleepTime)
	time.Sleep(sleepTime)

	log.Printf("Performing second request")
	singleRequest()
}

func setupSingleConn() {
	var hostport string
	if strings.Contains(*host, ":") {
		hostport = *host
	} else {
		hostport = net.JoinHostPort(*host, "443")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	if *httpVersion == "2" {
		tlsConfig.NextProtos = []string{http2.NextProtoTLS}
	}
	c, err := tls.Dial("tcp", hostport, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	tr := &http2.Transport{}
	cc, err := tr.NewClientConn(c)
	if err != nil {
		log.Fatal(err)
	}
	hc = &http.Client{Transport: cc}
}

func setupDefaultRoundtripper() {
	hc = &http.Client{}
	hc.Transport = &http.Transport{
		ForceAttemptHTTP2: *httpVersion == "2",
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
	}
}

func singleRequest() {
	url := fmt.Sprintf("https://%s%s", *host, *path)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("user-agent", "h2slam")
	// if *httpVersion == "1" {
	// 	req.Header.Set("connection", "keep-alive")
	// }

	res, err := hc.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if *httpVersion == "1" {
		if res.ProtoMajor != 1 {
			panic("not 1")
		}
	} else if *httpVersion == "2" {
		if res.ProtoMajor != 2 {
			panic("not 2")
		}
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	res.Body.Close()
	fmt.Println(len(body))
}
