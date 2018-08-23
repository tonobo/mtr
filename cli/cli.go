package cli

import (
	"errors"
	"fmt"
	"sync"
	"time"

	tm "github.com/buger/goterm"
	pj "github.com/hokaccha/go-prettyjson"
	"github.com/meyskens/mtr/mtr"
	"github.com/spf13/cobra"
)

var (
	COUNT            = 5
	TIMEOUT          = 800 * time.Millisecond
	INTERVAL         = 100 * time.Millisecond
	HOP_SLEEP        = time.Nanosecond
	MAX_HOPS         = 64
	RING_BUFFER_SIZE = 50
	jsonFmt          = false
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use: "mtr TARGET",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("No target provided")
		}
		m, ch := mtr.NewMTR(args[0], TIMEOUT, INTERVAL, HOP_SLEEP, MAX_HOPS, RING_BUFFER_SIZE)
		if jsonFmt {
			go func(ch chan struct{}) {
				for {
					<-ch
				}
			}(ch)
			m.Run(ch, COUNT)
			s, _ := pj.Marshal(m)
			fmt.Println(string(s))
			return nil
		}
		fmt.Println("Start:", time.Now())
		tm.Clear()
		mu := &sync.Mutex{}
		go func(ch chan struct{}) {
			for {
				mu.Lock()
				<-ch
				render(m)
				mu.Unlock()
			}
		}(ch)
		m.Run(ch, COUNT)
		close(ch)
		mu.Lock()
		render(m)
		mu.Unlock()
		return nil
	},
}

func render(m *mtr.MTR) {
	tm.MoveCursor(1, 1)
	m.Render(1)
	tm.Flush() // Call it every time at the end of rendering
}

func init() {
	RootCmd.Flags().IntVarP(&COUNT, "count", "c", COUNT, "Amount of pings per target")
	RootCmd.Flags().DurationVarP(&TIMEOUT, "timeout", "t", TIMEOUT, "ICMP reply timeout")
	RootCmd.Flags().DurationVarP(&INTERVAL, "interval", "i", INTERVAL, "Wait time between icmp packets before sending new one")
	RootCmd.Flags().DurationVar(&HOP_SLEEP, "hop-sleep", HOP_SLEEP, "Wait time between pinging next hop")
	RootCmd.Flags().IntVar(&MAX_HOPS, "max-hops", MAX_HOPS, "Maximal TTL count")
	RootCmd.Flags().IntVar(&RING_BUFFER_SIZE, "buffer-size", RING_BUFFER_SIZE, "Cached packet buffer size")
	RootCmd.Flags().BoolVar(&jsonFmt, "json", jsonFmt, "Print json results")
}
