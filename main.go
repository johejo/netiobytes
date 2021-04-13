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
	interval   time.Duration
	timeout    time.Duration
	_interface string
)

func init() {
	flag.DurationVar(&interval, "interval", 1*time.Second, "interval")
	flag.DurationVar(&timeout, "timeout", 1*time.Hour, "timeout")
	flag.StringVar(&_interface, "interface", "", "network interface")
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ifaceList, err := psutilnet.Interfaces()
	if err != nil {
		return err
	}

	ifaces := make(map[string]struct{})

	if _interface == "" {
	OUTER_LOOP:
		for _, i := range ifaceList {
			for _, a := range i.Addrs {
				ip, _, err := net.ParseCIDR(a.Addr)
				if err != nil {
					return err
				}
				if ip.IsLoopback() || ip.IsUnspecified() || strings.Contains(i.Name, "br") {
					continue OUTER_LOOP
				}
				ifaces[i.Name] = struct{}{}
				break
			}
		}
	} else {
		ifaces[_interface] = struct{}{}
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, timeout+interval)
	defer cancel()

	ctx, cancel = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(interval)
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
