package mtr

import (
	"container/ring"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	gm "github.com/buger/goterm"
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
	Statistic      map[string]*hop.HopStatistic `json:"statistic"`
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
		Statistic:      map[string]*hop.HopStatistic{},
		maxHops:        maxHops,
		ringBufferSize: ringBufferSize,
		maxUnknownHops: maxUnknownHops,
		ptrLookup:      ptr,
	}, make(chan struct{}), nil
}

func (m *MTR) registerStatistic(ttl int, r icmp.ICMPReturn) *hop.HopStatistic {
	if r.Addr == "" {
		r.Addr = "???"
	}
	id := fmt.Sprintf("%3d-%v", ttl, r.Addr)

	s, ok := m.Statistic[id]
	if !ok {
		s = &hop.HopStatistic{
			Sent:           0,
			TTL:            ttl,
			Target:         r.Addr,
			Timeout:        m.timeout,
			Last:           r,
			Best:           r,
			Worst:          r,
			Lost:           0,
			Packets:        ring.New(m.ringBufferSize),
			RingBufferSize: m.ringBufferSize,
		}
		m.Statistic[id] = s
	}

	s.Last = r
	s.Sent++

	if !r.Success {
		s.Lost++
		setSentToHopUnkown(ttl, m.Statistic)
		return s // do not count failed into statistics
	}

	setSentToHopUnkown(ttl, m.Statistic)

	s.SumElapsed = r.Elapsed + s.SumElapsed

	if s.Best.Elapsed > r.Elapsed {
		s.Best = r
	}
	if s.Worst.Elapsed < r.Elapsed {
		s.Worst = r
	}

	s.Packets = s.Packets.Prev()
	s.Packets.Value = r
	return s
}

// sent + lost
func ttlCheckedCount(ttl int, m map[string]*hop.HopStatistic) int {
	sent := 0

	for key, v := range m {
		if v.TTL != ttl {
			continue
		}

		if strings.HasSuffix(key, "-???") {
			sent += v.Lost
			continue
		}

		sent += v.Sent
	}
	return sent
}

func setSentToHopUnkown(ttl int, m map[string]*hop.HopStatistic) {
	for key, v := range m {
		if !strings.HasSuffix(key, "-???") {
			continue
		}
		if v.TTL != ttl {
			continue
		}

		v.Sent = ttlCheckedCount(ttl, m)
	}
}

func (m *MTR) Render(offset int) {
	gm.MoveCursor(1, offset)
	l := fmt.Sprintf("%d", m.ringBufferSize)
	gm.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets")
	i := 0

	keys := make([]string, 0, len(m.Statistic))
	for k := range m.Statistic {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lastTTL := 0
	for _, k := range keys {
		i++
		gm.MoveCursor(1, offset+i)
		m.mutex.RLock()
		m.Statistic[k].Render(lastTTL, m.ptrLookup)
		m.mutex.RUnlock()
		lastTTL = m.Statistic[k].TTL
	}
}

func (m *MTR) Run(ch chan struct{}, count int) {
	m.discover(ch, count)
}

// discover discovers all hops on the route
func (m *MTR) discover(ch chan struct{}, count int) {
	rand.Seed(time.Now().Unix())
	seq := rand.Intn(math.MaxInt16)
	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}
	pid := os.Getpid() & 0xffff

	for i := 1; i <= count; i++ {
		time.Sleep(m.interval)

		unknownHopsCount := 0
		for ttl := 1; ttl < m.maxHops; ttl++ {
			seq++
			time.Sleep(m.hopsleep)
			var hopReturn icmp.ICMPReturn
			var err error
			if ipAddr.IP.To4() != nil {
				hopReturn, err = icmp.SendDiscoverICMP(m.SrcAddress, &ipAddr, ttl, pid, m.timeout, seq)
			} else {
				hopReturn, err = icmp.SendDiscoverICMPv6(m.SrcAddress, &ipAddr, ttl, pid, m.timeout, seq)
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
				if unknownHopsCount >= m.maxUnknownHops {
					break
				}
				continue
			}
			unknownHopsCount = 0
		}
	}
}
