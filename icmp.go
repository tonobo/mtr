package main

import (
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type ICMPReturn struct {
	Success bool
	Addr    string
	Elapsed time.Duration
}

func Icmp(localAddr string, dst net.Addr, ttl, pid int, timeout time.Duration) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		return hop, err
	}
	defer c.Close()
	c.IPv4PacketConn().SetTTL(ttl)
	c.SetDeadline(time.Now().Add(timeout))
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: pid, Seq: 1,
			Data: []byte(""),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return hop, err
	}

	if _, err := c.WriteTo(wb, dst); err != nil {
		return hop, err
	}

	rb := make([]byte, 1500)
	_, peer, err := c.ReadFrom(rb)
	if err != nil {
		return hop, err
	}
	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = peer.String()
	hop.Success = true
	return hop, err
}
