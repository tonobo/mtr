package cli

import (
	"fmt"
	"log"
	"sync"
	"time"

	ui "github.com/gizak/termui"
	pj "github.com/hokaccha/go-prettyjson"
	tb "github.com/nsf/termbox-go"
	"github.com/spf13/cobra"
	"github.com/tonobo/mtr/pkg/mtr"
)

var (
	version string
	date    string

	COUNT            = 5
	TIMEOUT          = 800 * time.Millisecond
	INTERVAL         = 1000 * time.Millisecond
	HOP_SLEEP        = time.Nanosecond
	MAX_HOPS         = 64
	MAX_UNKNOWN_HOPS = 10
	RING_BUFFER_SIZE = 50
	PTR_LOOKUP       = false
	jsonFmt          = false
	srcAddr          = ""
	versionFlag      bool
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use:  "mtr TARGET",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			fmt.Printf("MTR Version: %s, build date: %s\n", version, date)
			return nil
		}
		m, ch, err := mtr.NewMTR(args[0], srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
			MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP)
		if err != nil {
			return err
		}
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
		if err := ui.Init(); err != nil {
			log.Fatalf("failed to initialize termui: %v", err)
		}
		defer ui.Close()

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
	m.Render(1)
	tb.Flush() // Call it every time at the end of rendering
}

func init() {
	RootCmd.Flags().IntVarP(&COUNT, "count", "c", COUNT, "Amount of pings per target")
	RootCmd.Flags().DurationVarP(&TIMEOUT, "timeout", "t", TIMEOUT, "ICMP reply timeout")
	RootCmd.Flags().DurationVarP(&INTERVAL, "interval", "i", INTERVAL, "Wait time between icmp packets before sending new one")
	RootCmd.Flags().DurationVar(&HOP_SLEEP, "hop-sleep", HOP_SLEEP, "Wait time between pinging next hop")
	RootCmd.Flags().IntVar(&MAX_HOPS, "max-hops", MAX_HOPS, "Maximal TTL count")
	RootCmd.Flags().IntVar(&MAX_UNKNOWN_HOPS, "max-unknown-hops", MAX_UNKNOWN_HOPS, "Maximal hops that do not reply before stopping to look")
	RootCmd.Flags().IntVar(&RING_BUFFER_SIZE, "buffer-size", RING_BUFFER_SIZE, "Cached packet buffer size")
	RootCmd.Flags().BoolVar(&jsonFmt, "json", jsonFmt, "Print json results")
	RootCmd.Flags().BoolVarP(&PTR_LOOKUP, "ptr", "n", PTR_LOOKUP, "Reverse lookup on host")
	RootCmd.Flags().BoolVar(&versionFlag, "version", false, "Print version")
	RootCmd.Flags().StringVar(&srcAddr, "address", srcAddr, "The address to be bound the outgoing socket")
}
