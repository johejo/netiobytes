package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	psutilnet "github.com/shirou/gopsutil/v3/net"
)

var (
	interval string
)

func init() {
	flag.StringVar(&interval, "interval", "1s", "interval")
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	_interval, err := time.ParseDuration(interval)
	if err != nil {
		return err
	}
	ifaceList, err := psutilnet.Interfaces()
	if err != nil {
		return err
	}

	ifaces := make(map[string]struct{})
	for _, i := range ifaceList {
		for _, a := range i.Addrs {
			ip, _, err := net.ParseCIDR(a.Addr)
			if err != nil {
				return err
			}
			if !ip.IsLoopback() && !ip.IsUnspecified() && !strings.Contains(i.Name, "br") {
				ifaces[i.Name] = struct{}{}
				break
			}
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(_interval)
	defer ticker.Stop()

	prev := make(map[string]psutilnet.IOCountersStat)
	for k := range ifaces {
		prev[k] = psutilnet.IOCountersStat{}
	}

	fmt.Println("datetime interface byteSent byteRecv")

	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		stats, err := psutilnet.IOCounters(true)
		if err != nil {
			return err
		}
		for _, s := range stats {
			if _, ok := ifaces[s.Name]; ok {
				if i != 0 {
					p := prev[s.Name]
					bsx := s.BytesSent - p.BytesSent
					brx := s.BytesRecv - p.BytesRecv
					fmt.Println(time.Now().Format(time.RFC3339), s.Name, format(bsx), format(brx))
				}
				prev[s.Name] = s
			}
		}
	}
}

func format(v uint64) string {
	return strings.ReplaceAll(humanize.Bytes(v), " ", "") + "/s"
}
