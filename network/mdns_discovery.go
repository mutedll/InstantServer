// network/mdns_discovery.go
package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	mdnsMulticastIPv4 = "224.0.0.251:5353"
)

type DiscoveredServer struct {
	Hostname  string `json:"hostname"`
	IP        string `json:"ip"`
	OpenPorts []int  `json:"open_ports"`
	ViaMDNS   bool   `json:"via_mdns"`
}

func encodeDNSName(domain string) []byte {
	var buf []byte
	for _, label := range strings.Split(strings.TrimSuffix(domain, "."), ".") {
		if len(label) == 0 || len(label) > 63 {
			continue
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0x00)
	return buf
}

func buildMDNSQueryPacket(fqdn string) []byte {
	pkt := make([]byte, 12)
	binary.BigEndian.PutUint16(pkt[0:2], 0x0000)
	binary.BigEndian.PutUint16(pkt[2:4], 0x0000)
	binary.BigEndian.PutUint16(pkt[4:6], 0x0001)
	qname := encodeDNSName(fqdn)
	pkt = append(pkt, qname...)
	qtail := make([]byte, 4)
	binary.BigEndian.PutUint16(qtail[0:2], 0x0001) // Type A
	binary.BigEndian.PutUint16(qtail[2:4], 0x0001) // Class IN
	pkt = append(pkt, qtail...)
	return pkt
}

func QueryMDNSHostLocal(hostname string, timeout time.Duration) string {
	cleanHost := strings.TrimSuffix(strings.TrimSpace(hostname), ".local") + ".local"
	if ips, err := net.LookupHost(cleanHost); err == nil && len(ips) > 0 {
		for _, ip := range ips {
			if !strings.Contains(ip, ":") {
				return ip
			}
		}
	}

	raddr, err := net.ResolveUDPAddr("udp4", mdnsMulticastIPv4)
	if err != nil {
		return ""
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return ""
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	query := buildMDNSQueryPacket(cleanHost)
	_, _ = conn.WriteToUDP(query, raddr)

	resp := make([]byte, 512)
	n, fromAddr, err := conn.ReadFromUDP(resp)
	if err == nil && n >= 16 && fromAddr != nil {
		return fromAddr.IP.String()
	}
	return ""
}

func DiscoverInstantServersOnLAN(ctx context.Context, preferredHostname string) []DiscoveredServer {
	var results []DiscoveredServer
	var mu sync.Mutex
	seenIPs := make(map[string]bool)

	if mdnsIP := QueryMDNSHostLocal(preferredHostname, 450*time.Millisecond); mdnsIP != "" {
		seenIPs[mdnsIP] = true
		results = append(results, DiscoveredServer{
			Hostname:  strings.TrimSuffix(preferredHostname, ".local") + ".local",
			IP:        mdnsIP,
			OpenPorts: []int{22, 80, 8088, 9090},
			ViaMDNS:   true,
		})
	}

	targetPorts := []int{8088, 9090, 80, 22}
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", 250*time.Millisecond)
	if err != nil {
		return results
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.To4()
	_ = conn.Close()
	if localAddr == nil {
		return results
	}

	prefix := fmt.Sprintf("%d.%d.%d.", localAddr[0], localAddr[1], localAddr[2])
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)

	for hostOctet := 1; hostOctet <= 254; hostOctet++ {
		select {
		case <-ctx.Done():
			break
		default:
		}
		candidateIP := fmt.Sprintf("%s%d", prefix, hostOctet)
		if seenIPs[candidateIP] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			var open []int
			for _, port := range targetPorts {
				c, dErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 90*time.Millisecond)
				if dErr == nil {
					_ = c.Close()
					open = append(open, port)
				}
			}
			if len(open) >= 2 {
				mu.Lock()
				if !seenIPs[ip] {
					seenIPs[ip] = true
					results = append(results, DiscoveredServer{
						Hostname:  preferredHostname + ".local",
						IP:        ip,
						OpenPorts: open,
						ViaMDNS:   false,
					})
				}
				mu.Unlock()
			}
		}(candidateIP)
	}
	wg.Wait()
	return results
}