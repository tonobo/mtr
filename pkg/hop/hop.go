package hop

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"time"

	gm "github.com/buger/goterm"
	"github.com/tonobo/mtr/pkg/icmp"
)

type HopStatistic struct {
	Dest           *net.IPAddr
	Timeout        time.Duration
	PID            int
	Sent           int
	TTL            int
	Targets        []string
	Last           icmp.ICMPReturn
	Best           icmp.ICMPReturn
	Worst          icmp.ICMPReturn
	SumElapsed     time.Duration
	Lost           int
	Packets        *ring.Ring
	RingBufferSize int
	dnsCache       map[string]string
}

type packet struct {
	Success      bool    `json:"success"`
	ResponseTime float64 `json:"respond_ms"`
}

func (h *HopStatistic) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Sent             int       `json:"sent"`
		Target           string    `json:"target"`
		Last             float64   `json:"last_ms"`
		Best             float64   `json:"best_ms"`
		Worst            float64   `json:"worst_ms"`
		Loss             float64   `json:"loss_percent"`
		Avg              float64   `json:"avg_ms"`
		Stdev            float64   `json:"stdev_ms"`
		PacketBufferSize int       `json:"packet_buffer_size"`
		TTL              int       `json:"ttl"`
		Packets          []*packet `json:"packet_list_ms"`
	}{
		Sent:             h.Sent,
		TTL:              h.TTL,
		Loss:             h.Loss(),
		Target:           fmt.Sprintf("%v", h.Targets),
		PacketBufferSize: h.RingBufferSize,
		Last:             h.Last.Elapsed.Seconds() * 1000,
		Best:             h.Best.Elapsed.Seconds() * 1000,
		Worst:            h.Worst.Elapsed.Seconds() * 1000,
		Avg:              h.Avg(),
		Stdev:            h.Stdev(),
		Packets:          h.packets(),
	})
}

func (h *HopStatistic) Avg() float64 {
	avg := 0.0
	if !(h.Sent-h.Lost == 0) {
		avg = h.SumElapsed.Seconds() * 1000 / float64(h.Sent-h.Lost)
	}
	return avg
}

func (h *HopStatistic) Stdev() float64 {
	avg := h.Avg()
	result := 0.0
	n := 0

	for _, p := range h.packets() {
		if p == nil || !p.Success {
			continue
		}

		n++
		distance := math.Abs(p.ResponseTime - avg)
		dsquare := distance * distance
		result += dsquare
	}

	result = result / float64(n)
	result = math.Sqrt(result)

	if math.IsNaN(result) {
		result = 0
	}

	return result
}

func (h *HopStatistic) Loss() float64 {
	return float64(h.Lost) / float64(h.Sent) * 100.0
}

func (h *HopStatistic) packets() []*packet {
	v := make([]*packet, h.RingBufferSize)
	i := 0
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			v[i] = nil
			i++
			return
		}
		x := f.(icmp.ICMPReturn)
		if x.Success {
			v[i] = &packet{
				Success:      true,
				ResponseTime: x.Elapsed.Seconds() * 1000,
			}
		} else {
			v[i] = &packet{
				Success:      false,
				ResponseTime: 0.0,
			}
		}
		i++
	})
	return v
}

func (h *HopStatistic) Render(ptrLookup bool) {
	if h.dnsCache == nil {
		h.dnsCache = map[string]string{}
	}
	packets := make([]byte, h.RingBufferSize)
	i := h.RingBufferSize - 1
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			packets[i] = ' '
		} else if !f.(icmp.ICMPReturn).Success {
			packets[i] = '?'
		} else {
			packets[i] = '.'
		}
		i--
	})
	l := fmt.Sprintf("%d", h.RingBufferSize)
	gm.Printf("%3d:|-- %-20s  %5.1f%%  %4d  %6.1f  %6.1f  %6.1f  %6.1f  %"+l+"s\n",
		h.TTL,
		fmt.Sprintf("%.20s", h.lookupAddr(ptrLookup, 0)),
		h.Loss(),
		h.Sent,
		h.Last.Elapsed.Seconds()*1000,
		h.Avg(),
		h.Best.Elapsed.Seconds()*1000,
		h.Worst.Elapsed.Seconds()*1000,
		packets,
	)
}

func (h *HopStatistic) lookupAddr(ptrLookup bool, index int) string {
	addr := "???"
	if h.Targets[index] != "" {
		addr = h.Targets[index]
		if ptrLookup {
			if key, ok := h.dnsCache[h.Targets[index]]; ok {
				addr = key
			} else {
				names, err := net.LookupAddr(h.Targets[index])
				if err == nil && len(names) > 0 {
					addr = names[0]
				}
			}
		}
		h.dnsCache[h.Targets[index]] = addr
	}
	return addr
}
