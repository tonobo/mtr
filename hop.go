package main

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"time"

	gm "github.com/buger/goterm"
)

type HopStatistic struct {
	Sent       int
	TTL        int
	Target     string
	Last       ICMPReturn
	Best       ICMPReturn
	Worst      ICMPReturn
	SumElapsed time.Duration
	Lost       int
	Packets    *ring.Ring
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
		PacketBufferSize int       `json:"packet_buffer_size"`
		TTL              int       `json:"ttl"`
		Packets          []*packet `json:"packet_list_ms"`
	}{
		Sent:    h.Sent,
		TTL:     h.TTL,
		Loss:    h.Loss(),
		Target:  h.Target,
		Last:    h.Last.Elapsed.Seconds() * 1000,
		Best:    h.Best.Elapsed.Seconds() * 1000,
		Worst:   h.Worst.Elapsed.Seconds() * 1000,
		Avg:     h.Avg(),
		Packets: h.packets(),
	})
}

func (h *HopStatistic) Avg() float64 {
	avg := 0.0
	if !(h.Sent-h.Lost == 0) {
		avg = h.SumElapsed.Seconds() * 1000 / float64(h.Sent-h.Lost)
	}
	return avg
}

func (h *HopStatistic) Loss() float64 {
	return float64(h.Lost) / float64(h.Sent) * 100.0
}

func (h *HopStatistic) packets() []*packet {
	v := make([]*packet, RING_BUFFER_SIZE)
	i := 0
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			v[i] = nil
			i++
			return
		}
		x := f.(ICMPReturn)
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

func (h *HopStatistic) Render() {
	packets := make([]byte, RING_BUFFER_SIZE)
	i := RING_BUFFER_SIZE - 1
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			packets[i] = ' '
		} else if !f.(ICMPReturn).Success {
			packets[i] = '?'
		} else {
			packets[i] = '.'
		}
		i--
	})
	addr := "???"
	if h.Target != "" {
		addr = h.Target
	}
	l := fmt.Sprintf("%d", RING_BUFFER_SIZE)
	gm.Printf("%3d:|-- %-20s  %5.1f%%  %4d  %6.1f  %6.1f  %6.1f  %6.1f  %"+l+"s\n",
		h.TTL,
		addr,
		h.Loss(),
		h.Sent,
		h.Last.Elapsed.Seconds()*1000,
		h.Avg(),
		h.Best.Elapsed.Seconds()*1000,
		h.Worst.Elapsed.Seconds()*1000,
		packets,
	)
}
