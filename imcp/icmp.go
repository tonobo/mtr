package imcp

import (
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ICMPReturn contains the info for a returned IMCP
type ICMPReturn struct {
	Success bool
	Addr    string
	Elapsed time.Duration
}

// SendIMCP sends a IMCP to a given destination but does allow an IMCP timeout
func SendDiscoverIMCP(localAddr string, dst net.Addr, ttl, pid int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
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

// SendIMCP sends a IMCP to a given destination
func SendIMCP(localAddr string, dst net.Addr, pid int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		return hop, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(timeout))
	body := fmt.Sprintf("ping%d", seq)
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: pid, Seq: 0,
			Data: []byte(body),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return hop, err
	}

	if _, err := c.WriteTo(wb, dst); err != nil {
		return hop, err
	}

	_, err = listenForSpecific(time.Now().Add(timeout), dst.String(), body)
	if err != nil {
		return hop, err
	}

	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = dst.String()
	hop.Success = true
	return hop, err
}

// listenForSpecific listens for a reply from a specific destination and returns the body if returned
func listenForSpecific(deadline time.Time, neededPeer, neededBody string) (string, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return "", err
	}
	conn.SetDeadline(deadline)
	defer conn.Close()
	for {
		bytes := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(bytes)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				return "", neterr
			}
		}
		if n == 0 {
			continue
		}

		if peer.String() != neededPeer {
			continue
		}

		x, err := icmp.ParseMessage(1, bytes[:n])
		if err != nil {
			continue
		}

		if x.Type.(ipv4.ICMPType).String() == "time exceeded" {
			return "", errors.New("time exceeded")
		}

		if x.Type.(ipv4.ICMPType).String() == "echo reply" {
			b, _ := x.Body.Marshal(1)
			if string(b[4:]) != neededBody {
				continue
			}
			return string(b[4:]), nil
		}

		panic(x.Type.(ipv4.ICMPType).String())
	}
}
