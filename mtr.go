package main

import (
	"container/ring"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	gm "github.com/buger/goterm"
)

type MTR struct {
	mutex     *sync.RWMutex
	timeout   time.Duration
	interval  time.Duration
	Address   string                `json:"destination"`
	Statistic map[int]*HopStatistic `json:"statistic"`
}

func NewMTR(addr string, timeout time.Duration, interval time.Duration) (*MTR, chan struct{}) {
	return &MTR{
		interval:  interval,
		timeout:   timeout,
		Address:   addr,
		mutex:     &sync.RWMutex{},
		Statistic: map[int]*HopStatistic{},
	}, make(chan struct{})
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
	s.Packets = s.Packets.Next()
}

func (m *MTR) Render(offset int) {
	gm.MoveCursor(1, offset)
	l := fmt.Sprintf("%d", RING_BUFFER_SIZE)
	gm.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets")
	for i := 1; i <= len(m.Statistic); i++ {
		gm.MoveCursor(1, offset+i)
		m.mutex.RLock()
		m.Statistic[i].Render(i)
		m.mutex.RUnlock()
	}
	return
}

func (m *MTR) Run(ch chan struct{}) {
	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}
	pid := os.Getpid() & 0xffff
	ttlDoubleBump := false

	for ttl := 1; ttl < 64; ttl++ {
		time.Sleep(m.interval)
		hopReturn, err := Icmp("0.0.0.0", &ipAddr, ttl, pid, m.timeout)
		if err != nil || !hopReturn.Success {
			if ttlDoubleBump {
				break
			}
			m.mutex.Lock()
			m.registerStatistic(ttl, hopReturn)
			m.mutex.Unlock()
			ch <- struct{}{}
			ttlDoubleBump = true
			continue
		}
		ttlDoubleBump = false
		m.mutex.Lock()
		m.registerStatistic(ttl, hopReturn)
		m.mutex.Unlock()
		ch <- struct{}{}
		if hopReturn.Addr == m.Address {
			break
		}
	}
}
