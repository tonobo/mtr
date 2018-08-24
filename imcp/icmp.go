package imcp

import (
	"bytes"
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
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return hop, err
	}

	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: pid, Seq: seq,
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
func SendIMCP(localAddr string, dst net.Addr, target string, ttl, pid int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		return hop, err
	}
	defer c.Close()
	c.IPv4PacketConn().SetTTL(ttl)
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return hop, err
	}

	body := fmt.Sprintf("ping%d", seq)
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: pid, Seq: seq,
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

	peer, _, err := listenForSpecific(c, time.Now().Add(timeout), target, body, seq, wb)
	if err != nil {
		return hop, err
	}

	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = peer
	hop.Success = true
	return hop, err
}

// listenForSpecific listens for a reply from a specific destination with a specifi body and returns the body if returned
func listenForSpecific(conn *icmp.PacketConn, deadline time.Time, neededPeer, neededBody string, needSeq int, sent []byte) (string, string, error) {
	for {
		b := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(b)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				return "", "", neterr
			}
		}
		if n == 0 {
			continue
		}

		if peer.String() != neededPeer {
			continue
		}

		x, err := icmp.ParseMessage(1, b[:n])
		if err != nil {
			continue
		}

		if x.Type.(ipv4.ICMPType).String() == "time exceeded" {
			body := x.Body.(*icmp.TimeExceeded).Data

			index := bytes.Index(body, sent[:4])
			if index > 0 {
				x, _ := icmp.ParseMessage(1, body[index:])
				seq := x.Body.(*icmp.Echo).Seq
				if seq == needSeq {
					return peer.String(), "", nil
				}
			}
		}

		if x.Type.(ipv4.ICMPType).String() == "echo reply" {
			b, _ := x.Body.Marshal(1)
			if string(b[4:]) != neededBody {
				continue
			}
			return peer.String(), string(b[4:]), nil
		}
	}
}
