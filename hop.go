package main

import (
	"container/ring"
	"fmt"
	"time"
)

type HopStatistic struct {
	Sent       int
	Last       ICMPReturn
	Best       ICMPReturn
	Worst      ICMPReturn
	SumElapsed time.Duration
	Lost       int
	Packets    *ring.Ring
}

func (h *HopStatistic) Render(ttl int) {
	failedCounter := 0
	successCounter := 0
	h.Packets.Do(func(f interface{}) {
		if f == nil {
			return
		}
		if !f.(ICMPReturn).Success {
			failedCounter++
		} else {
			successCounter++
		}
	})
	addr := "???"
	if h.Last.Addr != "" {
		addr = h.Last.Addr
	}
	avg := 0.0
	if !(h.Sent-h.Lost == 0) {
		avg = h.SumElapsed.Seconds() * 1000 / float64(h.Sent-h.Lost)
	}
	fmt.Printf("%3d:|-- %-20s  %5.1f%%  %4d  %6.1f  %6.1f  %6.1f  %6.1f\n",
		ttl,
		addr,
		float32(h.Lost)/float32(h.Sent)*100.0,
		h.Sent,
		h.Last.Elapsed.Seconds()*1000,
		avg,
		h.Best.Elapsed.Seconds()*1000,
		h.Worst.Elapsed.Seconds()*1000,
	)
}
