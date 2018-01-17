package main

import (
	"encoding/binary"
	"errors"
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

type ICMPBucket map[int]*ICMPRequest

type ICMPRequest struct {
	ReturnChannel chan *ICMPReturn
	DestAddr      net.Addr
	Sequence      uint32
	TTL           int
	ID            int
	Timeout       time.Duration
	startTime     time.Time
}

type icmpret struct {
	Addr       string
	ReceivedAt time.Time
	Orig       *icmp.Message
}

func (i *icmpret) Exceeded() bool {
	_, ok := i.Orig.Body.(*icmp.TimeExceeded)
	return ok
}

type PacketMap map[uint16]*ICMPRequest

var packets PacketMap

func init() {
	packets = PacketMap{}
}

func (i *icmpret) ID() uint16 {
	switch x := i.Orig.Body.(type) {
	case *icmp.Echo:
		return uint16(x.ID)
	case *icmp.TimeExceeded:
		if len(x.Data) > 25 {
			return binary.LittleEndian.Uint16(x.Data[25:])
		}
	}
	return 0
}

func (i *ICMPRequest) Send(c *net.IPConn) error {
	cn := ipv4.NewPacketConn(c)
	cn.SetTTL(i.TTL)
	cn.SetWriteDeadline(time.Now().Add(i.Timeout))
	buf := make([]byte, 2)
	packets[uint16(i.ID)] = i
	binary.LittleEndian.PutUint16(buf, uint16(i.ID))
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   i.ID,
			Seq:  int(i.Sequence),
			Data: buf,
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return err
	}
	if _, err := c.WriteTo(wb, i.DestAddr); err != nil {
		return err
	}
	return nil
}

const (
	ProtocolICMP     = 1  // Internet Control Message
	ProtocolIPv6ICMP = 58 // ICMP for IPv6
)

func Listen(protocol int, localAddr string, requests chan *ICMPRequest) {
	netaddr, _ := net.ResolveIPAddr("ip4", "0.0.0.0")
	conn, _ := net.ListenIP("ip4:icmp", netaddr)
	results := make(chan *icmpret, 1024)
	go func(c *net.IPConn, ch chan *icmpret) {
		buf := make([]byte, 1024)
		for {
			numRead, addr, _ := conn.ReadFrom(buf)
			x, err := icmp.ParseMessage(protocol, buf[:numRead])
			if err == nil {
				ch <- &icmpret{
					Orig:       x,
					Addr:       addr.String(),
					ReceivedAt: time.Now(),
				}
			}
		}
	}(conn, results)
	for {
		select {
		case m := <-requests:
			m.Send(conn)
		case r := <-results:
			req, ok := packets[uint16(r.ID())]
			if ok {
				req.ReturnChannel <- &ICMPReturn{
					Success: true,
					Addr:    r.Addr,
					Elapsed: r.ReceivedAt.Sub(req.startTime),
				}

			}
		}
	}
}

func Icmp(requests chan *ICMPRequest, x *ICMPRequest) (hop *ICMPReturn, err error) {
	hop = &ICMPReturn{Success: false}
	x.startTime = time.Now()
	x.ReturnChannel = make(chan *ICMPReturn)
	requests <- x
	select {
	case r := <-x.ReturnChannel:
		return r, nil
	case <-time.After(x.Timeout):
		err = errors.New("request timed out.")
	}
	return
}
