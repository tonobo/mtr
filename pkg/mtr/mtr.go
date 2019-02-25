package mtr

import (
	"container/ring"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	//ui "github.com/gizak/termui"
	//"github.com/gizak/termui/widgets"
	tb "github.com/nsf/termbox-go"

	"github.com/tonobo/mtr/pkg/hop"
	"github.com/tonobo/mtr/pkg/icmp"
)

type MTR struct {
	SrcAddress     string `json:"source"`
	mutex          *sync.RWMutex
	timeout        time.Duration
	interval       time.Duration
	Address        string `json:"destination"`
	hopsleep       time.Duration
	Statistic      map[int]*hop.HopStatistic `json:"statistic"`
	ringBufferSize int
	maxHops        int
	maxUnknownHops int
	ptrLookup      bool
}

func NewMTR(addr, srcAddr string, timeout time.Duration, interval time.Duration,
	hopsleep time.Duration, maxHops, maxUnknownHops, ringBufferSize int, ptr bool) (*MTR, chan struct{}, error) {
	if net.ParseIP(addr) == nil {
		addrs, err := net.LookupHost(addr)
		if err != nil || len(addrs) == 0 {
			return nil, nil, fmt.Errorf("invalid host or ip provided: %s", err)
		}
		addr = addrs[0]
	}
	if srcAddr == "" {
		if net.ParseIP(addr).To4() != nil {
			srcAddr = "0.0.0.0"
		} else {
			srcAddr = "::"
		}
	}
	return &MTR{
		SrcAddress:     srcAddr,
		interval:       interval,
		timeout:        timeout,
		hopsleep:       hopsleep,
		Address:        addr,
		mutex:          &sync.RWMutex{},
		Statistic:      map[int]*hop.HopStatistic{},
		maxHops:        maxHops,
		ringBufferSize: ringBufferSize,
		maxUnknownHops: maxUnknownHops,
		ptrLookup:      ptr,
	}, make(chan struct{}), nil
}

func (m *MTR) registerStatistic(ttl int, r icmp.ICMPReturn) *hop.HopStatistic {
	m.Statistic[ttl] = &hop.HopStatistic{
		Sent:           1,
		TTL:            ttl,
		Target:         r.Addr,
		Timeout:        m.timeout,
		Last:           r,
		Best:           r,
		Worst:          r,
		Lost:           0,
		SumElapsed:     r.Elapsed,
		Packets:        ring.New(m.ringBufferSize),
		RingBufferSize: m.ringBufferSize,
	}
	if !r.Success {
		m.Statistic[ttl].Lost++
	}
	m.Statistic[ttl].Packets.Value = r
	return m.Statistic[ttl]
}

func (m *MTR) Render(offset int) {
	tb.SetCursor(0, 0)
	l := fmt.Sprintf("%d", m.ringBufferSize)
	fmt.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets")
	for i := 1; i <= len(m.Statistic); i++ {
		tb.HideCursor()
		m.mutex.RLock()
		m.Statistic[i].Render(m.ptrLookup)
		m.mutex.RUnlock()
	}
}

func (m *MTR) ping(ch chan struct{}, count int) {
	for i := 0; i < count; i++ {
		time.Sleep(m.interval)
		for i := 1; i <= len(m.Statistic); i++ {
			time.Sleep(m.hopsleep)
			m.mutex.RLock()
			m.Statistic[i].Next(m.SrcAddress)
			m.mutex.RUnlock()
			ch <- struct{}{}
		}
	}
}

func (m *MTR) Run(ch chan struct{}, count int) {
	m.discover(ch)
	m.ping(ch, count-1)
}

// discover discovers all hops on the route
func (m *MTR) discover(ch chan struct{}) {
	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}
	pid := os.Getpid() & 0xffff
	unknownHopsCount := 0
	for ttl := 1; ttl < m.maxHops; ttl++ {
		time.Sleep(m.hopsleep)
		var hopReturn icmp.ICMPReturn
		var err error
		if ipAddr.IP.To4() != nil {
			hopReturn, err = icmp.SendDiscoverICMP(m.SrcAddress, &ipAddr, ttl, pid, m.timeout, 1)
		} else {
			hopReturn, err = icmp.SendDiscoverICMPv6(m.SrcAddress, &ipAddr, ttl, pid, m.timeout, 1)
		}

		m.mutex.Lock()
		s := m.registerStatistic(ttl, hopReturn)
		s.Dest = &ipAddr
		s.PID = pid
		m.mutex.Unlock()
		ch <- struct{}{}
		if hopReturn.Addr == m.Address {
			break
		}
		if err != nil || !hopReturn.Success {
			unknownHopsCount++
			if unknownHopsCount > m.maxUnknownHops {
				break
			}
			continue
		}
		unknownHopsCount = 0
	}
}
