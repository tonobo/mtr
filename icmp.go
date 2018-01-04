package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
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

func (i *icmpret) ID() (id int, seq int, ttl int) {
	switch x := i.Orig.Body.(type) {
	case *icmp.Echo:
		ary := strings.Split(string(x.Data[len(x.Data)-15:]), "-")
		fmt.Println(ary)
		if len(ary) != 3 {
			return
		}
		ttl, _ = strconv.Atoi(ary[2])
		return x.ID, x.Seq, ttl
	case *icmp.TimeExceeded:
		ary := strings.Split(string(x.Data[len(x.Data)-15:]), "-")
		fmt.Println(ary)
		if len(ary) != 3 {
			return
		}
		id, _ = strconv.Atoi(ary[0])
		seq, _ = strconv.Atoi(ary[1])
		ttl, _ = strconv.Atoi(ary[2])
		return
	}
	return
}

func (i *ICMPRequest) Send(c *net.IPConn) error {
	cn := ipv4.NewPacketConn(c)
	cn.SetTTL(i.TTL)
	cn.SetWriteDeadline(time.Now().Add(i.Timeout))
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   i.ID,
			Seq:  int(i.Sequence),
			Data: []byte(fmt.Sprintf("%05d-%05d-%03d", i.ID, i.Sequence, i.TTL)),
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

type TTLMap map[int]*ICMPRequest
type SequenceMap map[int]TTLMap
type RequestMap map[int]SequenceMap

var (
	Listen4Cache = RequestMap{}
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
			if Listen4Cache[m.ID] == nil {
				Listen4Cache[m.ID] = SequenceMap{}
			}
			if Listen4Cache[m.ID][int(m.Sequence)] == nil {
				Listen4Cache[m.ID][int(m.Sequence)] = TTLMap{}
			}
			Listen4Cache[m.ID][int(m.Sequence)][m.TTL] = m
			m.Send(conn)
		case r := <-results:
			id, seq, ttl := r.ID()
			if sc := Listen4Cache[id]; sc != nil {
				if tm := sc[seq]; tm != nil {
					if req := tm[ttl]; req != nil {
						req.ReturnChannel <- &ICMPReturn{
							Success: true,
							Addr:    r.Addr,
							Elapsed: r.ReceivedAt.Sub(req.startTime),
						}
					}
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
		fmt.Printf("%#v -> %#v\n", x, r)
		return r, nil
	case <-time.After(x.Timeout):
		fmt.Printf("LOST: %#v\n", x)
		err = errors.New("request timed out.")
	}
	return
}
