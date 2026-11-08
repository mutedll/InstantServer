// network/mdns_test.go
package network

import (
	"bytes"
	"testing"
)

func TestMDNSPacketEncoding(t *testing.T) {
	encoded := encodeDNSName("instant-server.local")
	if !bytes.Contains(encoded, []byte("instant-server")) || !bytes.Contains(encoded, []byte("local")) {
		t.Fatalf("encoded DNS name missing labels: %v", encoded)
	}
	if encoded[len(encoded)-1] != 0x00 {
		t.Fatalf("expected null-terminated DNS label sequence")
	}

	pkt := buildMDNSQueryPacket("instant-server.local")
	if len(pkt) < 20 {
		t.Fatalf("expected valid mDNS query packet >= 20 bytes, got %d", len(pkt))
	}
}