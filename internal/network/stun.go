package network

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pion/stun/v3"
)

var DefaultSTUNServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun2.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

// DiscoverPublicEndpoint queries public STUN servers to resolve our external public IP and UDP port.
func DiscoverPublicEndpoint(stunServers []string, timeout time.Duration) (*net.UDPAddr, error) {
	if len(stunServers) == 0 {
		stunServers = DefaultSTUNServers
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	var lastErr error
	for _, server := range stunServers {
		addr, err := querySTUN(server, timeout)
		if err == nil && addr != nil {
			return addr, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to discover public endpoint via STUN: %w", lastErr)
	}
	return nil, errors.New("no STUN servers reachable")
}

func querySTUN(server string, timeout time.Duration) (*net.UDPAddr, error) {
	client, err := stun.Dial("udp4", server)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	client.SetRTO(timeout)

	var xorAddr stun.XORMappedAddress
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	var queryErr error
	done := make(chan struct{})

	err = client.Do(message, func(res stun.Event) {
		defer close(done)
		if res.Error != nil {
			queryErr = res.Error
			return
		}
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			queryErr = getErr
			return
		}
	})
	if err != nil {
		return nil, err
	}

	select {
	case <-done:
		if queryErr != nil {
			return nil, queryErr
		}
		return &net.UDPAddr{
			IP:   xorAddr.IP,
			Port: xorAddr.Port,
		}, nil
	case <-time.After(timeout):
		return nil, errors.New("STUN query timed out")
	}
}
