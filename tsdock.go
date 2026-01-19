// reverse proxy server using tsnet to reach Docker socket over Tailscale
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"tailscale.com/tsnet"
)

func main() {
	// targetURL := flag.String("url", "https://httpbin.org/", "target URL to proxy to")
	tsHost := flag.String("tshost", "", "Tailscale hostname")
	// insecure := flag.Bool("k", false, "allow insecure upstream certificates")
	// flag.BoolVar(insecure, "insecure", false, "allow insecure upstream certificates")
	flag.Parse()

	// target, err := url.Parse(*targetURL)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// calculate base64 of the target host for use in the tsnet hostname or tsnet config dir
	// targetB64 := base64.RawURLEncoding.EncodeToString([]byte(*tsHost))

	// calculate random 6-digit hex string
	randomBytes := make([]byte, 3)
	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Fatalf("failed to generate random string: %v", err)
	}
	randomStr := fmt.Sprintf("%x", randomBytes)

	if *tsHost == "" {
		calculatedTsHost := fmt.Sprintf("tsdock-%s", randomStr)
		tsHost = &calculatedTsHost
	}

	s := &tsnet.Server{
		Dir:       fmt.Sprintf("./tsdock-config-%s", randomStr),
		Hostname:  *tsHost,
		Ephemeral: true,
	}
	defer s.Close()

	// wait until server is up
	fmt.Println("bringing server up")
	status, err := s.Up(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("server status: %+v\n", status)

	fmt.Printf("about to listen on %s", *tsHost)
	ln, err := s.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// fmt.Printf("Listening on https://%v\n", s.CertDomains()[0])

	//proxy := httputil.NewSingleHostReverseProxy(target)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// path := req.URL.Path
			fmt.Printf("Proxying request for %s on %s\n", req.URL.Path, *tsHost)
			fmt.Printf("Request headers: %v\n", req.Header)
			fmt.Printf("Host requested: %s -> %s\n", req.Host, *tsHost)
			// req.URL, _ = url.Parse("http://unix/socket")
			// req.URL.Path = path
			req.Host = "docker"
			req.Header.Set("Host", "docker")
			req.URL.Scheme = "http"
			req.URL.Host = "docker"
		},
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
				// unix:///Users/mkoldov/.docker/run/docker.sock
				// return net.Dial("unix", "unix:///Users/mkoldov/.docker/run/docker.sock")
			},
		},
		// Rewrite: func(r *httputil.ProxyRequest) {
		// 	// r.SetURL(target)
		// 	fmt.Printf("Proxying request for %s on %s\n", r.In.URL.Path, *tsHost)
		// 	//r.Out.Host = target.Host
		// 	fmt.Printf("Request headers: %v\n", r.In.Header)
		// 	fmt.Printf("Host requested: %s -> %s\n", r.In.Host, *tsHost)
		// },
	}

	// Configure insecure TLS if -k/--insecure flag is set
	// if *insecure {
	// 	proxy.Transport = &http.Transport{
	// 		TLSClientConfig: &tls.Config{
	// 			InsecureSkipVerify: true,
	// 		},
	// 	}
	// 	fmt.Println("Warning: Insecure mode enabled - SSL/TLS certificates will not be verified")
	// }

	// get tailscale MagicDNS name

	lc, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}
	config, err := lc.GetServeConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Tailscale config: %+v\n", config)

	fmt.Printf("Starting proxy server now...")
	fmt.Printf("You can connect to the Docker socket via Tailscale at https://%s.%s:443\n\n", *tsHost, status.CurrentTailnet.MagicDNSSuffix)
	fmt.Printf("   export DOCKER_HOST=tcp://%s.%s:443\n\n", *tsHost, status.CurrentTailnet.MagicDNSSuffix)
	err = http.Serve(ln, proxy)
	log.Fatal(err)
}
