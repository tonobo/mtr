package main

import (
	"container/ring"
	"fmt"
	"net"
	"sync"
	"time"

	gm "github.com/buger/goterm"
)

type MTR struct {
	mutex     *sync.RWMutex
	timeout   time.Duration
	interval  time.Duration
	Address   string `json:"destination"`
	hopsleep  time.Duration
	requests  chan *ICMPRequest
	Statistic map[int]*HopStatistic `json:"statistic"`
}

func NewMTR(addr string, requests chan *ICMPRequest, timeout time.Duration, interval time.Duration, hopsleep time.Duration) (*MTR, chan struct{}) {
	return &MTR{
		interval:  interval,
		timeout:   timeout,
		hopsleep:  hopsleep,
		Address:   addr,
		mutex:     &sync.RWMutex{},
		Statistic: map[int]*HopStatistic{},
		requests:  requests,
	}, make(chan struct{})
}

func (m *MTR) registerStatistic(ttl int, r *ICMPReturn) *HopStatistic {
	m.Statistic[ttl] = &HopStatistic{
		Sent:       1,
		TTL:        ttl,
		Target:     r.Addr,
		timeout:    m.timeout,
		Last:       r,
		Best:       r,
		Worst:      r,
		Lost:       0,
		SumElapsed: r.Elapsed,
		Packets:    ring.New(RING_BUFFER_SIZE),
		requests:   m.requests,
	}
	if !r.Success {
		m.Statistic[ttl].Lost++
	}
	m.Statistic[ttl].Packets.Value = r
	return m.Statistic[ttl]
}

func (m *MTR) Render(offset int) {
	gm.MoveCursor(1, offset)
	l := fmt.Sprintf("%d", RING_BUFFER_SIZE)
	gm.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets")
	for i := 1; i <= len(m.Statistic); i++ {
		gm.MoveCursor(1, offset+i)
		m.mutex.RLock()
		m.Statistic[i].Render()
		m.mutex.RUnlock()
	}
	return
}

func (m *MTR) ping(ch chan struct{}, count int) {
	for i := 0; i < count; i++ {
		time.Sleep(m.interval)
		for i := 1; i <= len(m.Statistic); i++ {
			hop := m.Statistic[i]
			hop.Next(i)
			ch <- struct{}{}
		}
	}
}

func (m *MTR) Run(ch chan struct{}, count int) {
	m.discover(ch)
	m.ping(ch, count)
}

func (m *MTR) discover(ch chan struct{}) {
	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}
	ttlDoubleBump := false
	for ttl := 1; ttl < MAX_HOPS; ttl++ {
		time.Sleep(m.hopsleep)
		hopReturn, err := Icmp(m.requests, &ICMPRequest{
			DestAddr: &ipAddr,
			TTL:      ttl,
			ID:       123,
			Timeout:  m.timeout,
		})
		if err != nil || !hopReturn.Success {
			if ttlDoubleBump {
				break
			}
			m.mutex.Lock()
			s := m.registerStatistic(ttl, hopReturn)
			s.dest = &ipAddr
			m.mutex.Unlock()
			ch <- struct{}{}
			ttlDoubleBump = true
			continue
		}
		ttlDoubleBump = false
		m.mutex.Lock()
		s := m.registerStatistic(ttl, hopReturn)
		s.dest = &ipAddr
		m.mutex.Unlock()
		ch <- struct{}{}
		if hopReturn.Addr == m.Address {
			break
		}
	}
}
