package main

import (
	"container/ring"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type MTR struct {
	mutex     *sync.RWMutex
	Address   string                `json:"destination"`
	Statistic map[int]*HopStatistic `json:"statistic"`
}

func NewMTR(addr string) *MTR {
	return &MTR{
		Address:   addr,
		mutex:     &sync.RWMutex{},
		Statistic: map[int]*HopStatistic{},
	}
}

func (m *MTR) registerStatistic(ttl int, r ICMPReturn) {
	if m.Statistic[ttl] == nil {
		m.Statistic[ttl] = &HopStatistic{
			Sent:       1,
			Last:       r,
			Best:       r,
			Worst:      r,
			Lost:       0,
			SumElapsed: r.Elapsed,
			Packets:    ring.New(RING_BUFFER_SIZE),
		}
		if !r.Success {
			m.Statistic[ttl].Lost++
		}
		m.Statistic[ttl].Packets.Value = r
		return
	}
	s := m.Statistic[ttl]
	s.Packets = s.Packets.Next()
	s.Packets.Value = r
	s.Sent++
	s.SumElapsed = r.Elapsed + s.SumElapsed
	if !r.Success {
		m.Statistic[ttl].Lost++
	}
	s.Last = r
	if s.Best.Elapsed > r.Elapsed {
		s.Best = r
	}
	if s.Worst.Elapsed < r.Elapsed {
		s.Worst = r
	}
}

func (m *MTR) Render() {
	fmt.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst")
	for i := 1; i <= len(m.Statistic); i++ {
		m.mutex.RLock()
		m.Statistic[i].Render(i)
		m.mutex.RUnlock()
	}
}

func (m *MTR) Run() {
	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}
	pid := os.Getpid() & 0xffff
	timeout := 500 * time.Millisecond
	ttlDoubleBump := false

	for ttl := 1; ttl < 64; ttl++ {
		hopReturn, err := Icmp("0.0.0.0", &ipAddr, ttl, pid, timeout)
		if err != nil || !hopReturn.Success {
			if ttlDoubleBump {
				break
			}
			m.mutex.Lock()
			m.registerStatistic(ttl, hopReturn)
			m.mutex.Unlock()
			ttlDoubleBump = true
			continue
		}
		ttlDoubleBump = false
		m.mutex.Lock()
		m.registerStatistic(ttl, hopReturn)
		m.mutex.Unlock()
		if hopReturn.Addr == m.Address {
			break
		}
	}
}
