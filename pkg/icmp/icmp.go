package icmp

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	ProtocolICMP     = 1  // Internet Control Message
	ProtocolIPv6ICMP = 58 // ICMP for IPv6
)

// ICMPReturn contains the info for a returned ICMP
type ICMPReturn struct {
	Success bool
	Addr    string
	Elapsed time.Duration
}

// SendDiscoverICMP sends a ICMP to a given destination with a TTL to discover hops
func SendDiscoverICMP(localAddr string, dst net.Addr, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	return SendICMP(localAddr, dst, "", ttl, id, timeout, seq)
}

// SendDiscoverICMPv6 sends a ICMP to a given destination with a TTL to discover hops
func SendDiscoverICMPv6(localAddr string, dst net.Addr, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	return SendICMPv6(localAddr, dst, "", ttl, id, timeout, seq)
}

// SendICMP sends a ICMP to a given destination which requires a  reply from that specific destination
func SendICMP(localAddr string, dst net.Addr, target string, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		log.Panicf("Failed to listen to address %v. Msg: %v.", localAddr, err.Error())
		return hop, err
	}
	defer c.Close()
	err = c.IPv4PacketConn().SetTTL(ttl)
	if err != nil {
		return hop, err
	}
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return hop, err
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(seq))
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: id, Seq: seq,
			Data: append(bs, 'x'),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return hop, err
	}

	if _, err := c.WriteTo(wb, dst); err != nil {
		return hop, err
	}

	peer, _, err := listenForSpecific4(c, time.Now().Add(timeout), target, append(bs, 'x'), id, seq, wb)
	if err != nil {
		return hop, err
	}

	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = peer
	hop.Success = true
	return hop, err
}

// SendICMPv6 sends a ICMP to a given destination which requires a  reply from that specific destination
func SendICMPv6(localAddr string, dst net.Addr, target string, ttl, id int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip6:ipv6-icmp", localAddr)
	if err != nil {
		log.Panicf("Failed to listen to address %v. Msg: %v.", localAddr, err.Error())
		return hop, err
	}
	defer c.Close()
	err = c.IPv6PacketConn().SetHopLimit(ttl)
	if err != nil {
		return hop, err
	}
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return hop, err
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(seq))
	wm := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest, Code: 0,
		Body: &icmp.Echo{
			ID: id, Seq: seq,
			Data: append(bs, 'x'),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return hop, err
	}

	if _, err := c.WriteTo(wb, dst); err != nil {
		return hop, err
	}

	peer, _, err := listenForSpecific6(c, time.Now().Add(timeout), target, append(bs, 'x'), id, seq, wb)
	if err != nil {
		return hop, err
	}

	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = peer
	hop.Success = true
	return hop, err
}

func listenForSpecific6(conn *icmp.PacketConn, deadline time.Time, neededPeer string, neededBody []byte, id, needSeq int, sent []byte) (string, []byte, error) {
	for {
		b := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(b)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				return "", []byte{}, neterr
			}
		}
		if n == 0 {
			continue
		}

		if neededPeer != "" && peer.String() != neededPeer {
			continue
		}

		x, err := icmp.ParseMessage(ProtocolIPv6ICMP, b[:n])
		if err != nil {
			continue
		}

		if x.Type.(ipv6.ICMPType) == ipv6.ICMPTypeTimeExceeded {
			body := x.Body.(*icmp.TimeExceeded).Data
			x, _ := icmp.ParseMessage(ProtocolIPv6ICMP, body[40:])
			switch x.Body.(type) {
			case *icmp.Echo:
				echoBody := x.Body.(*icmp.Echo)
				if echoBody.Seq == needSeq && echoBody.ID == id {
					return peer.String(), []byte{}, nil
				}
				continue
			default:
				// ignore
			}
		}

		if typ, ok := x.Type.(ipv6.ICMPType); ok && typ == ipv6.ICMPTypeEchoReply {
			b, _ := x.Body.Marshal(1)
			if string(b[4:]) != string(neededBody) {
				continue
			}
			echoBody := x.Body.(*icmp.Echo)
			if echoBody.Seq == needSeq && echoBody.ID == id {
				return peer.String(), b[4:], nil
			}
			continue
		}
	}
}

// listenForSpecific4 listens for a reply from a specific destination with a specifi body and returns the body if returned
func listenForSpecific4(conn *icmp.PacketConn, deadline time.Time, neededPeer string, neededBody []byte, pid, needSeq int, sent []byte) (string, []byte, error) {
	for {
		b := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(b)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				return "", []byte{}, neterr
			}
		}
		if n == 0 {
			continue
		}

		if neededPeer != "" && peer.String() != neededPeer {
			continue
		}

		x, err := icmp.ParseMessage(ProtocolICMP, b[:n])
		if err != nil {
			continue
		}

		if typ, ok := x.Type.(ipv4.ICMPType); ok && typ.String() == "time exceeded" {
			body := x.Body.(*icmp.TimeExceeded).Data

			index := bytes.Index(body, sent[:4])
			if index > 0 {
				x, _ := icmp.ParseMessage(ProtocolICMP, body[index:])
				switch x.Body.(type) {
				case *icmp.Echo:
					echoBody := x.Body.(*icmp.Echo)
					if echoBody.Seq == needSeq && echoBody.ID == pid {
						return peer.String(), []byte{}, nil
					}
					continue
				default:
					// ignore
				}
			}
		}

		if typ, ok := x.Type.(ipv4.ICMPType); ok && typ.String() == "echo reply" {
			b, _ := x.Body.Marshal(1)
			if string(b[4:]) != string(neededBody) {
				continue
			}
			echoBody := x.Body.(*icmp.Echo)
			if echoBody.Seq == needSeq && echoBody.ID == pid {
				return peer.String(), b[4:], nil
			}
			continue
		}
	}
}
