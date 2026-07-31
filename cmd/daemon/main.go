package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nekocode/runtime/httpapi"
	"nekocode/runtime/standard"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	token := flag.String("token", os.Getenv("NEKOCODE_DAEMON_TOKEN"), "optional bearer token for HTTP API")
	flag.Parse()

	rt, err := standard.New()
	if err != nil {
		log.Fatalf("initialize runtime: %v", err)
	}
	handler := httpapi.New(rt).Handler()
	handler = httpapi.WithBearerAuth(handler, *token)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		printStartup(*addr, strings.TrimSpace(*token) != "")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("daemon failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("daemon shutdown error: %v", err)
	}
	if err := rt.Close(); err != nil {
		log.Printf("runtime shutdown error: %v", err)
	}
}

func printStartup(addr string, hasToken bool) {
	fmt.Println("NekoCode daemon listening")
	for _, u := range accessURLs(addr) {
		fmt.Println("  " + u)
	}
	if hasToken {
		fmt.Println("Auth: enabled")
		fmt.Println(`Example: curl -H "Authorization: Bearer $NEKOCODE_DAEMON_TOKEN" http://127.0.0.1:8765/runs`)
	} else {
		fmt.Println("Auth: disabled")
		if isWideBind(addr) {
			fmt.Println("Warning: this address is reachable from your network. Use -token or NEKOCODE_DAEMON_TOKEN for shared WiFi.")
		}
		fmt.Println(`Example: curl -X POST http://127.0.0.1:8765/input -d '{"text":"hello"}'`)
	}
	fmt.Println("SSE: /events")
}

func accessURLs(addr string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []string{"http://" + addr}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "0.0.0.0" && host != "::" {
		return []string{"http://" + net.JoinHostPort(host, port)}
	}
	urls := []string{"http://127.0.0.1:" + port}
	for _, ip := range privateIPv4s() {
		urls = append(urls, "http://"+net.JoinHostPort(ip, port))
	}
	return urls
}

func privateIPv4s() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipFromAddr(addr)
			if ip == nil || ip.To4() == nil {
				continue
			}
			out = append(out, ip.String())
		}
	}
	return out
}

func ipFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

func isWideBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "" || host == "0.0.0.0" || host == "::"
}
